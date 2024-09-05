// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package task

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/sirupsen/logrus"
)

type Tasker interface {
	// SetInfo sets the task's ID, name, and dirty level
	SetInfo(id, name string, dirt int)
	// Unique identifier for the task
	ID() string
	// Human readable name for the task
	Name() string
	// DirtLevel
	DirtyLevel() int
	// Implement the task
	Execute()
	// IsDone returns true if the task has been executed
	IsDone() bool
	// Result returns the result of the task
	Result() error
	// Wait for the task's dependencies to be done
	WaitForDeps()

	// Claim resource limiter
	ClaimLimit(int64)
	// Release resource limiter
	ReleaseLimit()

	SetDepth(int)
	TLog(level logrus.Level, format string, args ...interface{})

	// AddDependency adds a dependency to the task. The task will be remapped by the scheduler
	// to an existing task if it has the same ID
	AddDependency(Tasker, bool) Tasker

	// Interacts with the scheduler to add a dependency
	// registerWithScheduler registers a function to add a dependency to the scheduler
	registerWithScheduler(func(parent, newTask Tasker, allowSelfCycle bool) Tasker)
	// dependencies returns the dependencies of the task
	dependencies() []Tasker
}

type BasicTask struct {
	basicTaskID       string
	basicTaskName     string
	dirtyLevel        int
	addDepToScheduler func(Tasker, Tasker, bool) Tasker
	deps              []Tasker
	doneSemaphore     chan struct{}
	depth             int
	taskLimiter       *TaskLock
}

func NewBasicTask(ctx context.Context, id, name string, depth, dirt int) *BasicTask {
	newTask := &BasicTask{
		depth: depth,
	}
	newTask.SetInfo(id, name, dirt)
	return newTask
}

func (b *BasicTask) SetInfo(id, name string, dirt int) {
	b.basicTaskID = id
	b.basicTaskName = fmt.Sprintf("|%d|%s", dirt, name)
	b.dirtyLevel = dirt
}

func (b *BasicTask) ID() string {
	return b.basicTaskID
}

func (b *BasicTask) Name() string {
	return b.basicTaskName
}

func (b *BasicTask) DirtyLevel() int {
	return b.dirtyLevel
}

func (b *BasicTask) Execute() {
	b.SetDone()
}

// ClaimLimit claims some ammount of the global resource limiter. If the weight is larger than the
// limit it will claim the entire limit. Once the limiter is claimed, it cannot be claimed again, and
// dependencies cannot be added.
func (b *BasicTask) ClaimLimit(weight int64) {
	if b.taskLimiter != nil {
		b.TLog(logrus.FatalLevel, "Task %s has already claimed a limiter", b.ID())
	}

	// Claim resource limiter
	b.taskLimiter = AcquireTaskLimiterInternal(b, weight)
}
func (b *BasicTask) ReleaseLimit() {
	if b.taskLimiter == nil {
		b.TLog(logrus.FatalLevel, "Task %s has not claimed a limiter", b.ID())
	}

	// Release resource limiter
	b.taskLimiter.Release()
	b.taskLimiter = nil
}

func (b *BasicTask) registerWithScheduler(f func(Tasker, Tasker, bool) Tasker) {
	b.doneSemaphore = make(chan struct{})
	b.addDepToScheduler = f
}

func (b *BasicTask) AddDependency(t Tasker, allowSelfCylce bool) Tasker {
	if b.taskLimiter != nil {
		b.TLog(logrus.FatalLevel, "Task is currently holding a limiter, cannot add dependency %s", t.ID())
	}

	b.TLog(logrus.InfoLevel, "Adding dependency:")
	b.TLog(logrus.InfoLevel, "-- %s  ", b.ID())
	b.TLog(logrus.InfoLevel, "---> %s", t.ID())
	t.SetDepth(b.depth + 1)
	t = b.addDepToScheduler(b, t, allowSelfCylce)
	if t == nil {
		b.TLog(logrus.InfoLevel, "---> CYCLE! MUST RESOLVE")
		return nil
	}
	b.deps = append(b.deps, t)
	return t
}

func (b *BasicTask) dependencies() []Tasker {
	return b.deps
}

func (b *BasicTask) IsDone() bool {
	select {
	case <-b.doneSemaphore:
		return true
	default:
		return false
	}
}

func (b *BasicTask) SetDone() {
	if b.taskLimiter != nil {
		b.TLog(logrus.FatalLevel, "Task %s has not released a limiter, can't mark done", b.ID())
	}
	close(b.doneSemaphore)
}

func (b *BasicTask) WaitForDeps() {
	for _, dep := range b.deps {
		dep.Result()
	}
}

func (b *BasicTask) ListDeps() []string {
	var deps []string
	for _, dep := range b.deps {
		deps = append(deps, dep.ID())
	}
	return deps
}

func (b *BasicTask) Result() error {
	<-b.doneSemaphore
	return nil
}

func (b *BasicTask) GetWorkDir() string {
	// Replace all non-alphanumeric characters with underscores
	sanitizedID := strings.Builder{}

	for _, c := range b.ID() {
		if ('a' <= c && c <= 'z') ||
			('A' <= c && c <= 'Z') ||
			('0' <= c && c <= '9') {
			sanitizedID.WriteRune(c)
		} else {
			sanitizedID.WriteRune('_')
		}
	}

	workDirBaseName := fmt.Sprintf("task-%s", sanitizedID.String())
	// Remove all path separators from the task ID to avoid creating nested directories. Replace with underscores.
	workDirBaseName = strings.ReplaceAll(workDirBaseName, string(os.PathSeparator), "_")
	workDir, err := os.MkdirTemp(buildconfig.CurrentBuildConfig.TempDir, workDirBaseName)
	if err != nil {
		b.TLog(logrus.FatalLevel, "Failed to create work dir: %v", err)
	}
	return workDir
}

func (b *BasicTask) SetDepth(depth int) {
	b.depth = depth
}

func (b *BasicTask) TLog(level logrus.Level, format string, args ...interface{}) {
	nameMaxLen := 60
	name := b.Name()
	if len(name) > nameMaxLen {
		name = name[:nameMaxLen]
	}
	// Pad the name with spaces so we have a consistent log format
	name = fmt.Sprintf("%-*s", nameMaxLen, name)
	indent := "  "
	for i := 0; i < b.depth; i++ {
		indent += "  "
	}
	if level == logrus.FatalLevel {
		logger.Log.Fatalf(name+indent+format, args...)
	} else {
		logger.Log.Logf(level, name+indent+format, args...)
	}
}

// Task which returns an integer value
type ValueTask[T any] interface {
	Tasker
	Value() T
}

type DefaultValueTask[T any] struct {
	BasicTask
	value T
}

func (i *DefaultValueTask[T]) Value() T {
	i.TLog(logrus.InfoLevel, "<- Providing value")
	<-i.doneSemaphore
	return i.value
}

func (i *DefaultValueTask[T]) SetValue(val T) {
	i.value = val
}
