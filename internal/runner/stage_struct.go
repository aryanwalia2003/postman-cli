package runner

import "time"

// Stage describes one segment of a load ramp.
// TargetWorkers is the number of concurrent goroutines to run during this stage.
// A TargetWorkers of 0 on the final stage is a clean ramp-down to zero.
type Stage struct {
	Duration      time.Duration
	TargetWorkers int
}