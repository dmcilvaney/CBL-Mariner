// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package main

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
)

type FibonacciTask struct {
	task.DefaultValueTask[int]
	// FibonacciTask specific fields
	n      int
	n1Task task.ValueTask[int]
	n2Task task.ValueTask[int]
}

func (f *FibonacciTask) ID() string {
	return fmt.Sprintf("%d", f.n)
}

func (f *FibonacciTask) Name() string {
	return fmt.Sprintf("Fibonacci(%d)", f.n)
}

// Contrived example to demonstrate the scheduler. Each iteration will queue to a dependency task
// and wait for it to complete before executing the next iteration.
func (t *FibonacciTask) Execute() {
	switch t.n {
	case 0:
		t.SetValue(0)
		t.SetDone()
		return
	case 1:
		t.SetValue(1)
		t.SetDone()
		return
	default:
		if t.n1Task == nil || t.n2Task == nil {
			n1Task := t.AddDependency(&FibonacciTask{
				n: t.n - 1,
			}, task.NoSelfCycle)
			n2Task := t.AddDependency(&FibonacciTask{
				n: t.n - 2,
			}, task.NoSelfCycle)
			var ok bool
			t.n1Task, ok = n1Task.(task.ValueTask[int])
			if !ok {
				panic("failed to cast n1Task to ValueTask[int]")
			}
			t.n2Task, ok = n2Task.(task.ValueTask[int])
			if !ok {
				panic("failed to cast n2Task to ValueTask[int]")
			}
		}
		n1 := t.n1Task.Value()
		n2 := t.n2Task.Value()

		t.SetValue(n1 + n2)
		t.SetDone()
		return
	}
}
