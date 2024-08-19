// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Throttles the number of tasks that can be run concurrently globally

package task

import (
	"context"

	"golang.org/x/sync/semaphore"
)

var Limiter *limiter

type limiter struct {
	ctx                context.Context
	maxConcurrentTasks int64
	sem                *semaphore.Weighted
}

type TaskLock struct {
	weight int64
}

func InitializeLimiter(ctx context.Context, limit int64) {
	Limiter = &limiter{
		ctx: ctx,
		sem: semaphore.NewWeighted(int64(limit)),
	}
	Limiter.maxConcurrentTasks = 1
}

func AcquireTaskLimiter(weight int64) TaskLock {
	if Limiter == nil {
		return TaskLock{}
	}

	if weight > Limiter.maxConcurrentTasks {
		weight = Limiter.maxConcurrentTasks
	}
	Limiter.sem.Acquire(Limiter.ctx, weight)
	return TaskLock{weight: weight}
}

func (tl TaskLock) Release() {
	if Limiter == nil {
		return
	}

	Limiter.sem.Release(tl.weight)
}
