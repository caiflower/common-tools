/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package taskx

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"sync/atomic"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/inflight"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/pkg/bean"
	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/caiflower/common-tools/taskx/dao/model"
	"github.com/caiflower/common-tools/taskx/proto"
	"github.com/uptrace/bun"
)

var SingletonTaskDispatcher = &taskDispatcher{
	allocateWorkerInflight: inflight.NewInFlight(),
}

var initOnce sync.Once

type taskDispatcher struct {
	cluster.DefaultCaller
	Cluster                cluster.ICluster  `autowired:""`
	TaskDao                dao.TaskDAO       `autowired:""`
	TaskBakDao             dao.TaskBakDAO    `autowired:""`
	SubtaskDao             dao.SubtaskDAO    `autowired:""`
	SubtaskBakDao          dao.SubtaskBakDAO `autowired:""`
	DBClient               dbv1.DB           `autowired:""`
	TaskReceiver           *taskReceiver     `autowired:""`
	cfg                    *Config
	running                atomic.Value
	runningL               atomic.Value
	allocateWorkerInflight *inflight.InFlight
	inQueueTasks           sync.Map
	delayQueue             *basic.DelayQueue
	leaderStopChan         chan struct{}
	lastMasterCallTime     atomic.Value // 上次 MasterCall 时间，用于节流
	taskCache              sync.Map     // taskID -> *cachedTask，缓存编译后的 DAG
	randSource             *rand.Rand   // 局部随机数生成器，避免全局锁
}

type Config struct {
	TaskWorker               int           `yaml:"taskWorker" default:"20"`
	TaskQueueSize            int           `yaml:"taskQueueSize" default:"100"`
	SubtaskWorker            int           `yaml:"subtaskWorker" default:"100"`
	SubtaskQueueSize         int           `yaml:"subtaskQueueSize" default:"200"`
	SubtaskRollbackWorker    int           `yaml:"subtaskRollbackWorker" default:"50"`
	SubtaskRollbackQueueSize int           `yaml:"subtaskRollbackQueueSize" default:"100"`
	RemoteCallTimeout        time.Duration `yaml:"remoteCallTimeout" default:"3s"`
	BackupTaskAge            time.Duration `yaml:"backupTaskAge" default:"168h"`
}

type affinity struct {
	Type   TaskAffinityType
	Worker string
}

// cachedTask 缓存编译后的 Task，避免每次调度都重建 DAG
type cachedTask struct {
	task      *Task
	createdAt time.Time
}

const taskCacheTTL = 30 * time.Second

// masterCallMinInterval MasterCall 最小调用间隔，避免频繁查询 DB
const masterCallMinInterval = 5 * time.Second

func InitTaskDispatcher(cfg *Config) {
	initOnce.Do(func() {
		_ = tools.DoTagFunc(&cfg, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})
		_tr.subtaskWorker = cfg.SubtaskWorker
		_tr.taskWorker = cfg.TaskWorker
		_tr.subtaskRollbackWorker = cfg.SubtaskRollbackWorker
		_tr.subtaskQueueSize = cfg.SubtaskQueueSize
		_tr.taskQueueSize = cfg.TaskQueueSize
		_tr.subtaskRollbackQueueSize = cfg.SubtaskRollbackQueueSize
		SingletonTaskDispatcher.cfg = cfg
		SingletonTaskDispatcher.delayQueue = basic.NewDelayQueue()
		SingletonTaskDispatcher.randSource = rand.New(rand.NewSource(time.Now().UnixNano()))
		_tr.cfg = cfg
		bean.AddBean(dao.NewTaskDAO())
		bean.AddBean(dao.NewSubtaskBakDAO())
		bean.AddBean(SingletonTaskDispatcher)
		bean.AddBean(_tr)
	})
}

func (t *taskDispatcher) MasterCall() {
	if t.Cluster == nil {
		logger.Trace("[MasterCall] Cluster is nil, skip")
		return
	}
	if t.runningL.Load() != nil && t.runningL.Load().(bool) {
		logger.Trace("[MasterCall] already running, skip")
		return
	}

	// 节流：距离上次调用不足 masterCallMinInterval 则跳过
	if lastCall := t.lastMasterCallTime.Load(); lastCall != nil {
		if time.Since(lastCall.(time.Time)) < masterCallMinInterval {
			logger.Trace("[MasterCall] throttled, lastCall=%v, elapsed=%v", lastCall.(time.Time).Format("15:04:05.000"), time.Since(lastCall.(time.Time)))
			return
		}
	}

	logger.Info("[MasterCall] executing, isLeader=%v, isReady=%v", t.Cluster.IsLeader(), t.Cluster.IsReady())

	t.runningL.Store(true)
	t.lastMasterCallTime.Store(time.Now())

	golocalv1.PutTraceID(tools.UUID())
	defer func() {
		golocalv1.Clean()
		t.runningL.Store(false)
	}()

	// handle task
	t.handleTask(context.TODO())
	// back task
	//t.backupTask()
}

// OnStartedLeading handles task distribution when becoming leader
func (t *taskDispatcher) OnStartedLeading() {
	logger.Info("[taskDispatcher] node %s starts dispatching tasks", t.Cluster.GetMyName())
	t.running.Store(true)
	t.leaderStopChan = make(chan struct{})

	// Start delay queue processor
	for t.running.Load().(bool) {
		// Take task from delay queue (supports stop signal)
		item := t.delayQueue.TakeWithStop(t.leaderStopChan)
		if item == nil {
			// 收到停止信号
			logger.Info("[taskDispatcher] leader stop signal received, exiting dispatch loop")
			return
		}

		// Handle batch task IDs
		var taskIDs = item.([]string)

		// Batch handle tasks
		if len(taskIDs) > 0 {
			logger.Trace("[OnStartedLeading] took taskIDs=%v from delayQueue", taskIDs)

			for _, v := range taskIDs {
				t.inQueueTasks.Delete(v)
			}

			t.handleTaskImmediately(context.TODO(), taskIDs)
		}
	}
}

func (t *taskDispatcher) OnStoppedLeading() {
	logger.Info("[taskDispatcher] node %s stops dispatching tasks", t.Cluster.GetMyName())
	t.running.Store(false)
	// 关闭 leaderStopChan，中断 TakeWithStop 的阻塞等待
	if t.leaderStopChan != nil {
		close(t.leaderStopChan)
		t.leaderStopChan = nil
	}
	// 清理任务缓存
	t.taskCache.Range(func(key, _ interface{}) bool {
		t.taskCache.Delete(key)
		return true
	})
}

func SubmitTask(ctx context.Context, task *Task) error {
	return SingletonTaskDispatcher.SubmitTask(ctx, task)
}

func (t *taskDispatcher) SubmitTask(ctx context.Context, task *Task) error {
	tx := dbv1.NewBatchTx(t.TaskDao.GetClient().GetDB())
	taskBean, subtaskBeans := task.convert2Bean()

	if TaskAffinityType(taskBean.AffinityType) != AffinityRandom && taskBean.PrimaryWorker == "" {
		nodeName := t.selectNodeByAffinity(AffinityRandom, "", "")
		if nodeName == "" {
			return errors.New("task node name failed")
		}
		taskBean.PrimaryWorker = nodeName
	}

	tx.Add(func(tx *bun.Tx) error {
		_, err := t.TaskDao.Insert(ctx, taskBean, tx)
		return err
	})

	// if not rollback executor, set rollback to NoneRollback
	for i, subtask := range subtaskBeans {
		if task.em.getRollbackProvider(taskBean.TaskName, subtask.TaskName) == nil {
			subtaskBeans[i].Rollback = string(NoneRollback)
		}
	}

	tx.Add(func(tx *bun.Tx) error {
		_, err := t.SubtaskDao.BatchInsert(ctx, subtaskBeans, tx)
		return err
	})

	if err := tx.Submit(); err != nil {
		return err
	}

	if task.task.Urgent {
		taskID := taskBean.ID
		logger.Debug("[SubmitTask] urgent task %s, notifying leader %s (myName=%s)", taskID, t.Cluster.GetLeaderName(), t.Cluster.GetMyName())
		t.notifyLeaderHandleTaskImmediately(ctx, taskID)
	}

	return nil
}

func SubmitTaskWithTx(task *Task, tx *bun.Tx) error {
	return SingletonTaskDispatcher.SubmitTaskWithTx(golocalv1.GetContext(), task, tx)
}

func (t *taskDispatcher) SubmitTaskWithTx(ctx context.Context, task *Task, tx *bun.Tx) error {
	taskBean, subtaskBeans := task.convert2Bean()

	// if not rollback executor, set rollback to NoneRollback（与 SubmitTask 保持一致）
	for i, subtask := range subtaskBeans {
		if task.em.getRollbackProvider(taskBean.TaskName, subtask.TaskName) == nil {
			subtaskBeans[i].Rollback = string(NoneRollback)
		}
	}

	_, err := t.TaskDao.Insert(ctx, taskBean, tx)
	if err != nil {
		return err
	}
	_, err = t.SubtaskDao.BatchInsert(ctx, subtaskBeans, tx)
	if err != nil {
		return err
	}
	return err
}

func (t *taskDispatcher) GetTaskOutput(ctx context.Context, taskID string) (outputs map[string]Output, err error) {
	outputs = make(map[string]Output)

	var (
		taskBak     *model.TaskBak
		subtaskBaks []model.SubtaskBak
		task        *model.Task
		subtasks    []model.Subtask
	)

	taskBak, err = t.TaskBakDao.GetByID(ctx, taskID)
	if err != nil {
		return
	}

	if taskBak != nil {
		output := Output{}
		_ = tools.Unmarshal([]byte(taskBak.Output), &output)
		outputs[taskBak.TaskName] = output
	} else {
		task, err = t.TaskDao.GetByID(ctx, taskID)
		if err != nil {
			return
		}
		output := Output{}
		_ = tools.Unmarshal([]byte(task.Output), &output)
		outputs[task.TaskName] = output
	}

	subtaskBaks, err = t.SubtaskBakDao.GetByTaskID(ctx, taskID)
	if err != nil {
		return
	}

	if len(subtaskBaks) > 0 {
		for _, subtask := range subtaskBaks {
			output := Output{}
			_ = tools.Unmarshal([]byte(subtask.Output), &output)
			outputs[subtask.TaskName] = output
		}
	} else {
		subtasks, err = t.SubtaskDao.GetByTaskID(ctx, taskID)
		if err != nil {
			return
		}
		for _, subtask := range subtasks {
			output := Output{}
			_ = tools.Unmarshal([]byte(subtask.Output), &output)
			outputs[subtask.TaskName] = output
		}
	}

	return
}

func (t *taskDispatcher) handleTask(ctx context.Context) {
	// Query tasks that need to be executed within the next 2 minutes
	now := time.Now()
	endTime := basic.NewFromTime(now.Add(2 * time.Minute))

	// Get tasks by filter
	tasks, err := t.TaskDao.GetTodoTask(ctx, []string{string(TaskPending), string(TaskRunning), string(TaskSubtaskRunning)}, endTime)
	if err != nil {
		logger.Error("[MasterCall] get tasks failed. err: %v", err)
		return
	}
	logger.Info("[handleTask] found %d todo tasks", len(tasks))
	if len(tasks) == 0 {
		return
	}

	// 清理 inQueueTasks 中已完成的任务（防止内存泄漏）
	t.inQueueTasks.Range(func(key, _ interface{}) bool {
		taskID := key.(string)
		for _, task := range tasks {
			if task.ID == taskID && isFinished(task.State) {
				t.inQueueTasks.Delete(key)
			}
		}
		return true
	})

	// Add task IDs to delay queue in batches
	var immediateTasks []string
	var scheduledTasks []struct {
		taskID      string
		executeTime time.Time
	}

	for _, task := range tasks {
		if _, ok := t.inQueueTasks.Load(task.ID); ok {
			continue
		}
		t.inQueueTasks.Store(task.ID, true)
		if !task.ExecuteTime.IsZero() {
			// Scheduled task
			scheduledTasks = append(scheduledTasks, struct {
				taskID      string
				executeTime time.Time
			}{task.ID, task.ExecuteTime.Time()})
		} else {
			// Immediate task
			immediateTasks = append(immediateTasks, task.ID)
		}
	}

	// Add immediate tasks as batch
	if len(immediateTasks) > 0 {
		logger.Trace("[handleTask] add immediate tasks %v to delayQueue, executeTime = %s", immediateTasks, now.Format("2006-01-02 15:04:05.000"))
		t.delayQueue.Add(immediateTasks, now)
	}

	// Add scheduled tasks
	for _, task := range scheduledTasks {
		logger.Trace("[MasterCall] add scheduled task %v, executeTime = %s", task.taskID, task.executeTime.Format("2006-01-02 15:04:05.000"))
		t.delayQueue.Add([]string{task.taskID}, task.executeTime)
	}
}

func (t *taskDispatcher) analysisTask(ctx context.Context, task *Task, subtaskMap map[string]*Subtask) (finished, retry bool, runningSubtasks []*model.Subtask, rollbackSubtasks []*model.Subtask) {
	// sync task status to db
	if task.IsFinished() {
		logger.Trace("[analysisTask] task=%s already finished, dbState=%s", task.GetID(), task.getState())
		finished = true
		// 同步 DB 状态：如果内存中判断已完成但 DB 状态未更新，需要更新 DB
		hasFailed := false
		for _, subtask := range subtaskMap {
			if subtask.GetState() == string(TaskFailed) {
				hasFailed = true
				break
			}
		}
		if hasFailed {
			_, _ = t.TaskDao.SetState(ctx, task.GetID(), string(TaskFailed))
			task.task.State = string(TaskFailed)
		} else {
			_, _ = t.TaskDao.SetState(ctx, task.GetID(), string(TaskSucceeded))
			task.task.State = string(TaskSucceeded)
		}
		logger.Trace("[analysisTask] task=%s synced DB state to %s", task.GetID(), task.getState())
		return
	}

	// 处理分支选择：检查已完成的节点是否有分支，执行条件并 Skip 未选中的分支目标
	t.processBranches(ctx, task)

	// ========== 回滚检测与执行 ==========
	rollbackableSubtasks := task.GetRollbackableSubtasks()
	logger.Trace("[analysisTask] task=%s rollbackableSubtasks=%v", task.GetID(), rollbackableSubtasks)
	for _, subtaskID := range rollbackableSubtasks {
		subtaskFromDB := subtaskMap[subtaskID]
		if subtaskFromDB != nil && t.canExecuteSubtask(subtaskFromDB, true) {
			rollbackSubtasks = append(rollbackSubtasks, subtaskFromDB.getModel())
		}
	}

	rollbackableIDSet := make(map[string]bool)
	for _, id := range rollbackableSubtasks {
		rollbackableIDSet[id] = true
	}

	// 判断回滚是否真正触发：只有 state=failed 的子任务才表示重试已耗尽、需要回滚
	// state=pending + retry>0 表示还在重试中，不应触发回滚
	rollbackTriggered := false
	hasRetrying := false
	for _, subtask := range subtaskMap {
		if subtask.GetState() == string(TaskFailed) && subtask.hasRollbackExecutor() {
			rollbackTriggered = true
			hasRetrying = subtask.subtask.Retry > 0
		}
	}

	if rollbackTriggered && !hasRetrying {
		// 回滚已触发：设置所有相关子任务的 rollback = rollback_pending
		for _, subtask := range subtaskMap {
			rb := subtask.getRollback()
			if rb != string(NoneRollback) && rb != "" {
				continue // 已有 rollback 状态，不覆盖
			}
			st := subtask.GetState()
			if (st == string(TaskSucceeded) || st == string(TaskFailed)) && subtask.hasRollbackExecutor() {
				// 终态子任务 + 有 rollback executor → 需要回滚
				//_ = t.SubtaskDao.SetRollbackAndState(ctx, subtask.GetID(), string(RollbackPending), subtask.subtask.Output)
				//subtask.subtask.Rollback = string(RollbackPending)
				logger.Info("[analysisTask] task=%s set %s rollback=rollback_pending (terminal state=%s)", task.GetID(), subtask.GetID(), st)
			}
		}

		// 叶子优先回滚：构建正向依赖（subtaskID → 依赖它的子任务）
		dependsOn := make(map[string][]string)
		for _, subtask := range subtaskMap {
			if subtask.subtask.PreSubtaskID != "" {
				for _, preID := range strings.Split(subtask.subtask.PreSubtaskID, ",") {
					preID = strings.TrimSpace(preID)
					if preID != "" {
						dependsOn[preID] = append(dependsOn[preID], subtask.GetID())
					}
				}
			}
		}
		// 只保留依赖都已回滚完成的叶子
		var leafRollbackSubtasks []*model.Subtask
		for _, subtask := range rollbackSubtasks {
			isLeaf := true
			for _, depID := range dependsOn[subtask.ID] {
				depSubtask := subtaskMap[depID]
				if depSubtask != nil && rollbackableIDSet[depID] && !isRollbackFinished(depSubtask.getRollback()) {
					isLeaf = false
					break
				}
			}
			if isLeaf {
				leafRollbackSubtasks = append(leafRollbackSubtasks, subtask)
			}
		}
		rollbackSubtasks = leafRollbackSubtasks

		if len(rollbackSubtasks) > 0 {
			logger.Trace("[analysisTask] task=%s dispatching %d leaf rollback subtasks (of %d rollbackable)", task.GetID(), len(rollbackSubtasks), len(rollbackableSubtasks))
			return
		}

		logger.Info("[analysisTask] task=%s all rollbacks done or no leaves, checking allDone", task.GetID())
		_, _ = t.TaskDao.SetState(ctx, task.GetID(), string(TaskFailed))
		task.task.State = string(TaskFailed)
		return
	}

	// 获取下一个可执行的子任务
	nextPendingSubTasks := task.NextSubTasks()
	logger.Trace("[analysisTask] task=%s nextPendingSubTasks=%d", task.GetID(), len(nextPendingSubTasks))
	if len(nextPendingSubTasks) > 0 {
		for _, subtask := range nextPendingSubTasks {
			subtaskFromDB := subtaskMap[subtask.GetID()]
			canExec := t.canExecuteSubtask(subtaskFromDB, false)
			logger.Trace("[analysisTask] task=%s subtask=%s canExec=%v state=%s", task.GetID(), subtask.GetID(), canExec, subtaskFromDB.GetState())
			if canExec {
				runningSubtasks = append(runningSubtasks, subtaskFromDB.getModel())
			}
		}

		return
	}

	return
}

// processBranches 处理分支选择逻辑
// 检查已完成的节点是否有分支定义，执行条件函数，并自动 Skip 未选中的分支目标节点
func (t *taskDispatcher) processBranches(ctx context.Context, task *Task) {
	compiled := task.getCompiled()
	if compiled == nil {
		return
	}

	for nodeKey, branches := range compiled.GetBranchesMap() {
		// 检查分支源节点是否已完成
		node := task.dag.nodes[nodeKey]
		if node == nil || node.state != NodeSucceeded {
			continue
		}

		for _, branch := range branches {
			if branch.Condition == nil {
				continue
			}

			// 获取分支源节点的输出作为条件输入
			ch := compiled.GetChannel(nodeKey)
			var input any
			if ch != nil {
				data, _, _ := ch.get()
				input = data
			}

			// 执行条件函数
			selectedKey, err := branch.Condition(nil, input)
			if err != nil {
				logger.Error("[processBranches] branch condition for node %s failed: %v", nodeKey, err)
				continue
			}

			// Skip 未选中的分支目标节点
			for endKey := range branch.EndNodes {
				if endKey == selectedKey {
					continue
				}
				// 检查目标节点是否还是 Pending 状态
				endNode := task.dag.nodes[endKey]
				if endNode != nil && endNode.state == NodePending {
					_ = task.SkipSubtask(endKey)
					// 同步更新 DB 中的子任务状态为 Skipped
					err = t.SubtaskDao.SetOutputAndState(ctx, endKey, "", string(TaskSkipped))
					if err != nil {
						logger.Error("[processBranches] failed to update DB state for skipped subtask %s: %v", endKey, err)
					}
					logger.Info("[processBranches] skipped unselected branch target %s (selected: %s)", endKey, selectedKey)
				}
			}
		}
	}
}

func (t *taskDispatcher) canExecuteSubtask(subtask *Subtask, isRollback bool) bool {
	now := time.Now()
	retryTime := subtask.getLastRunTime().Time().Add(time.Duration(subtask.getRetryInterval()) * time.Second)
	if !now.After(retryTime) {
		return false
	}

	if isRollback {
		return !subtask.isRollbackFinished()
	}
	return !subtask.IsFinished()
}

func (t *taskDispatcher) allocateWorker(ctx context.Context, _runningTasks []*model.Task, _runningSubtasks, _runningSubtaskRollbacks []*model.Subtask, taskAffinityMap map[string]affinity) {
	if len(_runningTasks) == 0 && len(_runningSubtasks) == 0 && len(_runningSubtaskRollbacks) == 0 {
		return
	}

	if !t.Cluster.IsReady() {
		logger.Warn("[allocateWorker] cluster not ready, skip delivering tasks")
		return
	}

	runningTasks := t.filterInflightTasks(_runningTasks)
	runningSubtasks := t.filterInflightSubtasks(_runningSubtasks)
	runningSubtaskRollbacks := t.filterInflightSubtasks(_runningSubtaskRollbacks)

	logger.Trace("[allocateWorker] after inflight filter: tasks=%d subtasks=%d rollbacks=%d", len(runningTasks), len(runningSubtasks), len(runningSubtaskRollbacks))

	if len(runningTasks) == 0 && len(runningSubtasks) == 0 && len(runningSubtaskRollbacks) == 0 {
		return
	}

	defer func() {
		// 清除分配中标志
		t.deleteInflightSubtasks(runningSubtasks)
		t.deleteInflightSubtasks(runningSubtaskRollbacks)
		t.deleteInflightTasks(runningTasks)
	}()

	// 注意：不在 allocateWorker 中清理 inflight，而是在 handleTaskImmediately 中
	// 根据 DB 状态清理已完成的任务，避免在 dispatch 阶段过早清理导致重复分配

	subtaskWorkerMap := make(map[string][]string)
	subtaskRollbackWorkerMap := make(map[string][]string)
	taskWorkerMap := make(map[string][]string)

	getAffinity := func(taskID string) affinity {
		affinityConf, exists := taskAffinityMap[taskID]
		if !exists {
			affinityConf = affinity{
				Type:   AffinityRandom,
				Worker: "",
			}
		}
		return affinityConf
	}

	// 先分配任务执行节点（子任务可能需要依赖 task 的 worker 做 affinity）
	t.allocateItems(ctx, len(runningTasks), func(i int) (taskID, itemID, currentWorker, affinityNode string) {
		return runningTasks[i].ID, runningTasks[i].ID, runningTasks[i].Worker, runningTasks[i].Worker
	}, func(ctx context.Context, i int, nodeName string) (int64, error) {
		return t.TaskDao.SetWorkerAndTaskStateWithOldWorker(ctx, runningTasks[i].ID, nodeName, string(TaskRunning), runningTasks[i].Worker)
	}, getAffinity, taskWorkerMap)

	// 构建 taskWorker 查找表（task 分配完 worker 后，子任务可以参考）
	taskAllocatedWorker := make(map[string]string) // taskID -> allocated worker
	for nodeName, taskIDs := range taskWorkerMap {
		for _, tid := range taskIDs {
			taskAllocatedWorker[tid] = nodeName
		}
	}
	// 同时从 DB 中已有 worker 的 task 获取
	for i := range runningTasks {
		if runningTasks[i].Worker != "" {
			taskAllocatedWorker[runningTasks[i].ID] = runningTasks[i].Worker
		}
	}

	// 分配子任务执行节点
	// 注意：currentWorker 是 DB 中的实际 worker（用于 CAS 条件），affinityNode 是亲和性提示（用于选节点）
	t.allocateItems(ctx, len(runningSubtasks), func(i int) (taskID, itemID, currentWorker, affinityNode string) {
		// affinityNode：如果子任务没有 worker，使用 task 的 worker 作为亲和性提示
		affNode := runningSubtasks[i].Worker
		if affNode == "" {
			if tw, ok := taskAllocatedWorker[runningSubtasks[i].TaskID]; ok {
				affNode = tw
			}
		}
		return runningSubtasks[i].TaskID, runningSubtasks[i].ID, runningSubtasks[i].Worker, affNode
	}, func(ctx context.Context, i int, nodeName string) (int64, error) {
		return t.SubtaskDao.SetWorkerAndStateWithOldWorker(ctx, runningSubtasks[i].ID, nodeName, string(TaskRunning), runningSubtasks[i].Worker)
	}, getAffinity, subtaskWorkerMap)

	// 分配回滚任务执行节点
	t.allocateItems(ctx, len(runningSubtaskRollbacks), func(i int) (taskID, itemID, currentWorker, affinityNode string) {
		return runningSubtaskRollbacks[i].TaskID, runningSubtaskRollbacks[i].ID, runningSubtaskRollbacks[i].Worker, runningSubtaskRollbacks[i].Worker
	}, func(ctx context.Context, i int, nodeName string) (int64, error) {
		return t.SubtaskDao.SetWorkerAndRollbackWithOldWorker(ctx, runningSubtaskRollbacks[i].ID, nodeName, string(RollingBack), runningSubtaskRollbacks[i].Worker)
	}, getAffinity, subtaskRollbackWorkerMap)

	t.deliverToCluster(ctx, subtaskWorkerMap, deliverSubtask)
	//t.deliverToCluster(ctx, taskWorkerMap, deliverTask)
	t.deliverToCluster(ctx, subtaskRollbackWorkerMap, deliverSubtaskRollback)
}

// allocateItemInfo 获取待分配项的信息
// 返回: taskID, itemID, currentWorker(DB中的实际worker，用于CAS条件), affinityNode(亲和性提示节点，用于选节点)
type allocateItemInfo func(i int) (taskID, itemID, currentWorker, affinityNode string)

// allocateItemCAS 尝试 CAS 更新 worker，返回 (affectedRows, error)
type allocateItemCAS func(ctx context.Context, i int, nodeName string) (int64, error)

// allocateItems 通用的节点分配逻辑，消除 subtask/task/rollback 三段重复代码
func (t *taskDispatcher) allocateItems(ctx context.Context, count int, getInfo allocateItemInfo, casUpdate allocateItemCAS, getAffinity func(string) affinity, workerMap map[string][]string) {
	for i := 0; i < count; i++ {
		taskID, itemID, currentWorker, affinityNode := getInfo(i)
		affinityConf := getAffinity(taskID)

		nodeName := t.selectNodeByAffinity(affinityConf.Type, affinityConf.Worker, affinityNode)
		if nodeName == "" {
			logger.Warn("[allocateItems] no available node for item %s", itemID)
			continue
		}
		if nodeName != currentWorker {
			cnt, err := casUpdate(ctx, i, nodeName)
			if err != nil {
				logger.Error("[allocateItems] set worker for item %s failed. err: %v", itemID, err)
				continue
			}
			if cnt == 0 {
				logger.Warn("[allocateItems] item %s CAS failed (worker changed), currentWorker=%s newNode=%s", itemID, currentWorker, nodeName)
				continue
			}
			logger.Trace("[allocateItems] item %s allocated to %s (was %s)", itemID, nodeName, currentWorker)
		} else {
			logger.Trace("[allocateItems] item %s already on %s", itemID, nodeName)
		}
		workerMap[nodeName] = append(workerMap[nodeName], itemID)
	}
}

func (t *taskDispatcher) filterInflightTasks(tasks []*model.Task) []model.Task {
	filtered := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		if t.allocateWorkerInflight.InsertString(task.ID) {
			filtered = append(filtered, *task)
		}
	}
	return filtered
}

func (t *taskDispatcher) filterInflightSubtasks(subtasks []*model.Subtask) []model.Subtask {
	filtered := make([]model.Subtask, 0, len(subtasks))
	for _, subtask := range subtasks {
		if t.allocateWorkerInflight.InsertString(subtask.ID) {
			filtered = append(filtered, *subtask)
		}
	}
	return filtered
}

func (t *taskDispatcher) deleteInflightTasks(tasks []model.Task) {
	for _, task := range tasks {
		t.allocateWorkerInflight.DeleteString(task.ID)
	}
}

func (t *taskDispatcher) deleteInflightSubtasks(subtasks []model.Subtask) {
	for _, task := range subtasks {
		t.allocateWorkerInflight.DeleteString(task.ID)
	}
}

func (t *taskDispatcher) selectNodeByAffinity(taskAffinityType TaskAffinityType, primaryWorker string, currentNode string) string {
	aliveNodes, lostNodes := t.Cluster.GetAliveNodeNames(), t.Cluster.GetLostNodeNames()

	if len(aliveNodes) == 0 {
		logger.Warn("[selectNode] no alive nodes available")
		return ""
	}

	switch taskAffinityType {
	case AffinityForceSameNode:
		if primaryWorker != "" && tools.StringSliceContains(aliveNodes, primaryWorker) && !tools.StringSliceContains(lostNodes, primaryWorker) {
			return primaryWorker
		}
		if currentNode != "" && tools.StringSliceContains(aliveNodes, currentNode) && !tools.StringSliceContains(lostNodes, currentNode) {
			return currentNode
		}
		// ForceSameNode 但没有指定 primaryWorker 且没有当前 worker，随机选一个
		return aliveNodes[t.randSource.Intn(len(aliveNodes))]
	case AffinityPreferSameNode:
		if primaryWorker != "" && tools.StringSliceContains(aliveNodes, primaryWorker) && !tools.StringSliceContains(lostNodes, primaryWorker) {
			return primaryWorker
		}
		if currentNode != "" && !tools.StringSliceContains(lostNodes, currentNode) {
			return currentNode
		}
		return aliveNodes[t.randSource.Intn(len(aliveNodes))]
	case AffinityRandom:
		fallthrough
	default:
		if currentNode != "" && !tools.StringSliceContains(lostNodes, currentNode) {
			return currentNode
		}
		return aliveNodes[t.randSource.Intn(len(aliveNodes))]
	}
}

func (t *taskDispatcher) deliverToCluster(ctx context.Context, workerMap map[string][]string, method string) {
	if len(workerMap) == 0 {
		return
	}

	logger.Trace("[deliverToCluster] method=%s workerMap=%v", method, workerMap)

	var wg sync.WaitGroup
	for nodeName, ids := range workerMap {
		if nodeName == t.Cluster.GetMyName() {
			t.deliverLocal(ctx, method, ids)
			continue
		}

		wg.Add(1)
		go func(nodeName string, ids []string) {
			defer wg.Done()

			conn, err := t.Cluster.GetGRPCClient(nodeName)
			if err != nil {
				logger.Error("[deliverToCluster] get gRPC client for node '%s' failed. err: %v", nodeName, err)
				return
			}

			client := proto.NewTaskXServiceClient(conn)
			callCtx, cancel := context.WithTimeout(ctx, t.cfg.RemoteCallTimeout)
			traceID := golocalv1.GetTraceID()

			switch method {
			case deliverTask:
				_, err = client.DeliverTask(callCtx, &proto.DeliverRequest{Ids: ids, TraceId: traceID})
			case deliverSubtask:
				_, err = client.DeliverSubtask(callCtx, &proto.DeliverRequest{Ids: ids, TraceId: traceID})
			case deliverSubtaskRollback:
				_, err = client.DeliverSubtaskRollback(callCtx, &proto.DeliverRequest{Ids: ids, TraceId: traceID})
			default:
				logger.Error("[deliverToCluster] unknown deliver method: %s", method)
			}
			cancel()

			if err != nil {
				logger.Error("[deliverToCluster] deliver %s to node '%s' failed. err: %v", method, nodeName, err)
			}
		}(nodeName, ids)
	}
	wg.Wait()
}

func (t *taskDispatcher) deliverLocal(ctx context.Context, method string, ids []string) {
	receiver := t.TaskReceiver
	if receiver == nil {
		logger.Error("[deliverLocal] TaskReceiver is nil")
		return
	}

	var err error
	switch method {
	case deliverTask:
		err = receiver.deliverTask(ctx, ids)
	case deliverSubtask:
		err = receiver.deliverSubtask(ctx, ids)
	case deliverSubtaskRollback:
		err = receiver.deliverSubtaskRollback(ctx, ids)
	default:
		logger.Error("[deliverLocal] unknown deliver method: %s", method)
	}

	if err != nil {
		logger.Error("[deliverLocal] deliver %s failed. err: %v", method, err)
	}
}

func (t *taskDispatcher) notifyLeaderHandleTaskImmediately(ctx context.Context, taskID string) {
	logger.Trace("[notifyLeaderHandleTaskImmediately] taskID=%s leader=%s myName=%s", taskID, t.Cluster.GetLeaderName(), t.Cluster.GetMyName())
	if t.Cluster.GetLeaderName() == t.Cluster.GetMyName() {
		t.handleTaskImmediately(ctx, []string{taskID})
		return
	}

	traceID := golocalv1.GetTraceID()

	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		leaderName := t.Cluster.GetLeaderName()
		if leaderName == t.Cluster.GetMyName() {
			t.handleTaskImmediately(ctx, []string{taskID})
			return
		}

		conn, err := t.Cluster.GetGRPCClient(leaderName)
		if err != nil {
			lastErr = err
			if i < maxRetries-1 {
				logger.Warn("[notifyLeader] task %s get gRPC client for leader '%s' failed (attempt %d/%d). err: %v", taskID, leaderName, i+1, maxRetries, err)
				time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			}
			continue
		}

		client := proto.NewTaskXServiceClient(conn)
		callCtx, cancel := context.WithTimeout(ctx, t.cfg.RemoteCallTimeout)
		_, err = client.HandleTaskImmediately(callCtx, &proto.HandleTaskImmediatelyRequest{TaskIds: []string{taskID}, TraceId: traceID})
		cancel()
		if err == nil {
			logger.Info("[notifyLeader] task %s notified leader %s via gRPC successfully", taskID, leaderName)
			return
		}
		lastErr = err
		logger.Warn("[notifyLeader] task %s gRPC call to leader '%s' failed (attempt %d/%d). err: %v", taskID, leaderName, i+1, maxRetries, err)
		if i < maxRetries-1 {
			logger.Warn("[notifyLeader] task %s remote call 'handleTaskImmediately' failed (attempt %d/%d). err: %v", taskID, i+1, maxRetries, err)
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	logger.Warn("[notifyLeader] task %s remote call 'handleTaskImmediately' failed after %d attempts. err: %v", taskID, maxRetries, lastErr)
}

func (t *taskDispatcher) handleTaskImmediately(ctx context.Context, taskIDs []string) {
	logger.Info("[handleTaskImmediately] tasks %v, isReady=%v, isLeader=%v", taskIDs, t.Cluster.IsReady(), t.Cluster.IsLeader())

	if !t.Cluster.IsReady() {
		logger.Warn("[handleTaskImmediately] cluster not ready, skip")
		return
	}
	if !t.Cluster.IsLeader() {
		logger.Warn("[handleTaskImmediately] not leader, skip")
		return
	}

	// Get all tasks
	tasks, err := t.TaskDao.GetByIDs(ctx, taskIDs)
	if err != nil {
		logger.Error("[handleTaskImmediately] get tasks by IDs failed. err: %v", err)
		return
	}
	logger.Trace("[handleTaskImmediately] got %d tasks from DB", len(tasks))

	var (
		runningTasks     []*model.Task
		runningSubtasks  []*model.Subtask
		rollbackSubtasks []*model.Subtask
	)

	taskAffinityMap := make(map[string]affinity)
	for i := range tasks {
		dbTask := &tasks[i]

		// 跳过已完成的任务
		if isFinished(dbTask.State) {
			logger.Trace("[handleTaskImmediately] task=%s state=%s is finished, skip", dbTask.ID, dbTask.State)
			continue
		}

		subtasks, err := t.SubtaskDao.GetByTaskID(ctx, dbTask.ID)
		if err != nil {
			logger.Error("[handleTaskImmediately] task=%s GetByTaskID failed: %v", dbTask.ID, err)
			continue
		}

		logger.Trace("[handleTaskImmediately] task=%s state=%s got %d subtasks from DB", dbTask.ID, dbTask.State, len(subtasks))

		task := t.getOrInitTask(ctx, dbTask, subtasks)
		if task == nil {
			continue
		}
		taskAffinityMap[task.GetID()] = affinity{
			Type:   task.getAffinityType(),
			Worker: task.getPrimaryWorker(),
		}

		finished, retry, running, rollback := t.analysisTask(ctx, task, task.subtaskMap)
		logger.Trace("[handleTaskImmediately] task=%s state=%s finished=%v retry=%v running=%d rollback=%d", task.GetID(), dbTask.State, finished, retry, len(running), len(rollback))
		if retry {
			continue
		}

		// task 从 Pending → SubtaskRunning，需要分配 worker 并 deliver
		if dbTask.State == string(TaskPending) {
			runningTasks = append(runningTasks, dbTask)
		}

		if len(running) > 0 {
			runningSubtasks = append(runningSubtasks, running...)
		} else if len(rollback) > 0 {
			rollbackSubtasks = append(rollbackSubtasks, rollback...)
		}
	}

	// Batch allocate workers
	logger.Info("[handleTaskImmediately] allocateWorker: tasks=%d subtasks=%d rollbacks=%d", len(runningTasks), len(runningSubtasks), len(rollbackSubtasks))
	if len(runningTasks) > 0 || len(runningSubtasks) > 0 || len(rollbackSubtasks) > 0 {
		t.allocateWorker(ctx, runningTasks, runningSubtasks, rollbackSubtasks, taskAffinityMap)
	}
}

// getOrInitTask 获取或初始化 Task（带缓存），避免每次调度都重建 DAG
func (t *taskDispatcher) getOrInitTask(ctx context.Context, dbTask *model.Task, subtasks []model.Subtask) *Task {
	// 检查缓存
	if cached, ok := t.taskCache.Load(dbTask.ID); ok {
		ct := cached.(*cachedTask)
		// 缓存未过期，用 DB 中的最新状态刷新子任务状态
		if time.Since(ct.createdAt) < taskCacheTTL {
			t.refreshSubtaskStates(ct.task, subtasks)
			return ct.task
		}
		// 缓存过期，删除
		t.taskCache.Delete(dbTask.ID)
	}

	// 缓存未命中，重建 Task
	task := &Task{}
	task, err := task.initByBean(dbTask, subtasks)
	if err != nil {
		logger.Error("[getOrInitTask] task %s initByBean failed. err: %v", dbTask.ID, err)
		return nil
	}

	// 写入缓存
	t.taskCache.Store(dbTask.ID, &cachedTask{
		task:      task,
		createdAt: time.Now(),
	})

	return task
}

// refreshSubtaskStates 用 DB 中的最新状态刷新缓存 Task 的子任务状态
// 同时同步 DAG 节点状态和 channel 通知，确保 GetExecutableNodes 返回正确结果
func (t *taskDispatcher) refreshSubtaskStates(task *Task, subtasks []model.Subtask) {
	for _, dbSubtask := range subtasks {
		cached, ok := task.subtaskMap[dbSubtask.ID]
		if !ok {
			continue
		}

		oldState := cached.subtask.State
		cached.subtask.State = dbSubtask.State
		cached.subtask.Output = dbSubtask.Output
		cached.subtask.Worker = dbSubtask.Worker
		cached.subtask.Rollback = dbSubtask.Rollback
		cached.subtask.Retry = dbSubtask.Retry
		cached.subtask.LastRunTime = dbSubtask.LastRunTime

		// 状态未变化，无需更新 DAG
		if oldState == dbSubtask.State {
			continue
		}

		logger.Trace("[refreshSubtaskStates] task=%s subtask=%s oldState=%s newState=%s", task.GetID(), dbSubtask.ID, oldState, dbSubtask.State)

		// 同步 DAG 节点状态和 channel 通知
		switch dbSubtask.State {
		case string(TaskPending):
			// retry 后状态从 running → pending，DAG 节点需要回退到 NodePending 以便重新执行
			_ = task.dag.UpdateNodeState(dbSubtask.ID, NodePending)
		case string(TaskSucceeded):
			if err := task.dag.UpdateNodeState(dbSubtask.ID, NodeSucceeded); err != nil {
				logger.Error("[refreshSubtaskStates] UpdateNodeState failed: %v", err)
			} else {
				logger.Trace("[refreshSubtaskStates] updated DAG node %s to Succeeded, node.state=%v", dbSubtask.ID, task.dag.GetNode(dbSubtask.ID).state)
			}
			if ch := task.compiled.GetChannel(dbSubtask.ID); ch != nil {
				ch.reportDependencies(nil)
				ch.reportValues(map[string]any{dbSubtask.ID: dbSubtask.Output})
			}
			for _, succKey := range task.dag.controlAdj[dbSubtask.ID] {
				if ch := task.compiled.GetChannel(succKey); ch != nil {
					ch.reportDependencies([]string{dbSubtask.ID})
				}
			}
			for _, succKey := range task.dag.dataAdj[dbSubtask.ID] {
				if ch := task.compiled.GetChannel(succKey); ch != nil {
					ch.reportValues(map[string]any{dbSubtask.ID: dbSubtask.Output})
				}
			}
		case string(TaskFailed):
			if err := task.dag.UpdateNodeState(dbSubtask.ID, NodeFailed); err != nil {
				logger.Error("[refreshSubtaskStates] UpdateNodeState to Failed failed: %v", err)
			} else {
				logger.Trace("[refreshSubtaskStates] updated DAG node %s to Failed", dbSubtask.ID)
			}
		case string(TaskSkipped):
			_ = task.dag.UpdateNodeState(dbSubtask.ID, NodeSkipped)
		case string(TaskRunning):
			_ = task.dag.UpdateNodeState(dbSubtask.ID, NodeRunning)
		}
	}
}
