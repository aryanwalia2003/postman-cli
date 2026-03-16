package runner

import (
	"context"
	"sync"
	"sync/atomic"
)

// Scheduler orchestrates duration, RPS, and stage-based load tests.
// It is the Phase 3 replacement for WorkerPool when dynamic control is needed.
type Scheduler struct {
	cfg SchedulerConfig

	// Live state — read by the progress renderer.
	activeWorkers atomic.Int64
	completedJobs atomic.Int64
	failedJobs    atomic.Int64

	results chan WorkerResult
	jobs    chan WorkerJob
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}