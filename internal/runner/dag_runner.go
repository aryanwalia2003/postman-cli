package runner

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
	"github.com/fatih/color"

	"reqx/internal/dag"
	"reqx/internal/planner"
)

// RunDAG executes the requests in plan according to the dependency graph.
//
// Nodes within the same topological level run concurrently. The level boundary
// acts as a barrier: every node in level N finishes before any node in level
// N+1 starts. This gives the maximum safe parallelism the declared dependencies
// allow.
//
// A node is skipped when:
//   - any of its direct dependencies was itself skipped, or
//   - its condition expression evaluates to false against the dependency result.
//
// Skipped nodes record a RequestMetric with StatusString == "SKIPPED".
func (cr *CollectionRunner) RunDAG(plan *planner.ExecutionPlan, ctx *RuntimeContext) ([]RequestMetric, error) {
	levels, err := dag.TopoSort(plan.DAG)
	if err != nil {
		return nil, err
	}

	n := len(plan.Requests)
	allMetrics := make([]RequestMetric, n)
	evalCtxs := make([]dag.EvalContext, n)
	skipped := make([]bool, n)

	for levelIdx, level := range levels {
		if cr.verbosity >= VerbosityNormal {
			color.Cyan("\n[DAG] Level %d — %d node(s) running in parallel\n", levelIdx, len(level))
		}

		type nodeResult struct {
			reqIdx int
			metric RequestMetric
			eval   dag.EvalContext
			skip   bool
		}

		results := make([]nodeResult, len(level))
		var wg sync.WaitGroup

		for slot, nodeIdx := range level {
			wg.Add(1)
			go func(slot, reqIdx int) {
				defer wg.Done()

				req := plan.Requests[reqIdx]

				// Check every declared dependency for skip propagation and conditions.
				for _, depIdx := range plan.DAG.Edges[reqIdx] {
					if skipped[depIdx] {
						if cr.verbosity >= VerbosityNormal {
							color.Yellow("[DAG] Skip %q — dep %q was skipped\n",
								req.Name, plan.Requests[depIdx].Name)
						}
						results[slot] = nodeResult{
							reqIdx: reqIdx,
							metric: RequestMetric{Name: req.Name, Protocol: "HTTP", StatusString: "SKIPPED"},
							skip:   true,
						}
						return
					}

					if req.Condition != "" {
						ok, condErr := dag.EvalCondition(req.Condition, evalCtxs[depIdx])
						if condErr != nil {
							color.Yellow("[DAG] Condition error on %q: %v — skipping\n", req.Name, condErr)
							results[slot] = nodeResult{
								reqIdx: reqIdx,
								metric: RequestMetric{Name: req.Name, Protocol: "HTTP", StatusString: "SKIPPED", ErrorMsg: condErr.Error()},
								skip:   true,
							}
							return
						}
						if !ok {
							if cr.verbosity >= VerbosityNormal {
								color.Yellow("[DAG] Skip %q — condition %q not met\n", req.Name, req.Condition)
							}
							results[slot] = nodeResult{
								reqIdx: reqIdx,
								metric: RequestMetric{Name: req.Name, Protocol: "HTTP", StatusString: "SKIPPED"},
								skip:   true,
							}
							return
						}
					}
				}

				// Build a single-element sub-plan so the existing Run() dispatches
				// the request through all HTTP/WS/SIO logic without duplication.
				// DAG is nil so Run() takes the normal linear path for this one node.
				subPlan := &planner.ExecutionPlan{
					Requests:       plan.Requests[reqIdx : reqIdx+1],
					CollectionAuth: plan.CollectionAuth,
					// CompiledScripts keys use the original request index; remap to 0.
					CompiledScripts: remapCompiledScripts(plan, reqIdx),
					DAG:             nil,
					IsDAGNode:       true,
				}

				nodeMetrics, runErr := cr.Run(subPlan, ctx)
				if runErr != nil {
					fmt.Printf("[DAG] Node %q error: %v\n", req.Name, runErr)
				}

				var m RequestMetric
				if len(nodeMetrics) > 0 {
					m = nodeMetrics[0]
				} else {
					m = RequestMetric{Name: req.Name, Protocol: "HTTP", Error: runErr}
				}

				results[slot] = nodeResult{
					reqIdx: reqIdx,
					metric: m,
					eval: dag.EvalContext{
						StatusCode: m.StatusCode,
						DurationMs: m.Duration.Milliseconds(),
						Failed:     m.Error != nil || (m.StatusCode >= 400 && m.StatusCode != 0),
					},
				}
			}(slot, nodeIdx)
		}

		wg.Wait()

		for _, r := range results {
			allMetrics[r.reqIdx] = r.metric
			evalCtxs[r.reqIdx] = r.eval
			skipped[r.reqIdx] = r.skip
		}
	}

	// Final barrier for all background connections started during DAG execution.
	// Only the top-level RunDAG call (on the main plan) should do this wait.
	if plan.IsDAGNode {
		// sub-nodes keep their connections alive
	} else {
		defer func() {
			if cr.verbosity >= VerbosityNormal {
				color.Cyan("\n[DAG] All levels finished. Waiting for background connections...\n")
			}
			ctx.AsyncStopOnce.Do(func() { close(ctx.AsyncStop) })
			ctx.AsyncWG.Wait()
			if cr.verbosity >= VerbosityNormal {
				color.Green("[DAG] All background connections closed cleanly.\n")
			}
		}()
	}

	return allMetrics, nil
}

// remapCompiledScripts returns a CompiledScripts map with reqIdx remapped to 0.
// Run() looks up compiled scripts by the request's position in the sub-plan's
// Requests slice (which is always 0 for single-node sub-plans).
func remapCompiledScripts(plan *planner.ExecutionPlan, reqIdx int) map[planner.ScriptKey]*goja.Program {
	if plan.CompiledScripts == nil {
		return nil
	}
	out := make(map[planner.ScriptKey]*goja.Program)
	for k, v := range plan.CompiledScripts {
		if k.RequestIndex == reqIdx {
			out[planner.ScriptKey{RequestIndex: 0, ScriptType: k.ScriptType}] = v
		}
	}
	return out
}
