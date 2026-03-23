package history

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"reqx/internal/dag"
	"reqx/internal/metrics"
	"reqx/internal/planner"
	"reqx/internal/runner"
)

// SaveRun persists an aggregated test report. Kept for callers that do not
// use a DAG plan.
func (d *DB) SaveRun(collection string, r metrics.Report) error {
	return d.SaveRunWithDAG(collection, r, nil, nil)
}

// SaveRunWithDAG persists an aggregated test report and, when the plan
// contains a DAG, also stores the per-node topology and execution outcomes
// for the first iteration. The graph data powers the UI visualizer.
func (d *DB) SaveRunWithDAG(
	collection string,
	r metrics.Report,
	plan *planner.ExecutionPlan,
	allMetrics [][]runner.RequestMetric,
) error {
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	errorPct := 0.0
	if r.TotalRequests > 0 {
		errorPct = float64(r.TotalFailures) / float64(r.TotalRequests) * 100
	}

	if _, err = tx.Exec(
		`INSERT INTO test_runs(id,collection,total_reqs,rps,p95_ms,error_pct) VALUES(?,?,?,?,?,?)`,
		id, collection, r.TotalRequests, r.RPS, r.P95.Milliseconds(), errorPct,
	); err != nil {
		return err
	}

	for _, s := range r.PerRequest {
		if _, err = tx.Exec(
			`INSERT INTO request_stats(run_id,name,successes,failures,p95_ms,avg_ms) VALUES(?,?,?,?,?,?)`,
			id, s.Name, s.Successes, s.Failures, s.P95.Milliseconds(), s.AvgDuration.Milliseconds(),
		); err != nil {
			return err
		}
	}

	if plan != nil && plan.DAG != nil && len(allMetrics) > 0 {
		if err := insertDAGNodes(tx, id, plan, allMetrics[0]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// insertDAGNodes writes one row per graph node into dag_nodes.
// Level index is computed via TopoSort so the UI can use it for layout.
// Only the first iteration's metrics are stored — sufficient for per-node
// duration and status in the graph view.
func insertDAGNodes(
	tx *sql.Tx,
	runID string,
	plan *planner.ExecutionPlan,
	firstMetrics []runner.RequestMetric,
) error {
	levels, err := dag.TopoSort(plan.DAG)
	if err != nil {
		return err
	}

	// Build a lookup: request index → level index.
	levelOf := make(map[int]int, len(plan.Requests))
	for lvl, nodes := range levels {
		for _, idx := range nodes {
			levelOf[idx] = lvl
		}
	}

	// Build a lookup: request name → first-iteration metric.
	metricOf := make(map[string]runner.RequestMetric, len(firstMetrics))
	for _, m := range firstMetrics {
		metricOf[m.Name] = m
	}

	for i, req := range plan.Requests {
		// Determine status string for this node.
		status := "skipped"
		durationMs := int64(0)
		if m, ok := metricOf[req.Name]; ok {
			switch {
			case m.StatusString == "SKIPPED":
				status = "skipped"
			case m.Error != nil || (m.StatusCode >= 400 && m.StatusCode != 0):
				status = "failed"
			case m.StatusCode > 0:
				status = "success"
			default:
				status = "unknown"
			}
			durationMs = m.Duration.Milliseconds()
		}

		// Serialize depends_on as a JSON array.
		depsJSON, _ := json.Marshal(req.DependsOn)

		if _, err := tx.Exec(
			`INSERT INTO dag_nodes(run_id,name,status,duration_ms,level_idx,depends_on) VALUES(?,?,?,?,?,?)`,
			runID, req.Name, status, durationMs, levelOf[i], string(depsJSON),
		); err != nil {
			return err
		}
	}

	return nil
}