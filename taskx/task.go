package taskx

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/taskx/dao"
	"github.com/dominikbraun/graph"
)

type TaskState string
type TaskRollbackState string

const (
	TaskPending        TaskState = "Pending"
	TaskRunning        TaskState = "Running"
	TaskSubtaskRunning TaskState = "SubtaskRunning"
	TaskFailed         TaskState = "Failed"
	TaskSucceeded      TaskState = "Succeeded"

	RollingBack       TaskRollbackState = "RollingBack"
	RollbackSucceeded TaskRollbackState = "RollbackSucceeded"
	RollbackFailed    TaskRollbackState = "RollbackFailed"
	RollbackPending   TaskRollbackState = "RollbackPending"
	NoneRollback      TaskRollbackState = "NoneRollback"

	DefaultRetryCount  = 3
	DefaultRetryTimout = 3
)

type ITask interface {
	GetTaskId() string
	GetTaskName() string
	GetTaskState() TaskState
	SetTaskState(state TaskState)
	AllocateTaskId()
	SetInput(content interface{}) *Task
	GetInput() string
	SetDescription(description string) *Task
	GetDescription() string
	UnmarshalInput(v interface{}) error
	IsFinished() bool
	SetRequestId(string string) *Task
	SetRetry(retry int) *Task
	SetRetryInterval(retryInterval int) *Task
	SetUrgent() *Task

	AddSubTask(Task *SubTask) error                          // add SubTask
	AddDirectedEdge(src, dst *SubTask) error                 // add directedEdge
	NextSubTasks() []*SubTask                                // get next pending subTasks
	UpdateTaskState(state TaskState)                         // update Task state
	UpdateSubTaskState(taskId string, state TaskState) error // update subtaskSate
	Size() int                                               // directedEdge count
	Order() int                                              // SubTask count
	Graph() string                                           // graph
}

type ISubTask interface {
	GetTaskId() string
	GetTaskState() TaskState
	SetTaskState(state TaskState)
	GetTaskName() string
	AllocateTaskId()
	SetInput(input interface{}) *SubTask
	GetInput() string
	SetOutput(output interface{}) *SubTask
	SetRetry(retry int) *SubTask
	SetRetryInterval(retryInterval int) *SubTask
	GetOutput() string
	UnmarshalOutput(v interface{}) error
	UnmarshalInput(v interface{}) error
	SetAttribute(key string, v interface{})
	GetAttribute(key string) interface{}
	IsFinished() bool
}

var taskHash = func(c *SubTask) string {
	return c.GetTaskId()
}

type Task struct {
	taskName      string
	input         string
	description   string
	taskId        string
	taskState     TaskState
	requestId     string
	retry         int
	retryInterval int
	urgent        bool
	failedSubtask bool
	subTasks      []*SubTask
	subtaskMap    map[string]*SubTask
	g             graph.Graph[string, *SubTask]
	copyg         graph.Graph[string, *SubTask]
	rollbacks     []*SubTask
}

type SubTask struct {
	taskName      string
	input         string
	output        string
	taskId        string
	retry         int
	rollback      string
	retryInterval int
	taskState     TaskState
	attribute     map[string]interface{}
}

func NewTask(taskName string) *Task {
	return &Task{
		taskId:        tools.GenerateId("task"),
		taskName:      taskName,
		taskState:     TaskPending,
		subtaskMap:    make(map[string]*SubTask),
		retry:         DefaultRetryCount,
		g:             graph.New(taskHash, graph.Directed(), graph.PreventCycles()),
		retryInterval: DefaultRetryTimout,
	}
}

func NewSubTask(taskName string) *SubTask {
	return &SubTask{
		taskName:      taskName,
		taskState:     TaskPending,
		retry:         DefaultRetryCount,
		retryInterval: DefaultRetryTimout,
		rollback:      string(RollbackPending),
	}
}

func (t *SubTask) AllocateTaskId() {
	if t.taskId == "" {
		t.taskId = tools.GenerateId("subtask")
	}
}

func (t *SubTask) GetTaskId() string {
	return t.taskId
}

func (t *SubTask) GetTaskState() TaskState {
	return t.taskState
}

func (t *SubTask) SetTaskState(state TaskState) {
	t.taskState = state
}

func (t *SubTask) GetTaskName() string {
	return t.taskName
}

func (t *SubTask) UnmarshalInput(v interface{}) error {
	return tools.DeByte([]byte(t.input), v)
}

func (t *SubTask) GetInput() string {
	return t.input
}

func (t *SubTask) SetInput(content interface{}) *SubTask {
	_tmp, _ := tools.ToByte(content)
	t.input = string(_tmp)
	return t
}

func (t *SubTask) SetOutput(output interface{}) *SubTask {
	_tmp, _ := tools.ToByte(output)
	t.output = string(_tmp)
	return t
}

func (t *SubTask) GetOutput() string {
	return t.output
}

func (t *SubTask) UnmarshalOutput(v interface{}) error {
	return tools.DeByte([]byte(t.output), v)
}

func (t *SubTask) SetAttribute(key string, v interface{}) {
	t.attribute[key] = v
}

func (t *SubTask) GetAttribute(key string) interface{} {
	return t.attribute[key]
}

func (t *SubTask) IsFinished() bool {
	return t.taskState == TaskFailed || t.taskState == TaskSucceeded
}

func (t *SubTask) SetRetry(retry int) *SubTask {
	t.retry = retry
	return t
}

func (t *SubTask) SetRetryInterval(retryTimeout int) *SubTask {
	t.retryInterval = retryTimeout
	return t
}

func (t *SubTask) needRollback() bool {
	return (t.taskState == TaskRunning || t.taskState == TaskSucceeded) && (t.rollback == string(RollbackPending) || t.rollback == string(RollingBack))
}

func (t *Task) GetTaskId() string {
	return t.taskId
}

func (t *Task) GetTaskName() string {
	return t.taskName
}

func (t *Task) AllocateTaskId() {
	if t.taskId == "" {
		t.taskId = tools.GenerateId("subtask")
	}
}

func (t *Task) GetInput() string {
	return t.input
}

func (t *Task) SetInput(content interface{}) *Task {
	_tmp, _ := tools.ToByte(content)
	t.input = string(_tmp)
	return t
}

func (t *Task) SetTaskState(state TaskState) {
	t.taskState = state
}

func (t *Task) GetTaskState() TaskState {
	return t.taskState
}

func (t *Task) SetDescription(description string) *Task {
	t.description = description
	return t
}

func (t *Task) GetDescription() string {
	return t.description
}

func (t *Task) UnmarshalInput(v interface{}) error {
	return tools.DeByte([]byte(t.input), v)
}

func (t *Task) AddSubTask(task *SubTask) error {
	task.AllocateTaskId()
	t.subTasks = append(t.subTasks, task)
	t.subtaskMap[task.GetTaskId()] = task
	return t.g.AddVertex(task)
}

func (t *Task) AddDirectedEdge(src, dst *SubTask) error {
	//t.isSort = false
	return t.g.AddEdge(src.GetTaskId(), dst.GetTaskId())
}

func (t *Task) SetRequestId(requestId string) *Task {
	t.requestId = requestId
	return t
}

func (t *Task) SetRetry(retry int) *Task {
	t.retry = retry
	return t
}

func (t *Task) SetRetryInterval(retryInterval int) *Task {
	t.retryInterval = retryInterval
	return t
}

// SetUrgent handle task immediately
func (t *Task) SetUrgent() *Task {
	t.urgent = true
	return t
}

func (t *Task) NextSubTasks() ([]*SubTask, bool) {
	var res []*SubTask
	// has failedSubtask
	if t.failedSubtask {
		return t.rollbacks, true
	}

	predecessorMap, _ := t.g.PredecessorMap()
	for k, v := range predecessorMap {
		if len(v) == 0 {
			res = append(res, t.subtaskMap[k])
		}
	}

	return res, false
}

func (t *Task) UpdateSubtaskState(taskId string, taskState TaskState) error {
	subtask := t.subtaskMap[taskId]
	if subtask == nil {
		return fmt.Errorf("%s Task not found", taskId)
	}

	if taskState == TaskSucceeded {
		adjacencyMap, err := t.g.AdjacencyMap()
		if err != nil {
			return err
		}
		for _, v := range adjacencyMap[subtask.GetTaskId()] {
			if err = t.g.RemoveEdge(v.Source, v.Target); err != nil {
				return err
			}
		}

		predecessorMap, err := t.g.PredecessorMap()
		if err != nil {
			return err
		}
		for _, v := range predecessorMap[subtask.GetTaskId()] {
			if err = t.g.RemoveEdge(v.Source, v.Target); err != nil {
				return err
			}
		}

		if err = t.g.RemoveVertex(subtask.GetTaskId()); err != nil {
			return err
		}
	} else if taskState == TaskFailed {
		t.failedSubtask = true
	}
	subtask.SetTaskState(taskState)

	return nil
}

func (t *Task) UpdateTaskState(state TaskState) {
	t.taskState = state
}

func (t *Task) IsFinished() bool {
	return t.taskState == TaskFailed || t.taskState == TaskSucceeded
}

func (t *Task) Size() int {
	size, _ := t.g.Size()
	return size
}

func (t *Task) Order() int {
	order, _ := t.g.Order()
	return order
}

func (t *Task) Graph() string {
	var buf bytes.Buffer
	adjacencyMap, _ := t.g.AdjacencyMap()
	sorts, _ := graph.TopologicalSort(t.g)
	for _, v := range sorts {
		buf.WriteString(fmt.Sprintf("[%s]", t.subtaskMap[v].GetTaskName()))
		var edgeStr string
		for _, edge := range adjacencyMap[v] {
			if edgeStr == "" {
				edgeStr += " => ["
			} else {
				edgeStr += ","
			}
			edgeStr += t.subtaskMap[edge.Target].GetTaskName()
		}
		if edgeStr != "" {
			edgeStr += "]"
		} else {
			edgeStr = " => []"
		}

		buf.WriteString(edgeStr + "\n")
	}
	return buf.String()
}

func (t *Task) convert2Bean() (*taskxdao.Task, []*taskxdao.Subtask) {
	now := time.Now()
	task := &taskxdao.Task{
		TaskId:        t.taskId,
		TaskName:      t.taskName,
		RequestId:     t.requestId,
		Input:         t.input,
		Retry:         t.retry,
		RetryInterval: t.retryInterval,
		Urgent:        t.urgent,
		TaskState:     string(t.taskState),
		Description:   t.description,
		CreateTime:    basic.Time(now),
		UpdateTime:    basic.Time(now),
		Status:        1,
	}

	predecessorMap, _ := t.g.PredecessorMap()
	subtasks := make([]*taskxdao.Subtask, 0, len(t.subTasks))
	for _, v := range t.subTasks {
		m := predecessorMap[v.GetTaskId()]
		preSubtaskId := ""
		for k, _ := range m {
			if preSubtaskId != "" {
				preSubtaskId += ","
			}
			preSubtaskId += k
		}
		subtasks = append(subtasks, &taskxdao.Subtask{
			TaskId:        t.GetTaskId(),
			SubtaskId:     v.GetTaskId(),
			TaskName:      v.taskName,
			Input:         v.input,
			Retry:         v.retry,
			RetryInterval: v.retryInterval,
			Rollback:      v.rollback,
			TaskState:     string(v.taskState),
			UpdateTime:    basic.Time(now),
			PreSubtaskId:  preSubtaskId,
			Status:        1,
		})
	}

	return task, subtasks
}

func (t *Task) initByBean(task *taskxdao.Task, subtasks []*taskxdao.Subtask) (*Task, error) {
	t.taskId = task.TaskId
	t.taskName = task.TaskName
	t.requestId = task.RequestId
	t.taskState = TaskState(task.TaskState)
	t.description = task.Description
	t.input = task.Input
	t.retry = task.Retry
	t.subtaskMap = make(map[string]*SubTask)
	t.urgent = task.Urgent
	t.retryInterval = task.RetryInterval
	t.g = graph.New(taskHash, graph.Directed(), graph.PreventCycles())
	for _, subtask := range subtasks {
		st := &SubTask{
			taskId:    subtask.SubtaskId,
			taskName:  subtask.TaskName,
			input:     subtask.Input,
			output:    subtask.Output,
			rollback:  subtask.Rollback,
			taskState: TaskState(subtask.TaskState),
		}
		err := t.AddSubTask(st)
		if err != nil {
			return nil, err
		}
	}

	for _, subtask := range subtasks {
		if subtask.PreSubtaskId == "" {
			continue
		}
		for _, v := range strings.Split(subtask.PreSubtaskId, ",") {
			subTask := t.subtaskMap[v]
			subTask2 := t.subtaskMap[subtask.SubtaskId]
			err := t.AddDirectedEdge(subTask, subTask2)
			if err != nil {
				return nil, err
			}
		}
		if subtask.TaskState == string(TaskFailed) {
			t.failedSubtask = true
		}
	}

	if t.failedSubtask {
		if err := t.rollbackSubtasks(); err != nil {
			return nil, err
		}
	} else {
		for _, subtask := range subtasks {
			if err := t.UpdateSubtaskState(subtask.SubtaskId, TaskState(subtask.TaskState)); err != nil {
				return nil, err
			}
		}
	}

	return t, nil
}

func (t *Task) rollbackSubtasks() error {
	var res [][]*SubTask
	predecessorMap, err := t.g.PredecessorMap()
	if err != nil {
		return err
	}

	for len(predecessorMap) != 0 {
		var tmp, removeTmp []*SubTask

		for k, v := range predecessorMap {
			if len(v) == 0 {
				if t.subtaskMap[k].needRollback() {
					tmp = append(tmp, t.subtaskMap[k])
				}

				removeTmp = append(removeTmp, t.subtaskMap[k])
			}
		}

		adjacencyMap, err1 := t.g.AdjacencyMap()
		if err1 != nil {
			return err1
		}
		for _, v := range removeTmp {
			for _, v1 := range adjacencyMap[v.taskId] {
				if err = t.g.RemoveEdge(v1.Source, v1.Target); err != nil {
					return err
				}
			}

			if err = t.g.RemoveVertex(v.taskId); err != nil {
				return err
			}
		}

		if len(tmp) > 0 {
			res = append(res, tmp)
		}

		predecessorMap, err = t.g.PredecessorMap()
		if err != nil {
			return err
		}
	}

	// rollback subtask
	if len(res) > 0 {
		t.rollbacks = res[len(res)-1]
	}

	return nil
}
