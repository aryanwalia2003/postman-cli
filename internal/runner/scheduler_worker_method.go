package runner

import (
	"context"
	"sync"
	"time"

	"reqx/internal/http_executor"
	"reqx/internal/scripting"
)


func (s *Scheduler) spawnWorker(ctx context.Context, id int) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Per-worker isolated state (persist across iterations).
		rtCtx := NewRuntimeContext()
		rtCtx.PersistConnections = true
		if s.cfg.BaseEnv != nil {
			rtCtx.SetEnvironment(s.cfg.BaseEnv.Clone())
		}
		if len(s.cfg.Personas) > 0 {
			p := s.cfg.Personas[(id-1)%len(s.cfg.Personas)]
			applyPersona(rtCtx, p)
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
		engine.ApplyRuntimeContext(rtCtx)

		// Worker owns the AsyncStop lifecycle — close it only when the goroutine exits.
		defer func() {
			rtCtx.AsyncStopOnce.Do(func() { close(rtCtx.AsyncStop) })
			rtCtx.AsyncWG.Wait()
		}()

		for {
			// ── Idle gate ──────────────────────────────────────────────────
			if int64(id) > s.desiredWorkers.Load() {
				// Quiesce background socket goroutines so they stop blocking
				// OS threads while this worker is inactive.
				rtCtx.PauseConnections()
				s.activeWorkers.Store(s.desiredWorkers.Load())

				for {
					if ctx.Err() != nil {
						return
					}
					if int64(id) <= s.desiredWorkers.Load() {
						break
					}
					select {
					case <-ctx.Done():
						return
					case <-s.wake():
					case <-time.After(100 * time.Millisecond):
					}
				}

				// Worker is active again — resume socket goroutines and
				// re-sync the executor with the new channel reference so it
				// picks up the freshly-closed active channel.
				rtCtx.ResumeConnections()
				engine.ApplyRuntimeContext(rtCtx)
			}

			s.activeWorkers.Store(s.desiredWorkers.Load())

			// Reset the AsyncStop channel between iterations so each DAG run
			// gets a fresh channel. Sockets from the previous iteration are
			// still alive listening on the OLD channel; they are unaffected.
			rtCtx.AsyncStop = make(chan struct{})
			rtCtx.AsyncStopOnce = new(sync.Once)
			rtCtx.AsyncWG = new(sync.WaitGroup)

			iter := int(s.completedIterations.Add(1))
			metrics, err := engine.Run(s.cfg.Plan, rtCtx)
			if err != nil {
				s.failedIterations.Add(1)
			}
			for i := range metrics {
				metrics[i].WorkerID = id
			}
			s.results <- WorkerResult{IterationIndex: iter, Metrics: metrics, Err: err}

			// Optional RPS control — per-worker pacing without a central job queue.
			if s.cfg.RPS > 0 {
				desired := s.desiredWorkers.Load()
				if desired < 1 {
					desired = 1
				}
				interval := time.Duration(float64(time.Second) * float64(desired) / s.cfg.RPS)
				if interval > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(interval):
					}
				}
			}
		}
	}()
}

func (s *Scheduler) wake() <-chan struct{} {
	s.wakeMu.Lock()
	ch := s.wakeCh
	s.wakeMu.Unlock()
	return ch
}