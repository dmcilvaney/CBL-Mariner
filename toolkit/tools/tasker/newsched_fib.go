// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package main

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
)

func main() {
	s := task.NewScheduler(false)
	f := &FibonacciTask{
		n: 7,
	}
	s.AddTask(nil, f, task.NoSelfCycle)

	result := f.Value()
	fmt.Println(result)
}
