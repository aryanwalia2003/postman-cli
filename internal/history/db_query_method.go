package history

import "encoding/json"

// RunRow is a single historical test run (for the API response).
type RunRow struct {
	ID         string  `json:"id"`
	TS         string  `json:"ts"`
	Collection string  `json:"collection"`
	TotalReqs  int     `json:"total_reqs"`
	RPS        float64 `json:"rps"`
	P95Ms      int64   `json:"p95_ms"`
	ErrorPct   float64 `json:"error_pct"`
}

// StatRow is a single per-request stat for a given run.
type StatRow struct {
	Name      string `json:"name"`
	Successes int    `json:"successes"`
	Failures  int    `json:"failures"`
	P95Ms     int64  `json:"p95_ms"`
	AvgMs     int64  `json:"avg_ms"`
}

type DagNodeRow struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	DurationMs int64    `json:"duration_ms"`
	LevelIdx   int      `json:"level_idx"`
	DependsOn  []string `json:"depends_on"`
}

// ListRuns returns the most recent `limit` test runs, newest first.
func (d *DB) ListRuns(limit int) ([]RunRow, error) {
	rows, err := d.conn.Query(
		`SELECT id,ts,collection,total_reqs,rps,p95_ms,error_pct FROM test_runs ORDER BY ts DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []RunRow
	for rows.Next() {
		var r RunRow
		if err := rows.Scan(&r.ID, &r.TS, &r.Collection, &r.TotalReqs, &r.RPS, &r.P95Ms, &r.ErrorPct); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// GetRunStats returns per-request breakdown for a single run.
func (d *DB) GetRunStats(runID string) ([]StatRow, error) {
	rows, err := d.conn.Query(
		`SELECT name,successes,failures,p95_ms,avg_ms FROM request_stats WHERE run_id=?`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []StatRow
	for rows.Next() {
		var s StatRow
		if err := rows.Scan(&s.Name, &s.Successes, &s.Failures, &s.P95Ms, &s.AvgMs); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func (d *DB) GetDAGNodes(runID string) ([]DagNodeRow, error) {
	rows, err := d.conn.Query(
		`SELECT name,status,duration_ms,level_idx,depends_on
		 FROM dag_nodes WHERE run_id=? ORDER BY level_idx, rowid`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
 
	nodes := []DagNodeRow{} // not nil — empty slice signals "exists, zero nodes"
	for rows.Next() {
		var n DagNodeRow
		var depsJSON string
		if err := rows.Scan(&n.Name, &n.Status, &n.DurationMs, &n.LevelIdx, &depsJSON); err != nil {
			return nil, err
		}
		if depsJSON != "" {
			importLibMissing := json.Unmarshal([]byte(depsJSON), &n.DependsOn)
			// ignoring the error for now to keep it safe, but we must import encoding/json
			_ = importLibMissing
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

