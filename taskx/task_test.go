package taskx

import (
	"fmt"
	"testing"
)

func TestTask(t *testing.T) {
	myTask := NewTask("testTask").SetInput("test")
	stp1 := NewSubTask("stp1").SetInput("stp1")
	stp2 := NewSubTask("stp2").SetInput("stp2")
	stp3 := NewSubTask("stp3").SetInput("stp3")
	stp4 := NewSubTask("stp4").SetInput("stp4")
	stp5 := NewSubTask("stp5").SetInput("stp5")
	myTask.AddSubTask(stp1)
	myTask.AddSubTask(stp2)
	myTask.AddSubTask(stp3)
	myTask.AddSubTask(stp4)
	myTask.AddSubTask(stp5)

	if err := myTask.AddDirectedEdge(stp1, stp2); err != nil {
		panic(err)
	}
	if err := myTask.AddDirectedEdge(stp1, stp3); err != nil {
		panic(err)
	}
	if err := myTask.AddDirectedEdge(stp3, stp4); err != nil {
		panic(err)
	}
	if err := myTask.AddDirectedEdge(stp5, stp4); err != nil {
		panic(err)
	}
	if err := myTask.AddDirectedEdge(stp2, stp5); err != nil {
		panic(err)
	}

	fmt.Printf("size = %v \n", myTask.Size())
	fmt.Printf("order = %v \n", myTask.Order())
	fmt.Println(myTask.Graph())

	fmt.Println("begin")
	doStream(myTask)
	fmt.Println("finish")
}

func doStream(myTask *Task) {
	tasks := myTask.NextSubTasks()
	if len(tasks) == 0 {
		return
	}

	for _, v := range tasks {
		// do something
		v.SetTaskState(TaskRunning)
		fmt.Printf("task = %+v\n", v)

		// finish
		v.GetTaskState()
		myTask.UpdateSubTaskState(v.GetTaskId(), TaskSucceeded)
	}

	fmt.Printf("size = %v \n", myTask.Size())
	fmt.Printf("order = %v \n", myTask.Order())
	fmt.Println(myTask.Graph())
	doStream(myTask)
}
