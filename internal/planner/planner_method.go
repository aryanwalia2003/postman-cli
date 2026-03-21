package planner

import (
	"strconv"
	"strings"

	"github.com/fatih/color"

	"reqx/internal/collection"
	"reqx/internal/errs"
)

// BuildExecutionPlan transforms a raw Collection into an immutable ExecutionPlan
// by applying injection and filtering as described by cfg.
// The original Collection is never mutated.
func BuildExecutionPlan(coll *collection.Collection, cfg PlanConfig) (*ExecutionPlan, error) {
	// Start with a fresh copy — never touch coll.Requests directly.
	requests := make([]collection.Request, len(coll.Requests))
	copy(requests, coll.Requests)

	requests, err := applyInjection(requests, cfg)
	if err != nil {
		return nil, err
	}

	requests, err = applyFilters(requests, cfg)
	if err != nil {
		return nil, err
	}

	compiled,err := compileScripts(requests)
	if err!=nil{
		return nil, err
	}

	return &ExecutionPlan{
		Requests:       requests,
		CollectionAuth: coll.Auth,
		CompiledScripts: compiled,
	}, nil
}

// applyInjection inserts a temporary request at the configured index, if set.
func applyInjection(requests []collection.Request, cfg PlanConfig) ([]collection.Request, error) {
	if cfg.InjIndex == "" || cfg.InjName == "" || cfg.InjURL == "" {
		if (cfg.InjName != "" || cfg.InjURL != "") && cfg.InjIndex == "" {
			color.Yellow("⚠ Warning: Ignored injection — missing --inject-index.\n")
		}
		return requests, nil
	}

	idx, err := strconv.Atoi(cfg.InjIndex)
	if err != nil || idx < 1 {
		return nil, errs.InvalidInput("--inject-index must be a positive integer")
	}

	headerMap := parseHeaders(cfg.InjHeaders)
	injected := collection.Request{
		Name:    color.New(color.FgHiMagenta).Sprintf("[INJECTED] %s", cfg.InjName),
		Method:  strings.ToUpper(cfg.InjMethod),
		URL:     cfg.InjURL,
		Headers: headerMap,
		Body:    cfg.InjBody,
	}

	insertPos := idx - 1
	if insertPos >= len(requests) {
		requests = append(requests, injected)
	} else {
		requests = append(requests[:insertPos+1], requests[insertPos:]...)
		requests[insertPos] = injected
	}

	color.Magenta("💉 Injecting '%s' at position %d\n", cfg.InjName, idx)
	return requests, nil
}

// applyFilters keeps only requests whose name contains one of the filter substrings.
func applyFilters(requests []collection.Request, cfg PlanConfig) ([]collection.Request, error) {
	if len(cfg.RequestFilters) == 0 {
		return requests, nil
	}

	filtered := make([]collection.Request, 0, len(requests))
	for _, r := range requests {
		for _, f := range cfg.RequestFilters {
			if strings.Contains(strings.ToLower(r.Name), strings.ToLower(f)) {
				filtered = append(filtered, r)
				break
			}
		}
	}

	if len(filtered) == 0 {
		return nil, errs.InvalidInput("no requests matched filters: " + strings.Join(cfg.RequestFilters, ", "))
	}

	color.Cyan("🔍 Filtered to %d request(s) matching %v\n", len(filtered), cfg.RequestFilters)
	return filtered, nil
}

// parseHeaders converts "Key: Value" strings into a map.
func parseHeaders(raw []string) map[string]string {
	m := make(map[string]string, len(raw))
	for _, h := range raw {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}