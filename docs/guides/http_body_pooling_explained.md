# Deep Dive: Optimizing HTTP Body Reading with `sync.Pool`

This document explains why we should replace `io.ReadAll` with a pooled `bytes.Buffer` in the `CollectionRunner`, as suggested in the performance audit.

## The Problem: `io.ReadAll` & GC Pressure

In the current implementation:
```go
bodyBytes, _ := io.ReadAll(resp.Body)
```

### 1. Dynamic Reallocation
`io.ReadAll` starts with a small internal buffer and grows it as it reads. Every time the buffer reaches capacity, Go allocates a **new, larger slice** and copies the existing data into it. 
*   If a response is 1MB, Go might perform 5-10 separate allocations just to fit the data.
*   The old smaller slices are immediately discarded and become "garbage" that the GC must clean up.

### 2. High Frequency = High Latency
In a load test where you might be making 500 requests per second:
*   Each request creates multiple short-lived byte slices.
*   The Garbage Collector (GC) has to work overtime to scan and reclaim this memory.
*   This leads to **GC "Stop-the-World" pauses**, which directly increases your reported latency percentiles (p99).

---

## The Solution: `sync.Pool` + `bytes.Buffer`

By using a pool, we "borrow" a pre-allocated buffer, use it, and then give it back for the next request.

### 1. The Pool Setup
```go
var bufferPool = sync.Pool{
    New: func() any { 
        // We create a buffer once. It will live for the life of the process.
        return new(bytes.Buffer) 
    },
}
```

### 2. The Optimized Implementation
Instead of `io.ReadAll`, we do this:

```go
// 1. Get a buffer from the pool
buf := bufferPool.Get().(*bytes.Buffer)

// 2. Ensure it's empty (important!)
buf.Reset()

// 3. Read the body into the pooled buffer
_, err := io.Copy(buf, resp.Body)

// 4. Use the data
bodyString := buf.String() 

// 5. Put it back for someone else to use
bufferPool.Put(buf)
```

## Why this is faster:
1.  **Zero Allocations in the Hot Path:** After the first few requests, the pool is "warmed up". Subsequent requests perform **zero** heap allocations for the body buffer.
2.  **Stable Memory Usage:** Your memory profile stays flat instead of "saw-toothing" (going up and down rapidly).
3.  **Lower Latency:** Fewer GC cycles mean the CPU spends more time executing your load test and less time cleaning up memory.

## Implementation Checklist for `internal/runner/collection_runner_method.go`:
- [ ] Define `bodyBufferPool` at the package level or inside `CollectionRunner`.
- [ ] Replace `io.ReadAll(resp.Body)` with `io.Copy(buf, resp.Body)`.
- [ ] Ensure `buf.Reset()` is called before or after use.
- [ ] Ensure the buffer is returned to the pool using `defer` or a manual `Put` call after the script execution is done.
