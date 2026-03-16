package runner

// SchedulerStats is a point-in-time snapshot of the scheduler's live counters.
// Used by the progress bar renderer without touching internal atomics directly.
type SchedulerStats struct {
	ActiveWorkers int64
	Completed     int64
	Failed        int64
}

// Stats returns a consistent snapshot of current load test progress.
func (s *Scheduler) Stats() SchedulerStats {
	return SchedulerStats{
		ActiveWorkers: s.activeWorkers.Load(),
		Completed:     s.completedJobs.Load(),
		Failed:        s.failedJobs.Load(),
	}
}