// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"fmt"

	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
)

type RpmFileTask struct {
	task.DefaultValueTask[toolkit_types.RpmFile]
	RpmFile *toolkit_types.RpmFile

	rpmPathDB *map[string]toolkit_types.RpmFile
}

func (r *RpmFileTask) ID() string {
	return r.RpmFile.Path
}

func (r *RpmFileTask) Name() string {
	return fmt.Sprintf("RPM_PATH: %s", r.RpmFile.Path)
}

// Get the file path of the rpm file, then query the runtime dependencies of the rpm file, and then ensure we have the deps available.
func (r *RpmFileTask) Execute() error {
	// Decide if we can use the cache, or do we need to build?
	return nil
}
