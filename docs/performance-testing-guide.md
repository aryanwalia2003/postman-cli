# 🚀 ReqX: The Ultimate Performance Testing Guide

Welcome to the **Performance Testing** subsystem of ReqX. This guide will help you understand not just how to use our tool, but the core concepts of load testing and why it matters for your application's reliability.

---

## 1. What is Load Testing?

Imagine your application is a coffee shop. 
- **Functional Testing:** Checking if the coffee tastes good (Does the API return the right JSON?).
- **Load Testing:** Checking what happens when 50 people walk in at the same time and order Lattes.

**Key Concepts:**
*   **Concurrency (Virtual Users):** How many people are using your app *simultaneously*.
*   **Throughput (RPS):** How many requests your server can process *per second*.
*   **Latency (Response Time):** How long a single person has to wait for their coffee.
*   **Bottleneck:** The point where your shop (server) can't keep up, and lines (queues) start forming.

---

## 2. Why ReqX?

ReqX combines the **simplicity of Postman** (friendly JSON collections) with the **power of k6/JMeter** (high-performance concurrent execution). 

Most tools make you write complex JavaScript or XML to run load tests. With ReqX, if you have a collection, you have a load test.

---

## 3. Execution Modes

ReqX offers three distinct ways to apply pressure to your server.

### A. Iteration Mode (The "Batch" Run)
Run a collection a fixed number of times. Best for quick smoke tests or warming up a cache.
*   **Flag:** `-n` (iterations), `-c` (concurrency)
*   **Example:** `reqx run vuc.json -n 100 -c 10`
*   *Interpretation:* Run the collection 100 times, but only 10 at a time in parallel.

### B. Duration Mode (The "Stress" Test)
Run workers continuously for a set amount of time. Better for finding memory leaks or stability issues.
*   **Flag:** `-d` (duration), `-c` (concurrency)
*   **Example:** `reqx run vuc.json -d 5m -c 50`
*   *Interpretation:* 50 virtual users will hit your server as fast as they can for exactly 5 minutes.

### C. Ramping Stages (The "Real World" simulation) 🛡️ *Advanced*
Traffic in the real world doesn't jump from 0 to 100 instantly. It builds up. Stages allow you to "ramp up" and "ramp down" workers.
*   **Flag:** `--stages "duration:workers,..."`
*   **Example:** `reqx run vuc.json --stages "1m:10,3m:50,1m:0"`
*   *Interpretation:* 
    1. Ramp up from 0 to 10 workers over 1 minute.
    2. Ramp up from 10 to 50 workers over the next 3 minutes.
    3. Ramp down to 0 workers over the last 1 minute.

---

## 4. Arrival Rate Control (RPS Capping)

By default, ReqX workers fire requests as fast as your server can answer. If your server is fast, workers will send more requests. This is called "Closed Loop" testing.

If you want to test "Open Loop" (where users arrive at a fixed rate regardless of server speed), use the `--rps` flag.
*   **Example:** `reqx run vuc.json -d 1m -c 50 --rps 20`
*   *Logic:* Even though you have 50 workers ready, ReqX will ensure no more than 20 requests are sent per second.

---

## 5. Reading the Diagnostics

When the test finishes, ReqX gives you a **Professional Grade Report**.

### Percentiles (P95/P99)
Don't trust the Average! Average hides the "angry users."
*   **P95:** The response time experienced by 95% of your users. If P95 is 2s, it means only 5% of users waited longer than 2s.
*   **P99:** The absolute "worst-case" for almost everyone.

### Failure Breakdown
If your test has errors, ReqX shows you exactly which request in your collection is the "Weak Link."
```text
  PER-REQUEST BREAKDOWN
  Request                Runs    Pass    Fail    Avg       P95
  -------------------------------------------------------------
  1. Login               300     300     0       150ms     200ms
  2. Fetch Dashboard     300     250     50      1.2s      4.5s  <-- BOTTLENECK!
```

---

## 6. Pro Tips for Power Users

1.  **Quiet Mode (`-q`):** When running `> 100` workers, terminal logs become unreadable. Use `-q` to see a beautiful real-time progress bar instead.
2.  **Exporting (`--export results.json`):** Save every single request's timing data to a JSON file for analysis in Excel, Python, or Grafana.
3.  **Variable Isolation:** Every worker (Virtual User) gets its own isolated copy of the environment. If Worker A updates a variable, it won't mess up Worker B.

---

## Summary of Commands

| Goal | Command |
| :--- | :--- |
| **Simple Load** | `reqx run coll.json -n 100 -c 10` |
| **Duration Test** | `reqx run coll.json -d 5m -c 20 -q` |
| **Capped Traffic** | `reqx run coll.json -d 2m -c 50 --rps 15` |
| **Ramping Test** | `reqx run coll.json --stages "30s:10,2m:50,30s:0" -q` |
| **Full Export** | `reqx run coll.json -n 1000 -c 100 --export results.json` |

---
*Happy Testing! May your P99s be low and your Success Rates be 100%.*
