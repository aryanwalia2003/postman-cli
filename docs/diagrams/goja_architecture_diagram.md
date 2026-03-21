# Goja VM Architecture: Per-VU Ownership

This document explains the transition from a global `sync.Pool` to per-VU VM ownership and pre-compiled scripts.

```mermaid
flowchart TD
    subgraph PLANNING ["planning phase"]
        direction TB
        SOURCE["script source<br/>string[] in collection.Script"]
        COMPILE["goja.Compile()<br/>parse + emit bytecode — once"]
        PROGRAM["*goja.Program (shared, read-only)"]
        
        SOURCE --> COMPILE
        COMPILE --> PROGRAM
    end

    PROGRAM -- "read-only ref" --> VM2_OWN
    PROGRAM -- "read-only ref" --> VM1_OWN

    subgraph VU2 ["VU 2 (goroutine)"]
        direction TB
        VM2_OWN["GojaRunner owns VM<br/>1 goja.Runtime — never shared"]
        EXEC2["vm.RunProgram(compiled)<br/>bytecode only — no re-parse"]
        INJECT2["inject pm/console<br/>fresh per call<br/>clear to Undefined() on exit"]
        
        VM2_OWN --> EXEC2
        EXEC2 --> INJECT2
    end

    subgraph VU1 ["VU 1 (goroutine)"]
        direction TB
        VM1_OWN["GojaRunner owns VM<br/>1 goja.Runtime — never shared"]
        EXEC1["vm.RunProgram(compiled)<br/>bytecode only — no re-parse"]
        INJECT1["inject pm/console<br/>fresh per call<br/>clear to Undefined() on exit"]
        
        VM1_OWN --> EXEC1
        EXEC1 --> INJECT1
    end

    %% Documentation reference
    PROGRAM -.->|"stored in ExecutionPlan"| PLAN_REF[ExecutionPlan]

    %% Color Styling
    style PLANNING fill:#1b4d3e,stroke:#2e7d32,color:#ffffff
    style VU1 fill:#1565c0,stroke:#0d47a1,color:#ffffff
    style VU2 fill:#1565c0,stroke:#0d47a1,color:#ffffff
    
    style SOURCE fill:#424242,stroke:#212121,color:#ffffff
    style COMPILE fill:#424242,stroke:#212121,color:#ffffff
    style PROGRAM fill:#311b92,stroke:#1a237e,color:#ffffff
    
    style VM1_OWN fill:#424242,stroke:#212121,color:#ffffff
    style EXEC1 fill:#424242,stroke:#212121,color:#ffffff
    style INJECT1 fill:#424242,stroke:#212121,color:#ffffff
    
    style VM2_OWN fill:#424242,stroke:#212121,color:#ffffff
    style EXEC2 fill:#424242,stroke:#212121,color:#ffffff
    style INJECT2 fill:#424242,stroke:#212121,color:#ffffff
```

### Key Improvements:
1.  **Pre-compilation:** JavaScript is parsed and compiled to bytecode **once** during the plan building phase. Requests only run the bytecode.
2.  **Strict Isolation:** Each Virtual User (Worker) has its own VM. A script in VU 1 physically cannot access variables from VU 2.
3.  **Zero Pool Overhead:** No more acquire/release logic or `sync.Pool` lock contention.
4.  **Performance:** `vm.RunProgram` is significantly faster than compiling a raw string on every request.
