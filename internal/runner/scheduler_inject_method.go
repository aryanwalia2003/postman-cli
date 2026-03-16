package runner

import (
	"context"
	"time"
)

// injectJobs is the controller goroutine: it feeds jobs into s.jobs and manages
// worker lifecycle. When done it drains workers and closes s.results.
func (s *Scheduler) injectJobs(ctx context.Context) {
	defer func() {
		close(s.jobs)      // signal workers: no more jobs
		s.wg.Wait()        // wait for all in-flight iterations
		close(s.results)   // signal Run() collector: done
	}()

	if len(s.cfg.Stages) > 0 {
		s.runStages(ctx)
	} else {
		s.runDuration(ctx)
	}
}

// runDuration runs a fixed number of workers for cfg.Duration, optionally
// rate-limited to cfg.RPS jobs per second.
func (s *Scheduler) runDuration(ctx context.Context) {
	// Spawn the fixed worker pool.
	for i := 0; i < s.cfg.MaxWorkers; i++ {
		s.spawnWorker(ctx, i+1)
	}

	if s.cfg.RPS > 0 {
		s.rpsLoop(ctx)
	} else {
		s.freeFireLoop(ctx)
	}
}

// freeFireLoop feeds jobs as fast as workers can consume them.
func (s *Scheduler) freeFireLoop(ctx context.Context) {
	seq := 0
	for {
		select {
		case <-ctx.Done():
			return
		case s.jobs <- WorkerJob{IterationIndex: seq}:
			seq++
		}
	}
}

// rpsLoop feeds exactly cfg.RPS jobs per second using a ticker.
func (s *Scheduler) rpsLoop(ctx context.Context) {
	interval := time.Duration(float64(time.Second) / s.cfg.RPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	seq := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case s.jobs <- WorkerJob{IterationIndex: seq}:
				seq++
			default:
				// Workers full; drop tick to avoid blocking.
			}
		}
	}
}