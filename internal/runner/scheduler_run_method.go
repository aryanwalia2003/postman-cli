package runner

import (
	"context"
	"time"
)

// Run executes the load test and returns all collected results.
// It blocks until the test is complete (duration elapsed or all stages done).
func (s *Scheduler) Run() []WorkerResult {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()

	// Apply wall-clock deadline if Duration is set.
	if s.cfg.Duration > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.cfg.Duration)
		s.cancel = cancel
		defer cancel()
	}

	// Launch the appropriate job injector in a goroutine.
	go s.injectJobs(ctx)

	// Collect results until the results channel is closed.
	all := make([]WorkerResult, 0, 512)
	for r := range s.results {
		all = append(all, r)
	}
	return all
}

// totalStageDuration sums all stage durations for display purposes.
func (s *Scheduler) totalStageDuration() time.Duration {
	var total time.Duration
	for _, st := range s.cfg.Stages {
		total += st.Duration
	}
	return total
}