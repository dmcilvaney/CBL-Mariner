// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
)

type LoadSpecDataTask struct {
	task.DefaultValueTask[*toolkit_types.SpecDataDB]
}

func NewLoadSpecDataTask() *LoadSpecDataTask {
	newSpecDataTask := &LoadSpecDataTask{}

	newSpecDataTask.SetInfo(
		"SPEC_DATA_DB",
		"SPEC_DATA_DB",
		0,
	)

	//logger.Log.Infof("Creating new task: '%s'", newSpecDataTask.ID())
	return newSpecDataTask
}

func (s *LoadSpecDataTask) Execute() {
	s.TLog(logrus.InfoLevel, "Execute(): '%s'", s.ID())

	if toolkit_types.GlobalSpecDataDB == nil {
		logger.Log.Fatalf("GlobalSpecDataDB is nil")
	}

	s.SetValue(toolkit_types.GlobalSpecDataDB)
	s.SetDone()
}
