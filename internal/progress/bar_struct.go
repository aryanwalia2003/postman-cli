package progress

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Bar renders a real-time progress bar to stdout using carriage-return overwriting.
// In bounded mode (total > 0) it shows a fill bar. In unbounded mode (total == 0,
// duration-based run) it shows a spinner with elapsed time instead.
type Bar struct {
	total     int64 // 0 = unbounded (duration mode)
	done      atomic.Int64
	errors    atomic.Int64
	workers   atomic.Int64 // live worker count (updated externally for stage mode)
	startTime time.Time
	stopCh    chan struct{}
}

// NewBar constructs a Bar. Pass total=0 for duration-based (unbounded) runs.
func NewBar(total, initialWorkers int) *Bar {
	b := &Bar{
		total:     int64(total),
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}
	b.workers.Store(int64(initialWorkers))
	return b
}

// Start begins the background render loop (250ms ticks).
func (b *Bar) Start() {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.render()
			case <-b.stopCh:
				b.render()
				fmt.Println()
				return
			}
		}
	}()
}

// Stop halts the render loop. Call once all jobs are finished.
func (b *Bar) Stop() { close(b.stopCh) }

// IncrementDone records one completed iteration.
func (b *Bar) IncrementDone() { b.done.Add(1) }

// IncrementErrors records one failed iteration.
func (b *Bar) IncrementErrors() { b.errors.Add(1) }

// SetWorkers updates the live worker count (used by stage mode).
func (b *Bar) SetWorkers(n int64) { b.workers.Store(n) }