package taskx

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caiflower/common-tools/cluster"
	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/inflight"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/taskx/dao"
)

func commonCluster() (cluster1, cluster2, cluster3 *cluster.Cluster) {
	c1 := cluster.Config{Enable: "true"}
	c2 := cluster.Config{Enable: "true"}
	c3 := cluster.Config{Enable: "true"}

	c1.Nodes = append(c1.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c2.Nodes = append(c2.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c3.Nodes = append(c3.Nodes,
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost1",
			Port: 8080,
		},
		&struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost2",
			Port: 8081,
		}, &struct {
			Name  string
			Ip    string
			Port  int
			Local bool
		}{
			Ip:   "127.0.0.1",
			Name: "localhost3",
			Port: 8082,
		})

	c1.Nodes[0].Local = true
	cluster1, err := cluster.NewClusterWithArgs(c1, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	c2.Nodes[1].Local = true
	cluster2, err = cluster.NewClusterWithArgs(c2, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	c3.Nodes[2].Local = true
	cluster3, err = cluster.NewClusterWithArgs(c3, logger.NewLogger(&logger.Config{
		Level: "DebugLevel",
	}))
	if err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)

	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster1.GetMyName(), cluster1.GetMyTerm(), cluster1.GetLeaderName(), cluster1.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster2.GetMyName(), cluster1.GetMyTerm(), cluster2.GetLeaderName(), cluster2.IsReady())
	fmt.Printf("clusterName: %s term:%d leader: %s isready: %v\n", cluster3.GetMyName(), cluster1.GetMyTerm(), cluster3.GetLeaderName(), cluster3.IsReady())
	return cluster1, cluster2, cluster3
}

const (
	taskName         = "taskDemo"
	taskRollbackName = "taskRollbackDemo"

	stepOne   = "stepOne"
	stepTwo   = "stepTwo"
	stepThree = "stepThree"
	stepFour  = "stepFour"
	stepFive  = "stepFive"
)

type TaskDemo struct {
}

func (t *TaskDemo) Name() string {
	return taskName
}

func (t *TaskDemo) FinishedTask(data *TaskData) (retry bool, err error) {
	logger.Info("FinishedTask")
	return false, nil
}
func (t *TaskDemo) FailedTask(data *TaskData) (retry bool, err error) {
	return false, errors.New("FailedTask")
}

func (t *TaskDemo) GetExecutor() (TaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
		stepOne:   t.StepOne,
		stepTwo:   t.StepTwo,
		stepThree: t.StepThree,
		stepFour:  t.StepFour,
		stepFive:  t.StepFive,
	}
}

func (t *TaskDemo) GetExecutorWithRollback() (TaskExecutor, map[string]SubTaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
			stepOne:   t.StepOne,
			stepTwo:   t.StepTwo,
			stepThree: t.StepThree,
			stepFour:  t.StepFour,
			stepFive:  t.StepFive,
		}, map[string]SubTaskExecutor{
			stepOne: t.StepOneRollback,
		}
}

func (t *TaskDemo) StepOne(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step one")
	return false, data.Input, err
}

func (t *TaskDemo) StepOneRollback(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step one rollback")
	return false, data.Input, err
}

func (t *TaskDemo) StepTwo(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step two")
	return false, "", nil
}

func (t *TaskDemo) StepThree(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step three")
	return false, data.Input, err
}

func (t *TaskDemo) StepFour(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step four")
	return false, data.Input, err
}

func (t *TaskDemo) StepFive(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step five")
	return false, data.Input, err
}

func commonTaskx(cluster1, cluster2, cluster3 cluster.ICluster) (dispatcher1, dispatcher2, dispatcher3 *taskDispatcher, receiver1, receiver2, receiver3 *taskReceiver) {
	config := dbv1.Config{
		Url:          "proxysql.app.svc.cluster.local:6033",
		User:         "test-user",
		Password:     "test-user",
		DbName:       "task_test",
		Debug:        true,
		EnableMetric: true,
	}

	l := logger.Config{
		Level: logger.DebugLevel,
	}

	logger.InitLogger(&l)

	client, err := dbv1.NewDBClient(config)
	if err != nil {
		panic(err)
	}

	taskDao := &taskxdao.TaskDao{
		IDB: client,
	}
	subtaskDao := &taskxdao.SubtaskDao{
		IDB: client,
	}
	cfg := &Config{
		RemoteCallTimout:     time.Second * 3,
		BackupTaskAgeSeconds: 120,
	}

	receiver1 = &taskReceiver{
		Cluster:                  cluster1,
		TaskDao:                  taskDao,
		SubtaskDao:               subtaskDao,
		subtaskInflight:          inflight.NewInFlight(),
		taskInflight:             inflight.NewInFlight(),
		subtaskWorker:            50,
		taskWorker:               5,
		subtaskRollbackWorker:    10,
		taskQueueSize:            1000,
		subtaskQueueSize:         1000,
		subtaskRollbackQueueSize: 200,
		cfg:                      cfg,
	}
	receiver2 = &taskReceiver{
		Cluster:                  cluster2,
		TaskDao:                  taskDao,
		SubtaskDao:               subtaskDao,
		subtaskInflight:          inflight.NewInFlight(),
		taskInflight:             inflight.NewInFlight(),
		subtaskWorker:            50,
		taskWorker:               5,
		subtaskRollbackWorker:    10,
		taskQueueSize:            1000,
		subtaskQueueSize:         1000,
		subtaskRollbackQueueSize: 200,
		cfg:                      cfg,
	}
	receiver3 = &taskReceiver{
		Cluster:                  cluster3,
		TaskDao:                  taskDao,
		SubtaskDao:               subtaskDao,
		subtaskInflight:          inflight.NewInFlight(),
		taskInflight:             inflight.NewInFlight(),
		subtaskWorker:            50,
		taskWorker:               5,
		subtaskRollbackWorker:    10,
		taskQueueSize:            1000,
		subtaskQueueSize:         1000,
		subtaskRollbackQueueSize: 200,
		cfg:                      cfg,
	}
	dispatcher1 = &taskDispatcher{
		Cluster:                cluster1,
		TaskDao:                taskDao,
		SubtaskDao:             subtaskDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver1,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	dispatcher2 = &taskDispatcher{
		Cluster:                cluster2,
		TaskDao:                taskDao,
		SubtaskDao:             subtaskDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver2,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	dispatcher3 = &taskDispatcher{
		Cluster:                cluster3,
		TaskDao:                taskDao,
		SubtaskDao:             subtaskDao,
		DBClient:               client,
		cfg:                    cfg,
		TaskReceiver:           receiver3,
		allocateWorkerInflight: inflight.NewInFlight(),
	}
	receiver1.TaskDispatcher = dispatcher1
	receiver2.TaskDispatcher = dispatcher2
	receiver3.TaskDispatcher = dispatcher3

	return
}

func TestDisPatch(t *testing.T) {
	cluster1, cluster2, cluster3 := commonCluster()
	dispatcher1, dispatcher2, dispatcher3, receiver1, receiver2, receiver3 := commonTaskx(cluster1, cluster2, cluster3)

	demo := &TaskDemo{}
	RegisterTaskExecutor(demo.GetExecutor())

	receiver1.Start()
	receiver2.Start()
	receiver3.Start()

	tracker1 := cluster.NewDefaultJobTracker(5, cluster1, dispatcher1)
	tracker2 := cluster.NewDefaultJobTracker(5, cluster2, dispatcher2)
	tracker3 := cluster.NewDefaultJobTracker(5, cluster3, dispatcher3)
	tracker1.Start()
	tracker2.Start()
	tracker3.Start()

	go cluster1.StartUp()
	go cluster2.StartUp()
	go cluster3.StartUp()

	// 提交一个任务
	submitTask(dispatcher1)

	global.DefaultResourceManger.Signal()
}

func submitTask(dispatcher1 *taskDispatcher) {
	task := NewTask(taskName).SetRequestId("testTraceId").SetDescription("test").SetUrgent()
	one := NewSubtask(stepOne).SetInput("one")
	two := NewSubtask(stepTwo).SetInput("two")
	three := NewSubtask(stepThree).SetInput("three")
	four := NewSubtask(stepFour).SetInput("four")
	five := NewSubtask(stepFive).SetInput("five")
	err := task.AddSubTask(one)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(two)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(three)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(four)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(five)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(one, two)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(two, three)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(two, four)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(three, five)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(four, five)
	if err != nil {
		panic(err)
	}

	err = dispatcher1.SubmitTask(task)
	if err != nil {
		panic(err)
	}
}

type TaskRollbackDemo struct {
}

func (t *TaskRollbackDemo) Name() string {
	return taskRollbackName
}

func (t *TaskRollbackDemo) FinishedTask(data *TaskData) (retry bool, err error) {
	logger.Info("FinishedTask")
	return false, nil
}
func (t *TaskRollbackDemo) FailedTask(data *TaskData) (retry bool, err error) {
	return false, errors.New("FailedTask")
}

func (t *TaskRollbackDemo) GetExecutorWithRollback() (TaskExecutor, map[string]SubTaskExecutor, map[string]SubTaskExecutor) {
	return t, map[string]SubTaskExecutor{
			stepOne:   t.StepOne,
			stepTwo:   t.StepTwo,
			stepThree: t.StepThree,
			stepFour:  t.StepFour,
			stepFive:  t.StepFive,
		}, map[string]SubTaskExecutor{
			stepTwo:   t.StepTwoRollback,
			stepThree: t.StepThreeRollback,
			stepFour:  t.StepFourRollback,
		}
}

func (t *TaskRollbackDemo) StepOne(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step one")
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepOneRollback(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step one rollback")
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepTwo(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step two")
	return false, "", nil
}

func (t *TaskRollbackDemo) StepTwoRollback(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step two rollback")
	return false, data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepThree(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step three")
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepThreeRollback(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step three rollback")
	return false, data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepFour(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step four")
	return false, data.Input, err
}

func (t *TaskRollbackDemo) StepFourRollback(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step four rollback")
	return false, data.Input + " rollback", err
}

func (t *TaskRollbackDemo) StepFive(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step five")
	return false, data.Input, errors.New("test five err")
}

func TestDisPatchRollback(t *testing.T) {
	cluster1, cluster2, cluster3 := commonCluster()
	dispatcher1, dispatcher2, dispatcher3, receiver1, receiver2, receiver3 := commonTaskx(cluster1, cluster2, cluster3)
	demo := &TaskRollbackDemo{}
	RegisterTaskExecutorWithRollback(demo.GetExecutorWithRollback())

	receiver1.Start()
	receiver2.Start()
	receiver3.Start()

	tracker1 := cluster.NewDefaultJobTracker(5, cluster1, dispatcher1)
	tracker2 := cluster.NewDefaultJobTracker(5, cluster2, dispatcher2)
	tracker3 := cluster.NewDefaultJobTracker(5, cluster3, dispatcher3)
	tracker1.Start()
	tracker2.Start()
	tracker3.Start()

	go cluster1.StartUp()
	go cluster2.StartUp()
	go cluster3.StartUp()

	// 提交一个任务
	submitRollbackTask(taskRollbackName, dispatcher1)

	global.DefaultResourceManger.Signal()
}

func submitRollbackTask(taskName string, dispatcher1 *taskDispatcher) string {
	task := NewTask(taskName).SetRequestId("testTraceId").SetDescription("test").SetUrgent()
	one := NewSubtask(stepOne).SetInput("one")
	two := NewSubtask(stepTwo).SetInput("two")
	three := NewSubtask(stepThree).SetInput("three")
	four := NewSubtask(stepFour).SetInput("four")
	five := NewSubtask(stepFive).SetInput("five")
	err := task.AddSubTask(one)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(two)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(three)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(four)
	if err != nil {
		panic(err)
	}
	err = task.AddSubTask(five)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(one, two)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(two, three)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(two, four)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(three, five)
	if err != nil {
		panic(err)
	}
	err = task.AddDirectedEdge(four, five)
	if err != nil {
		panic(err)
	}

	err = dispatcher1.SubmitTask(task)
	if err != nil {
		panic(err)
	}

	return task.taskId
}

func TestGetTaskOutput(t *testing.T) {
	cluster1, cluster2, cluster3 := commonCluster()
	dispatcher1, dispatcher2, dispatcher3, receiver1, receiver2, receiver3 := commonTaskx(cluster1, cluster2, cluster3)
	demo := &TaskDemo{}
	RegisterTaskExecutorWithRollback(demo.GetExecutorWithRollback())

	receiver1.Start()
	receiver2.Start()
	receiver3.Start()
	defer receiver1.Close()
	defer receiver2.Close()
	defer receiver3.Close()

	tracker1 := cluster.NewDefaultJobTracker(5, cluster1, dispatcher1)
	tracker2 := cluster.NewDefaultJobTracker(5, cluster2, dispatcher2)
	tracker3 := cluster.NewDefaultJobTracker(5, cluster3, dispatcher3)
	tracker1.Start()
	tracker2.Start()
	tracker3.Start()
	defer tracker1.Close()
	defer tracker2.Close()
	defer tracker3.Close()

	go cluster1.StartUp()
	go cluster2.StartUp()
	go cluster3.StartUp()
	defer cluster1.Close()
	defer cluster2.Close()
	defer cluster3.Close()

	// 提交一个任务
	taskId := submitRollbackTask(taskName, dispatcher1)

	// 等待任务完成
	time.Sleep(60 * time.Second)

	outputMap, err := dispatcher1.GetTaskOutput(taskId)
	if err != nil {
		return
	}

	for k, output := range outputMap {
		fmt.Printf("k = %s, output = %v \n", k, output)
	}
}
