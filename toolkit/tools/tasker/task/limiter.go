// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Throttles the number of tasks that can be run concurrently globally

package task

import (
	"context"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

var Limiter *limiter
var debugLimiter = true

type limiter struct {
	ctx                context.Context
	maxConcurrentTasks int64
	sem                *semaphore.Weighted
}

type TaskLock struct {
	t      Tasker
	weight int64
}

func InitializeLimiter(ctx context.Context, limit int64) {
	Limiter = &limiter{
		ctx: ctx,
		sem: semaphore.NewWeighted(int64(limit)),
	}
	Limiter.maxConcurrentTasks = limit
}

func AcquireTaskLimiterInternal(t Tasker, weight int64) *TaskLock {
	if Limiter == nil {
		return &TaskLock{}
	}

	gotLimiter := make(chan struct{})
	defer close(gotLimiter)
	if debugLimiter {
		go func() {
			t.TLog(logrus.InfoLevel, "Acquiring %d ammount of resources...", weight)
			startTime := time.Now()
			for {
				// Random value from -10 to 10
				randomExtraDelay := time.Duration((rand.Intn(20) - 10)) * time.Second

				select {
				case <-gotLimiter:
					t.TLog(logrus.InfoLevel, "Acquired %d ammount of resources!", weight)
					return
				case <-time.After(180*time.Second + randomExtraDelay):
					t.TLog(logrus.InfoLevel, "Still waiting for %d ammount of resources, time elapsed: %s", weight, time.Since(startTime).Round(time.Minute))
				}
			}
		}()
	}

	if weight > Limiter.maxConcurrentTasks {
		weight = Limiter.maxConcurrentTasks
	}
	Limiter.sem.Acquire(Limiter.ctx, weight)

	return &TaskLock{weight: weight, t: t}
}

func (tl TaskLock) Release() {
	if Limiter == nil {
		return
	}

	if debugLimiter {
		tl.t.TLog(logrus.InfoLevel, "Releasing %d ammount of resources", tl.weight)
	}

	Limiter.sem.Release(tl.weight)
}
