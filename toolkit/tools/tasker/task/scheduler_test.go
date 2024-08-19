// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package task

import (
	"os"
	"strconv"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Run the tests
	logger.InitStderrLog()
	m.Run()
}

func CreateTasks(s *Scheduler, name string, num int) {
	var prevTask Tasker = nil
	for i := 0; i < num; i++ {
		task := &BasicTask{}
		task.SetInfo(strconv.Itoa(i)+name, name, i)
		s.AddTask(prevTask, task)
		prevTask = task
	}
}

func TestWriteDOTGraph(t *testing.T) {
	scheduler := NewScheduler(true)
	CreateTasks(scheduler, "BUILDSPEC", 4)
	CreateTasks(scheduler, "BUILDSRPM", 4)
	CreateTasks(scheduler, "CACHE", 4)
	CreateTasks(scheduler, "CAP", 4)
	tempFile := "test.dot"
	fHandle, err := os.Create(tempFile)
	assert.NoError(t, err)
	defer fHandle.Close()
	err = scheduler.WriteDOTGraph(fHandle)
	assert.NoError(t, err)
}

// func TestScheduler_WriteDOTGraph(t *testing.T) {
// 	type fields struct {
// 	}
// 	tests := []struct {
// 		name       string
// 		fields     fields
// 	}{
// 		{
// 			name: "Test WriteDOTGraph",
// 			fields: fields{
// 				graphNodes: [][]string{

// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {

// 		})
// 	}
// }
