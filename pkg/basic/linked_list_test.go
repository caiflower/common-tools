package basic

import (
	"fmt"
	"testing"
)

type testLinkedList struct {
	Name string
}

func TestLinkedList(t *testing.T) {
	l := LinkedList[int]{}
	l.AddFirst(2)
	l.AddFirst(1)
	l.AddLast(3)
	l.AddLast(4)
	fmt.Println(l.Contains(2) == true)

	for i := 1; i <= 4; i++ {
		first, err := l.RemoveFirst()
		fmt.Println(err == nil && first == i)
	}

	l.AddFirst(2)
	l.AddFirst(1)
	l.AddLast(3)
	l.AddLast(4)

	for i := 4; i >= 1; i-- {
		last, err := l.RemoveLast()
		fmt.Println(err == nil && last == i)
	}

	fmt.Println(l.Contains(2) == false)

	l2 := LinkedList[testLinkedList]{}
	o2 := testLinkedList{Name: "1"}
	l2.AddFirst(testLinkedList{Name: "1"})
	fmt.Println(l2.Contains(o2) == true)

	l3 := LinkedList[*testLinkedList]{}
	o3 := &testLinkedList{Name: "1"}
	l3.AddFirst(&testLinkedList{Name: "1"})
	fmt.Println(l3.Contains(o3) == false)
}
