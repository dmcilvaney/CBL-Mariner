// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"os"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/stretchr/testify/assert"
)

var defaultBuildConfig = buildconfig.BuildConfig{
	DistTag: ".test",
	RpmsDirsByDirtLevel: map[int]string{
		0: "./testdata/RPMS",
	},
	DoCheck: false,
}

func TestMain(m *testing.M) {
	logger.InitStderrLog()
	os.Exit(m.Run())
}

func TestNewSpecFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want *toolkit_types.SpecFile
	}{
		{
			name: "TestNewSpecFile",
			args: args{
				path: "./testdata/test.spec",
			},
			want: &toolkit_types.SpecFile{
				Path: "./testdata/test.spec",
				ProvidedRpms: []*toolkit_types.RpmFile{
					{
						Path: "testdata/RPMS/x86_64/test_pkg-1-1.test.x86_64.rpm",
					},
				},
				PredictedProvides: []*pkgjson.PackageVer{
					{
						Name:      "test_pkg",
						Version:   "1-1.test",
						Condition: "=",
					}, {
						Name:      "test_pkg(x86-64)",
						Version:   "1-1.test",
						Condition: "=",
					},
				},
				Sources: []string{
					"test.patch",
					"test.txt",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// if got := NewSpecFile(tt.args.path, defaultBuildConfig); !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("NewSpecFile() = %v, want %v", got, tt.want)
			// }
			assert.Equal(t, toolkit_types.NewSpecFile(tt.args.path, 0, defaultBuildConfig), tt.want)
		})
	}
}
