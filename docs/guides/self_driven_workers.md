🚀 ReqX Architecture Evolution: Moving to Self-Driven Workers
1. The Problem: The "Job-Queue" Bottleneck

Currently, ReqX uses a Centralized Job Distribution model (Pull-based).

How it works: The Scheduler acts as a dispatcher, pushing WorkerJob structs into a Go channel. Workers wait for a job, execute one collection iteration, and then go back to the queue.

The Bottlenecks:

Channel Contention: At high concurrency (e.g., 5,000+ workers), every worker fighting for the same channel lock creates a performance bottleneck.

State Fragmentation: Because every iteration is a "new job," it's harder to maintain a persistent user session (like a Doctor staying logged in across 50 requests).

Micro-management Overhead: The Scheduler spends too much CPU time managing individual iterations instead of orchestrating the overall test.

2. The Solution: Self-Driven Workers (Autonomous VUs)

We are moving to a Virtual User (VU) Model. Instead of workers pulling tasks, the Scheduler spawns "Autonomous Workers" that know exactly what to do.

The New Paradigm

Scheduler: Becomes a "Conductor." It spawns 
𝑁
N
 workers and gives them a stop signal (Context).

Worker: Becomes a "Virtual User." It starts once and runs the collection in a continuous for loop until it's told to stop.

3. Key Benefits 🌟
A. Massively Scalable (Lock-Free)

Since there is no central job queue, there is zero contention. Each worker runs independently in its own goroutine memory space. This allows ReqX to scale to tens of thousands of workers on a single machine.

B. Realistic User Simulation (Session Persistence)

In the real world, a user doesn't "reset" after every action.

Current Model: Iteration 1 (Login) -> Reset -> Iteration 2 (Login).

Self-Driven Model: Worker logs in once and performs subsequent actions using the same auth token and cookies for the entire test duration.

C. True Duration-Based Testing

It becomes trivial to implement --duration 5m. Workers just loop until the timer expires. We don't need to guess how many iterations to pre-generate.

4. How We Implement the Change
Step 1: Update the Worker Logic

Instead of a single execution, the worker function now contains a loop.

code
Go
download
content_copy
expand_less
// internal/runner/worker_pool_method.go

func runWorker(ctx context.Context, id int, cfg WorkerConfig) {
    // 1. Setup isolated state (Env Clone, Persona) ONCE
    workerCtx := setupWorkerContext(cfg, id)
    
    for {
        select {
        case <-ctx.Done():
            // Exit signal received (Duration reached or Ctrl+C)
            return
        default:
            // 2. Execute the entire collection
            metrics, err := engine.Run(cfg.Plan, workerCtx)
            
            // 3. Report metrics to sharded aggregator
            reportMetrics(metrics)
            
            // 4. Update atomic iteration counter for Progress Bar
            atomic.AddInt64(&globalIterations, 1)
        }
    }
}
Step 2: Scheduler as a Conductor

The Scheduler no longer feeds a channel; it manages a context.

code
Go
download
content_copy
expand_less
// internal/runner/scheduler_run_method.go

func (s *Scheduler) Run(duration time.Duration) {
    // Create a context that cancels when the duration is up
    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()

    for i := 0; i < s.numWorkers; i++ {
        s.wg.Add(1)
        go func(id int) {
            defer s.wg.Done()
            runWorker(ctx, id, s.cfg) // Workers start their own loops
        }(i)
    }

    s.wg.Wait() // Wait for all workers to finish their final loop
}
5. Summary of Architectural Shift
Feature	Job-Queue (Old)	Self-Driven (New)
Communication	Channel per job	Single Context Signal
Persistence	Hard (State resets)	Native (State persists in loop)
Scalability	Limited by Channel Lock	Near-Infinite (Lock-free)
Control	Number of Iterations (-n)	Time Duration (-d) & Iterations
