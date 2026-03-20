package metrics

import (
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

type mergedStats struct {
	order           []string
	byName          map[string]*RequestStat
	globalHistogram *hdrhistogram.Histogram
	totalSuccess    int
	totalFailures   int
	totalBytesSent  int64
	totalBytesRecv  int64
	statusCodes     map[int]int
}

func mergeShardResults(results []shardResult, order []string) mergedStats {
	byName          := make(map[string]*RequestStat, len(order))
	globalHistogram := newHistogram()
	statusCodes     := make(map[int]int, 32)
	var totalSuccess, totalFailures int
	var totalBytesSent, totalBytesRecv int64

	for i := range results {
		r := results[i]
		totalSuccess    += r.totalSuccess
		totalFailures   += r.totalFailures
		totalBytesSent  += r.totalBytesSent
		totalBytesRecv  += r.totalBytesRecv

		if r.globalHistogram != nil {
			_ = globalHistogram.Merge(r.globalHistogram)
		}
		for code, count := range r.statusCodes {
			statusCodes[code] += count
		}
		mergeShardMaps(byName, r.byName)
	}

	return mergedStats{
		order:           order,
		byName:          byName,
		globalHistogram: globalHistogram,
		totalSuccess:    totalSuccess,
		totalFailures:   totalFailures,
		totalBytesSent:  totalBytesSent,
		totalBytesRecv:  totalBytesRecv,
		statusCodes:     statusCodes,
	}
}

func mergeShardMaps(dst map[string]*RequestStat, src map[string]*RequestStat) {
	for name, stat := range src {
		existing, ok := dst[name]
		if !ok {
			dst[name] = stat
			continue
		}
		existing.TotalRuns     += stat.TotalRuns
		existing.Successes     += stat.Successes
		existing.Failures      += stat.Failures
		existing.BytesSent     += stat.BytesSent
		existing.BytesReceived += stat.BytesReceived

		if existing.Histogram == nil {
			existing.Histogram = newHistogram()
		}
		if stat.Histogram != nil {
			_ = existing.Histogram.Merge(stat.Histogram)
		}

		if existing.TTFBHistogram == nil {
			existing.TTFBHistogram = newHistogram()
		}
		if stat.TTFBHistogram != nil {
			_ = existing.TTFBHistogram.Merge(stat.TTFBHistogram)
		}

		if existing.StatusCodes == nil {
			existing.StatusCodes = make(map[int]int, len(stat.StatusCodes))
		}
		for code, count := range stat.StatusCodes {
			existing.StatusCodes[code] += count
		}

		mergeErrorGroups(&existing.TopErrors, stat.TopErrors)
	}
}

func mergeErrorGroups(dst *[]ErrorGroup, src []ErrorGroup) {
	for _, g := range src {
		for i := range *dst {
			if (*dst)[i].Message == g.Message {
				(*dst)[i].Count += g.Count
				goto next
			}
		}
		*dst = append(*dst, g)
	next:
	}
}

func finalizeReport(m mergedStats, totalDuration time.Duration) Report {
	perRequest := make([]RequestStat, 0, len(m.order))
	for _, name := range m.order {
		s := m.byName[name]
		if s == nil {
			continue
		}
		s.P50         = durFromQuantileMs(s.Histogram, 50)
		s.P90         = durFromQuantileMs(s.Histogram, 90)
		s.P95         = durFromQuantileMs(s.Histogram, 95)
		s.P99         = durFromQuantileMs(s.Histogram, 99)
		s.AvgDuration = durFromMeanMs(s.Histogram)
		s.AvgTTFB     = durFromMeanMs(s.TTFBHistogram)
		s.P95TTFB     = durFromQuantileMs(s.TTFBHistogram, 95)
		perRequest = append(perRequest, *s)
	}

	totalReqs := m.totalSuccess + m.totalFailures

	var successRate float64
	if totalReqs > 0 {
		successRate = float64(m.totalSuccess) / float64(totalReqs) * 100
	}

	var rps, throughputMBps float64
	if totalDuration > 0 {
		secs := totalDuration.Seconds()
		rps = float64(totalReqs) / secs
		throughputMBps = float64(m.totalBytesRecv) / secs / 1_000_000
	}

	return Report{
		TotalRequests:      totalReqs,
		TotalSuccess:       m.totalSuccess,
		TotalFailures:      m.totalFailures,
		SuccessRate:        successRate,
		AvgLatency:         durFromMeanMs(m.globalHistogram),
		P50:                durFromQuantileMs(m.globalHistogram, 50),
		P90:                durFromQuantileMs(m.globalHistogram, 90),
		P95:                durFromQuantileMs(m.globalHistogram, 95),
		P99:                durFromQuantileMs(m.globalHistogram, 99),
		RPS:                rps,
		TotalBytesSent:     m.totalBytesSent,
		TotalBytesReceived: m.totalBytesRecv,
		ThroughputMBps:     throughputMBps,
		StatusCodes:        m.statusCodes,
		TotalDuration:      totalDuration,
		PerRequest:         perRequest,
	}
}