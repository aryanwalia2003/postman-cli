package progress

import (
	"fmt"
	"strings"
	"time"
)

const barWidth = 28

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// render writes a single overwriting line to stdout.
func (b *Bar) render() {
	done    := b.done.Load()
	errs    := b.errors.Load()
	workers := b.workers.Load()
	elapsed := time.Since(b.startTime)

	var rps float64
	if elapsed.Seconds() > 0 {
		rps = float64(done) / elapsed.Seconds()
	}

	if b.total > 0 {
		b.renderBounded(done, errs, workers, rps, elapsed)
	} else {
		b.renderUnbounded(done, errs, workers, rps, elapsed)
	}
}

// renderBounded shows a fill bar for iteration-based runs.
func (b *Bar) renderBounded(done, errs, workers int64, rps float64, elapsed time.Duration) {
	pct := float64(done) / float64(b.total)
	if pct > 1 { pct = 1 }
	filled := int(pct * barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Printf("\r  [%s] %3.0f%%  Workers: %d  Done: %d/%d  Errors: %d  RPS: %.1f  %s   ",
		bar, pct*100, workers, done, b.total, errs, rps, fmtElapsed(elapsed))
}

// renderUnbounded shows a spinner for duration-based runs (total unknown).
func (b *Bar) renderUnbounded(done, errs, workers int64, rps float64, elapsed time.Duration) {
	frame := spinFrames[(int(elapsed/250/time.Millisecond))%len(spinFrames)]
	fmt.Printf("\r  %s  Workers: %d  Done: %d  Errors: %d  RPS: %.1f  %s   ",
		frame, workers, done, errs, rps, fmtElapsed(elapsed))
}

func fmtElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", m, s)
}