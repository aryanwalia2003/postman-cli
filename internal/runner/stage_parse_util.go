package runner

import (
	"reqx/internal/errs"
	"strconv"
	"strings"
	"time"
)

// ParseStages converts a stages string like "10s:5,30s:20,10s:0" into []Stage.
// Format per segment: "<duration>:<workers>"  e.g. "30s:20" or "1m:50"
func ParseStages(raw string) ([]Stage, error) {
	if raw == "" {
		return nil, errs.InvalidInput("stages string is empty")
	}
	segments := strings.Split(raw, ",")
	stages := make([]Stage, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		parts := strings.SplitN(seg, ":", 2)
		if len(parts) != 2 {
			return nil, errs.InvalidInput("invalid stage segment: " + seg)
		}
		dur, err := time.ParseDuration(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, errs.Wrap(err, errs.KindInvalidInput, "bad duration in stage: "+seg)
		}
		workers, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || workers < 0 {
			return nil, errs.InvalidInput("bad worker count in stage: " + seg)
		}
		stages = append(stages, Stage{Duration: dur, TargetWorkers: workers})
	}
	return stages, nil
}