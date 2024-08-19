package docker_test

import (
	"path"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/docker"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Run the tests
	logger.InitStderrLog()
	m.Run()
}

func TestRun(t *testing.T) {
	// Define test cases
	tests := []struct {
		name           string
		command        string
		args           []string
		mountPoint     docker.DockerMount
		expectedStdout string
		expectedStderr string
		expectedError  error
	}{
		{
			name:    "Test with valid mount points",
			command: "cat",
			args:    []string{"/container/path/test.txt"},
			mountPoint: docker.DockerMount{
				Source: "./testdata/",
				Dest:   "/container/path/",
			},
			expectedStdout: "test\n",
			expectedStderr: "",
			expectedError:  nil,
		},
		{
			name:           "Test with empty mount points",
			command:        "echo",
			args:           []string{"Hello, World!"},
			mountPoint:     docker.DockerMount{},
			expectedStdout: "Hello, World!\n",
			expectedStderr: "",
			expectedError:  nil,
		},
		{
			name:           "Test stderr",
			command:        "sh",
			args:           []string{"-c", "echo error 1>&2"},
			mountPoint:     docker.DockerMount{},
			expectedStdout: "",
			expectedStderr: "error\n",
			expectedError:  nil,
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the Run function
			stdout, stderr, err := docker.Run(tt.command, tt.args, &tt.mountPoint, nil, nil, docker.SrpmImageTag, docker.CreateReposAndRun, "", false)

			// Check if the error matches the expected error
			assert.Equal(t, tt.expectedError, err)

			// Check if the output matches the expected output
			assert.Equal(t, tt.expectedStdout, stdout)

			// Check if the stderr matches the expected stderr
			assert.Equal(t, tt.expectedStderr, stderr)
		})
	}
}

func TestWithOverlay(t *testing.T) {
	tempTestDir := t.TempDir()
	tests := []struct {
		name           string
		command        string
		args           []string
		mountPoint     docker.DockerMount
		overlayMounts  []docker.DockerOverlay
		expectedStdout string
		expectedError  error
	}{
		{
			name:    "Ensure overlay host is RO",
			command: "bash",
			args:    []string{"-c", "cp /container/path/test.txt /container/overlay/test.txt && cat /container/overlay/test.txt"},
			mountPoint: docker.DockerMount{
				Source: "./testdata/",
				Dest:   "/container/path/",
			},
			overlayMounts: []docker.DockerOverlay{
				{
					Source:   tempTestDir,
					Dest:     "/container/overlay",
					Priority: 1,
				},
			},
			expectedError:  nil,
			expectedStdout: "test\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := docker.Run(tt.command, tt.args, &tt.mountPoint, tt.overlayMounts, nil, docker.SrpmImageTag, docker.CreateReposAndRun, "", false)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedStdout, stdout)

			// Ensure no files are written back to the host
			exists, err := file.PathExists(path.Join(tempTestDir, "test.txt"))
			assert.False(t, exists)
			assert.NoError(t, err)
		})
	}
}
