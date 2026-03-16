package runner

import (
	"context"
	"time"
)

// runStages walks through each Stage, adjusting the active worker count
// by spawning or draining goroutines between stages.
func (s *Scheduler) runStages(ctx context.Context) {
	currentWorkers := 0
	workerID := 0

	for _, stage := range s.cfg.Stages {
		target := stage.TargetWorkers

		// Scale up: spawn missing workers.
		for currentWorkers < target {
			workerID++
			s.spawnWorker(ctx, workerID)
			currentWorkers++
		}

		// Scale down: reduce job pressure so excess workers idle out.
		// Workers self-terminate when ctx is cancelled or jobs closes,
		// so we simply update our count and let the timer shrink them.
		if currentWorkers > target {
			currentWorkers = target
		}

		// Hold this stage for its duration (or until ctx expires).
		stageTimer := time.NewTimer(stage.Duration)
		if s.cfg.RPS > 0 {
			s.rpsForDuration(ctx, stageTimer.C)
		} else {
			s.freeFireForDuration(ctx, stageTimer.C)
		}
		stageTimer.Stop()

		if ctx.Err() != nil {
			return
		}
	}
}

// freeFireForDuration feeds jobs until stageDone fires or ctx expires.
func (s *Scheduler) freeFireForDuration(ctx context.Context, stageDone <-chan time.Time) {
	seq := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-stageDone:
			return
		case s.jobs <- WorkerJob{IterationIndex: seq}:
			seq++
		}
	}
}

// rpsForDuration feeds at cfg.RPS until stageDone fires or ctx expires.
func (s *Scheduler) rpsForDuration(ctx context.Context, stageDone <-chan time.Time) {
	interval := time.Duration(float64(time.Second) / s.cfg.RPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	seq := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-stageDone:
			return
		case <-ticker.C:
			select {
			case s.jobs <- WorkerJob{IterationIndex: seq}:
				seq++
			default:
			}
		}
	}
}