package metrics

import (
	"reqx/internal/runner"

	"github.com/HdrHistogram/hdrhistogram-go"
)

type shardResult struct {
	byName          map[string]*RequestStat
	globalHistogram *hdrhistogram.Histogram
	totalSuccess    int
	totalFailures   int
	totalBytesSent  int64
	totalBytesRecv  int64
	statusCodes     map[int]int // global status distribution
}

func consumeShard(ch <-chan runner.RequestMetric) shardResult {
	byName          := make(map[string]*RequestStat, 64)
	globalHistogram := newHistogram()
	statusCodes     := make(map[int]int, 16)
	var totalSuccess, totalFailures int
	var totalBytesSent, totalBytesRecv int64

	for m := range ch {
		stat, ok := byName[m.Name]
		if !ok {
			stat = &RequestStat{
				Name:          m.Name,
				Histogram:     newHistogram(),
				TTFBHistogram: newHistogram(),
				StatusCodes:   make(map[int]int, 8),
			}
			byName[m.Name] = stat
		}

		stat.TotalRuns++
		failed := m.Error != nil || (m.StatusCode != 0 && m.StatusCode >= 400)
		if failed {
			stat.Failures++
			totalFailures++
			addError(&stat.TopErrors, errorMessage(m))
		} else {
			stat.Successes++
			totalSuccess++
		}

		// Latency
		if m.Duration > 0 {
			recordDurationMs(stat.Histogram, m.Duration)
			recordDurationMs(globalHistogram, m.Duration)
		}

		// TTFB
		if m.TTFB > 0 {
			recordDurationMs(stat.TTFBHistogram, m.TTFB)
		}

		// Status code distribution
		if m.StatusCode > 0 {
			stat.StatusCodes[m.StatusCode]++
			statusCodes[m.StatusCode]++
		}

		// Bandwidth
		stat.BytesSent     += m.BytesSent
		stat.BytesReceived += m.BytesReceived
		totalBytesSent     += m.BytesSent
		totalBytesRecv     += m.BytesReceived
	}

	return shardResult{
		byName:          byName,
		globalHistogram: globalHistogram,
		totalSuccess:    totalSuccess,
		totalFailures:   totalFailures,
		totalBytesSent:  totalBytesSent,
		totalBytesRecv:  totalBytesRecv,
		statusCodes:     statusCodes,
	}
}