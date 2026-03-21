This is a high-level architectural refactor. Use this guide to direct your coding agent to implement the **Execution Plan Pattern**.

---

# 🛠️ Task: Refactor ReqX to use Immutable Execution Plans

## 1. The Problem: Shared Mutable State

Currently, `ReqX` treats the `Collection` object as both the **Definition** and the **Runtime State**.

- **The Issue:** When running with multiple workers (`-c 5000`), the `run` command logic (filtering, injection) modifies the shared `Collection` pointer.
- **The Risk:** This creates a critical **Data Race**. If one worker is reading the list of requests while the main thread is filtering or injecting into that same list, the program will crash or produce non-deterministic results.
- **The Verdict:** The `Collection` should be a **read-only blueprint**.

## 2. The Solution: The "Planner" Architecture

We will introduce a **Planner** layer between the Loading phase and the Execution phase.

1. **Load:** Read the JSON file into a `Collection` struct. **(Immutable thereafter)**.
2. **Plan:** The `Planner` takes the `Collection` + CLI Flags (`--request`, `--inject`) and produces a one-time `ExecutionPlan`.
3. **Execute:** The `Scheduler` and `Workers` consume the `ExecutionPlan`, not the `Collection`.

---

## 3. Code Patches & Steps

### Step 1: Create the `planner` Package

Create a new directory `internal/planner`. This package will house the logic for transforming a raw collection into a specific run scenario.

**File: `internal/planner/plan_struct.go`**

```go
package planner

import "reqx/internal/collection"

// ExecutionPlan is the immutable set of instructions for a specific test run.
type ExecutionPlan struct {
	// The final ordered list of requests to be executed per iteration.
	Requests []collection.Request
}
```

**File: `internal/planner/planner_method.go`**
Move the filtering and injection logic from `cmd/run_cmd_ctor.go` into here.

```go
package planner

import (
	"reqx/internal/collection"
	"strings"
    "strconv"
    "github.com/fatih/color"
)

type PlanConfig struct {
	RequestFilters []string
	InjIndex       string
	InjName        string
	InjMethod      string
	InjURL         string
	InjBody        string
	InjHeaders     []string
}

func BuildExecutionPlan(coll *collection.Collection, cfg PlanConfig) (*ExecutionPlan, error) {
	// 1. Start with the full list from the collection
	finalRequests := make([]collection.Request, len(coll.Requests))
	copy(finalRequests, coll.Requests)

	// 2. Apply Injection (Logic moved from run_cmd_ctor)
	if cfg.InjIndex != "" && cfg.InjName != "" {
        // ... (Insert injection logic here, updating finalRequests)
    }

	// 3. Apply Filtering (Logic moved from run_cmd_ctor)
	if len(cfg.RequestFilters) > 0 {
        // ... (Insert filtering logic here, updating finalRequests)
    }

	return &ExecutionPlan{Requests: finalRequests}, nil
}
```

### Step 2: Update the `runner` Configs

The Scheduler and Workers should no longer know about the `Collection`. They only care about the `ExecutionPlan`.

**File: `internal/runner/scheduler_config_struct.go` & `worker_pool_method.go`**

- Replace `Coll *collection.Collection` with `Plan *planner.ExecutionPlan`.

**Example Change in `internal/runner/worker_pool_method.go`:**

```go
type WorkerConfig struct {
	Plan         *planner.ExecutionPlan // Changed from Coll
	BaseEnv      *environment.Environment
    // ...
}
```

### Step 3: Refactor `cmd/run_cmd_ctor.go`

This is the main cleanup. The `RunE` function should now follow this clean flow:

1. Load Collection JSON.
2. Load Environment / Personas.
3. Call `planner.BuildExecutionPlan(...)`.
4. Pass the resulting `Plan` to the `Scheduler` or `WorkerPool`.

**Remove the manual slice manipulation (appending/filtering) currently sitting inside the `RunE` loop.**

### Step 4: Update the Worker Execution Logic

In `scheduler_worker_method.go` and `worker_pool_method.go`, update the iteration loop to read from the plan.

**File: `internal/runner/scheduler_worker_method.go`**

```go
func (s *Scheduler) executeOne(ctx context.Context, workerID int) ([]RequestMetric, error) {
    // ... setup context ...

    // Execute the requests defined in the plan
    metrics, err := engine.Run(s.cfg.Plan.Requests, rtCtx) // s.cfg.Coll becomes s.cfg.Plan.Requests

    // ...
}
```

---

## Success Criteria for the Agent

1. **Compilation:** `go build -o reqx.exe main.go` runs without errors.
2. **Immutability:** No file in `internal/runner` or `internal/http_executor` performs an `append` or reassignment on a `collection.Request` slice.
3. **Correctness:** Running `reqx run vuc.json -f Login` still correctly filters the requests, but does so by creating a `Plan`, not by modifying the `Collection`.
4. **Performance:** Running with `-c 100` works flawlessly with no data races (Verify using `go run -race main.go ...`).

---

**Note to Agent:** Ensure all internal imports are updated to `reqx/internal/...` following the recent rebrand. Don't forget to add `import "reqx/internal/planner"` where needed.
