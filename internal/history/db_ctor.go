package history

import (
	"database/sql"
	"path/filepath"

	"reqx/internal/storage"

	_ "github.com/glebarez/go-sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS test_runs (
	id          TEXT PRIMARY KEY,
	ts          DATETIME DEFAULT CURRENT_TIMESTAMP,
	collection  TEXT    NOT NULL,
	total_reqs  INTEGER NOT NULL,
	rps         REAL    NOT NULL,
	p95_ms      INTEGER NOT NULL,
	error_pct   REAL    NOT NULL
);

CREATE TABLE IF NOT EXISTS request_stats (
	run_id       TEXT    NOT NULL REFERENCES test_runs(id),
	name         TEXT    NOT NULL,
	successes    INTEGER NOT NULL,
	failures     INTEGER NOT NULL,
	p95_ms       INTEGER NOT NULL,
	avg_ms       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS dag_nodes (
	run_id       TEXT    NOT NULL REFERENCES test_runs(id),
	name         TEXT    NOT NULL,
	status       TEXT    NOT NULL,
	duration_ms  INTEGER NOT NULL,
	level_idx    INTEGER NOT NULL,
	depends_on   TEXT    NOT NULL
);
`

// Open opens (or creates) the history database at ~/.reqx/history.db.
func Open() (*DB, error) {
	dir, err := storage.GetDefaultConfigDir()
	if err != nil {
		return nil, err
	}
	if err := storage.EnsureDirExists(dir); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", filepath.Join(dir, "history.db"))
	if err != nil {
		return nil, err
	}
	// WAL mode: reads never block writes.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}
	if _, err := conn.Exec(schema); err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() { d.conn.Close() }
