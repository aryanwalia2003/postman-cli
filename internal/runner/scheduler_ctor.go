package runner

// NewScheduler constructs a Scheduler from the given config.
// Call Run() to start the load test.
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	return &Scheduler{
		cfg:     cfg,
		results: make(chan WorkerResult, 2048),
		jobs:    make(chan WorkerJob),
	}
}