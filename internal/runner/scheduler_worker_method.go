package runner

import (
	"context"
	"reqx/internal/http_executor"
	"reqx/internal/scripting"
)

// spawnWorker launches one goroutine that consumes jobs until s.jobs closes or ctx expires.
func (s *Scheduler) spawnWorker(ctx context.Context, id int) {
	s.wg.Add(1)
	s.activeWorkers.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.activeWorkers.Add(-1)
		for {
			select {
			case <-ctx.Done():
				return
			case job, ok := <-s.jobs:
				if !ok {
					return
				}
				metrics, err := s.executeOne(ctx, id)
				s.completedJobs.Add(1)
				if err != nil {
					s.failedJobs.Add(1)
				}
				s.results <- WorkerResult{
					IterationIndex: job.IterationIndex,
					Metrics:        metrics,
					Err:            err,
				}
			}
		}
	}()
}

// executeOne runs a single collection pass in an isolated context.
func (s *Scheduler) executeOne(ctx context.Context, workerID int) ([]RequestMetric, error) {
	rtCtx := NewRuntimeContext()
	if s.cfg.BaseEnv != nil {
		rtCtx.SetEnvironment(s.cfg.BaseEnv.Clone())
	}

	exec := http_executor.NewDefaultExecutor()
	if s.cfg.NoCookies {
		exec.DisableCookies()
	}

	engine := NewCollectionRunner(exec, nil, nil, scripting.NewGojaRunner())
	engine.SetVerbosity(s.cfg.Verbosity)
	if s.cfg.ClearCookies {
		engine.SetClearCookiesPerRequest(true)
	}

	metrics, err := engine.Run(s.cfg.Coll, rtCtx)
	for i := range metrics {
		metrics[i].WorkerID = workerID
	}
	return metrics, err
}