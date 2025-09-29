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
	"fmt"
	"testing"
)

func TestTask(t *testing.T) {
	myTask := NewTask("testTask").SetInput("test")
	stp1 := NewSubtask("stp1").SetInput("stp1")
	stp2 := NewSubtask("stp2").SetInput("stp2")
	stp3 := NewSubtask("stp3").SetInput("stp3")
	stp4 := NewSubtask("stp4").SetInput("stp4")
	stp5 := NewSubtask("stp5").SetInput("stp5")
	_ = myTask.AddSubTask(stp1)
	_ = myTask.AddSubTask(stp2)
	_ = myTask.AddSubTask(stp3)
	_ = myTask.AddSubTask(stp4)
	_ = myTask.AddSubTask(stp5)

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
	tasks, _ := myTask.NextSubTasks()
	if len(tasks) == 0 {
		return
	}

	for _, v := range tasks {
		// do something
		v.SetTaskState(TaskRunning)
		fmt.Printf("task = %+v\n", v)

		// finish
		v.GetTaskState()
		_ = myTask.UpdateSubtaskState(v.GetTaskId(), TaskSucceeded)
	}

	fmt.Printf("size = %v \n", myTask.Size())
	fmt.Printf("order = %v \n", myTask.Order())
	fmt.Println(myTask.Graph())
	doStream(myTask)
}
