package runner

import (
	"reqx/internal/collection"
	"reqx/internal/environment"
	"time"
)

// SchedulerConfig holds all inputs the Scheduler needs to run a Phase 3 load test.
// Exactly one of Stages or Duration must be set; RPS is optional with either.
//
// Modes:
//   - Stages only:   ramp up/down workers, run until all stages complete.
//   - Duration only: fixed worker count (MaxWorkers) for the given wall time.
//   - Duration + RPS: inject jobs at exactly RPS/s for the given wall time.
type SchedulerConfig struct {
	Coll        *collection.Collection
	BaseEnv     *environment.Environment
	NoCookies   bool
	ClearCookies bool
	Verbosity   int

	
	Stages     []Stage       // ramp plan; mutually exclusive with MaxWorkers+Duration
	Duration   time.Duration // wall-clock run time (0 = use stage total)
	MaxWorkers int           // fixed concurrency for Duration mode (ignored when Stages set)
	RPS        float64       // max arrival rate in requests/sec (0 = unlimited)
}