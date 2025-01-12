package taskx

import (
	"bytes"
	"fmt"

	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/dominikbraun/graph"
)

type TaskState string

const (
	TaskPending   TaskState = "Pending"
	TaskRunning   TaskState = "Running"
	TaskFailed    TaskState = "Failed"
	TaskSucceeded TaskState = "Succeeded"
)

type ITask interface {
	GetTaskId() string
	GetTaskName() string
	GetTaskState() TaskState
	SetTaskState(state TaskState)
	AllocateTaskId()
	GetContent() string
	UnmarshalContent(v interface{}) error

	AddSubTask(Task *SubTask)                                // add SubTask
	AddDirectedEdge(src, dst *SubTask) error                 // add directedEdge
	NextPendingSubTasks() []*SubTask                         // get next pending subTasks
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
	GetTaskType() string
	AllocateTaskId()
	GetContent() string
	UnmarshalContent(v interface{}) error
}

var taskHash = func(c *SubTask) string {
	return c.GetTaskId()
}

type Task struct {
	taskName string
	content  string

	taskId     string
	taskState  TaskState
	subTasks   []*SubTask
	subTaskMap map[string]*SubTask
	isSort     bool
	g          graph.Graph[string, *SubTask]
}

type SubTask struct {
	taskType string
	content  string

	taskId    string
	taskState TaskState
}

func NewTask(taskName string, content interface{}) *Task {
	return &Task{
		taskName:   taskName,
		taskState:  TaskPending,
		content:    tools.ToJson(content),
		subTaskMap: make(map[string]*SubTask),
		g:          graph.New(taskHash, graph.Directed(), graph.PreventCycles()),
	}
}

func NewSubTask(taskType string, content interface{}) *SubTask {
	return &SubTask{
		taskType:  taskType,
		taskState: TaskPending,
		content:   tools.ToJson(content),
	}
}

func (t *SubTask) AllocateTaskId() {
	if t.taskId == "" {
		t.taskId = tools.GenerateId("Task")
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

func (t *SubTask) GetTaskType() string {
	return t.taskType
}

func (t *SubTask) UnmarshalContent(v interface{}) error {
	return tools.Unmarshal([]byte(t.content), v)
}

func (t *SubTask) GetContent() string {
	return t.content
}

func (t *Task) GetTaskId() string {
	return t.taskId
}

func (t *Task) GetTaskName() string {
	return t.taskName
}

func (t *Task) AllocateTaskId() {
	if t.taskId == "" {
		t.taskId = tools.GenerateId("Task")
	}
}

func (t *Task) GetContent() string {
	return t.content
}

func (t *Task) SetTaskState(state TaskState) {
	t.taskState = state
}

func (t *Task) GetTaskState() TaskState {
	return t.taskState
}

func (t *Task) UnmarshalContent(v interface{}) error {
	return tools.Unmarshal([]byte(t.content), v)
}

func (t *Task) AddSubTask(Task *SubTask) {
	Task.AllocateTaskId()
	t.subTasks = append(t.subTasks, Task)
	t.subTaskMap[Task.GetTaskId()] = Task
	t.g.AddVertex(Task)
}

func (t *Task) AddDirectedEdge(src, dst *SubTask) error {
	t.isSort = false
	return t.g.AddEdge(src.GetTaskId(), dst.GetTaskId())
}

func (t *Task) NextPendingSubTasks() []*SubTask {
	if t.isSort == false {
		t.Sort()
	}

	var res []*SubTask
	predecessorMap, _ := t.g.PredecessorMap()
	for _, v := range t.subTasks {
		if v.GetTaskState() == TaskPending {
			if len(predecessorMap[v.GetTaskId()]) == 0 {
				res = append(res, v)
			} else {
				break
			}
		}
	}

	return res
}

func (t *Task) UpdateSubTaskState(taskId string, taskState TaskState) error {
	subtask := t.subTaskMap[taskId]
	if subtask == nil {
		return fmt.Errorf("%s Task not found", taskId)
	}

	if taskState == TaskFailed || taskState == TaskSucceeded {
		adjacencyMap, err := t.g.AdjacencyMap()
		if err != nil {
			return err
		}
		for _, v := range adjacencyMap[subtask.GetTaskId()] {
			if err = t.g.RemoveEdge(v.Source, v.Target); err != nil {
				return err
			}
		}

		if err = t.g.RemoveVertex(subtask.GetTaskId()); err != nil {
			return err
		} else {
			subtask.SetTaskState(taskState)
		}
	} else {
		subtask.SetTaskState(taskState)
	}

	return nil
}

func (t *Task) UpdateTaskState(state TaskState) {
	t.taskState = state
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
		buf.WriteString(fmt.Sprintf("[%s]", t.subTaskMap[v].GetTaskType()))
		var edgeStr string
		for _, edge := range adjacencyMap[v] {
			if edgeStr == "" {
				edgeStr += " => ["
			} else {
				edgeStr += ","
			}
			edgeStr += t.subTaskMap[edge.Target].GetTaskType()
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

func (t *Task) Sort() {
	sorts, _ := graph.TopologicalSort(t.g)
	for i, v := range sorts {
		subtask := t.subTaskMap[v]
		t.subTasks[i] = subtask
	}
	t.isSort = true
}
