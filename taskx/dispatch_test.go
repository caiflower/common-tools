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
	taskName = "taskDemo"

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

func (t *TaskDemo) StepOne(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step one")
	return false, data.Input, err
}

func (t *TaskDemo) StepTwo(data *TaskData) (retry bool, output interface{}, err error) {
	logger.Info("step two")
	return false, "", errors.New("StepTwo failed")
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

func TestDisPatch(t *testing.T) {
	config := dbv1.Config{
		Url:          "proxysql.app.svc.cluster.local:6033",
		User:         "root",
		Password:     "xxx",
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
	subtaskDao := &taskxdao.SubTaskDao{
		IDB: client,
	}

	cluster1, cluster2, cluster3 := commonCluster()
	receiver1 := &taskReceiver{
		Cluster:          cluster1,
		TaskDao:          taskDao,
		SubTaskDao:       subtaskDao,
		subtaskInflight:  inflight.NewInFlight(),
		taskInflight:     inflight.NewInFlight(),
		subtaskWorker:    50,
		taskWorker:       5,
		taskQueueSize:    1000,
		subtaskQueueSize: 1000,
	}
	receiver2 := &taskReceiver{
		Cluster:          cluster2,
		TaskDao:          taskDao,
		SubTaskDao:       subtaskDao,
		subtaskInflight:  inflight.NewInFlight(),
		taskInflight:     inflight.NewInFlight(),
		subtaskWorker:    50,
		taskWorker:       5,
		taskQueueSize:    1000,
		subtaskQueueSize: 1000,
	}
	receiver3 := &taskReceiver{
		Cluster:          cluster3,
		TaskDao:          taskDao,
		SubTaskDao:       subtaskDao,
		subtaskInflight:  inflight.NewInFlight(),
		taskInflight:     inflight.NewInFlight(),
		subtaskWorker:    50,
		taskWorker:       5,
		taskQueueSize:    1000,
		subtaskQueueSize: 1000,
	}
	dispatcher1 := &taskDispatcher{
		Cluster:    cluster1,
		TaskDao:    taskDao,
		SubTaskDao: subtaskDao,
	}
	dispatcher2 := &taskDispatcher{
		Cluster:    cluster2,
		TaskDao:    taskDao,
		SubTaskDao: subtaskDao,
	}
	dispatcher3 := &taskDispatcher{
		Cluster:    cluster3,
		TaskDao:    taskDao,
		SubTaskDao: subtaskDao,
	}
	receiver1.TaskDispatcher = dispatcher1
	receiver2.TaskDispatcher = dispatcher2
	receiver3.TaskDispatcher = dispatcher3

	// 提交一个任务
	submitTask(dispatcher1)
	submitTask(dispatcher1)

	// begin consume
	receiver1.Start()
	receiver2.Start()
	receiver3.Start()
	//receiver1.Close()
	//receiver2.Close()
	//receiver3.Close()

	demo := &TaskDemo{}
	RegisterTaskExecutor(demo.GetExecutor())

	tracker1 := cluster.NewDefaultJobTracker(5, cluster1, dispatcher1)
	tracker2 := cluster.NewDefaultJobTracker(5, cluster2, dispatcher2)
	tracker3 := cluster.NewDefaultJobTracker(5, cluster3, dispatcher3)
	tracker1.Start()
	tracker2.Start()
	tracker3.Start()

	go cluster1.StartUp()
	go cluster2.StartUp()
	go cluster3.StartUp()

	global.DefaultResourceManger.Signal()
}

func submitTask(dispatcher1 *taskDispatcher) {
	task := NewTask(taskName).SetRequestId("testTraceId").SetDescription("test").SetUrgent()
	one := NewSubTask(stepOne).SetInput("one")
	two := NewSubTask(stepTwo).SetInput("two")
	three := NewSubTask(stepThree).SetInput("three")
	four := NewSubTask(stepFour).SetInput("four")
	five := NewSubTask(stepFive).SetInput("five")
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
