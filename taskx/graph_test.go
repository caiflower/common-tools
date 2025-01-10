package taskx

import (
	"fmt"
	"testing"

	"github.com/dominikbraun/graph"
)

func TestGraph(t *testing.T) {
	g := graph.New(taskHash, graph.Directed(), graph.PreventCycles())

	task1 := Task{TaskId: "task1"}
	task2 := Task{TaskId: "task2"}
	task3 := Task{TaskId: "task3"}
	task4 := Task{TaskId: "task4"}
	err := g.AddVertex(task1)
	if err != nil {
		panic(err)
	}
	err = g.AddVertex(task2)
	if err != nil {
		panic(err)
	}
	err = g.AddVertex(task3)
	if err != nil {
		panic(err)
	}
	err = g.AddVertex(task4)
	if err != nil {
		panic(err)
	}

	err = g.AddEdge(taskHash(task1), taskHash(task2))
	if err != nil {
		panic(err)
	}
	err = g.AddEdge(taskHash(task2), taskHash(task3))
	if err != nil {
		panic(err)
	}
	err = g.AddEdge(taskHash(task1), taskHash(task4))
	if err != nil {

	}

	printGraph(g)

	err = g.RemoveEdge(taskHash(task1), taskHash(task2))
	if err != nil {
		panic(err)
	}
	err = g.RemoveEdge(taskHash(task1), taskHash(task4))
	if err != nil {
		panic(err)
	}
	err = g.RemoveVertex(taskHash(task1))
	if err != nil {
		panic(err)
	}
	fmt.Println("--------remove task1----------")

	printGraph(g)
}

func printGraph(g graph.Graph[string, Task]) {
	order, err := g.Order()
	if err != nil {
		panic(err)
	}
	fmt.Printf("oder =  %v\n", order)

	predecessorMap, err := g.PredecessorMap()
	if err != nil {
		panic(err)
	}

	for k, v := range predecessorMap {
		fmt.Println(k, v)
	}
	fmt.Println("-------------")

	adjacencyMap, err := g.AdjacencyMap()
	if err != nil {
		return
	}
	for k, v := range adjacencyMap {
		fmt.Println(k, v)
	}
}
