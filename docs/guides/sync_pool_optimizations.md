# `sync.Pool` Optimization Opportunities in ReqX

To ensure the ReqX CLI runs with extremely low Garbage Collection (GC) overhead during high-throughput load tests, we can strategically introduce `sync.Pool` across the architecture. By reusing memory buffers and heavy objects instead of allocating them per-request or per-iteration, we can drastically reduce heap fragmentation and CPU spikes caused by the GC.

Based on an analysis of the codebase, here are the highest-impact places where `sync.Pool` should be implemented:

## 1. JavaScript VM Engine (`goja.Runtime`)
**Location:** `internal/scripting/goja_runner_method.go`
**Current State:** 
```go
vm := goja.New() // Called for *every* script execution
```
**The Problem:** `goja.New()` is extremely expensive. It allocates a massive amount of memory to construct a fresh JavaScript runtime, build global objects, and set up the execution context. Creating this per-request (or twice per request for `prerequest` and `test` scripts) will heavily bottleneck the CPU and exhaust memory under load.
**The Solution:** 
Create a `sync.Pool` for `*goja.Runtime`. 
```go
var vmPool = sync.Pool{
    New: func() any { return goja.New() },
}
```
When executing a script:
1. `vm := vmPool.Get().(*goja.Runtime)`
2. Inject the request context.
3. Run the script.
4. Clear the injected variables (`vm.Set("pm", nil)`) to prevent state bleed.
5. `vmPool.Put(vm)`
*Note: You may also want to cache compiled `goja.Program` instances to avoid recompiling the same script source on every execution.*

## 2. Environment Variable Substitution (String Builder)
**Location:** `internal/runner/collection_runner_method.go` (`replaceVars` method)
**Current State:**
```go
for k, v := range ctx.Environment.Variables {
    out = strings.ReplaceAll(out, "{{"+k+"}}", v)
}
```
**The Problem:** For each variable in the environment, `strings.ReplaceAll` creates a brand new string allocation. If an environment has 20 variables, a single URL or JSON Body string could be re-allocated 20 times. Multiplied by 10,000 requests, this creates millions of orphaned string allocations.
**The Solution:**
Pool a `*strings.Builder` or `*bytes.Buffer`. Instead of looping `ReplaceAll`, do a single-pass regex or template parsing algorithm using the pooled buffer to construct the final string linearly without intermediate allocations.

## 3. HTTP Response Body Reading (`io.ReadAll`)
**Location:** `internal/runner/collection_runner_method.go` 
**Current State:**
```go
bodyBytes, _ := io.ReadAll(resp.Body)
```
**The Problem:** `io.ReadAll` dynamically grows a byte slice to fit the response body limitlessly. For APIs returning large JSON responses, this results in significant GC pressure.
**The Solution:**
Pool `*bytes.Buffer` instances specifically for reading response bodies. 
```go
var bufferPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}
```
Fetch a buffer, use `io.Copy(buf, resp.Body)`, pass the string reference to the scripting context, and release the buffer back to the pool once the script is done processing. Be sure to call `buf.Reset()` before putting it back.

## 4. JSON Payload Formatting Output 
**Location:** `internal/runner/collection_runner_method.go` (VerbosityFull logging)
**Current State:**
```go
var pretty bytes.Buffer
if err := json.Indent(&pretty, bodyBytes, "", "  "); err == nil { ... }
```
**The Problem:** Creates a new `bytes.Buffer` for every full-verbosity request log to properly indent JSON.
**The Solution:**
Reuse the same `bufferPool` mentioned above to fetch a `*bytes.Buffer`, reset it, run `json.Indent` into it, output to the console, and return it to the pool.

## 5. Script Testing Results Slice (`TestResults`)
**Location:** `internal/scripting/goja_runner_method.go`
**Current State:**
```go
testResults := make(TestResults, 0)
```
**The Problem:** A new slice is allocated to hold script results for every execution. 
**The Solution:** 
Pool the backing arrays of the `TestResults` slice.
```go
var testResultsPool = sync.Pool{
    New: func() any { 
       tr := make(TestResults, 0, 10) // pre-allocate capacity
       return &tr
    },
}
```
Pull a slice pointer, pass it to the `pm` object, use it, and truncate it (`*tr = (*tr)[:0]`) before putting it back into the pool.

## Summary Checklist
- [x] Implement `vmPool` in `scripting` for `*goja.Runtime`.
- [x] Implement `bufferPool` in `runner` for reading HTTP responses and indenting JSON logs.
- [ ] Refactor `cr.replaceVars` to use a pooled `*strings.Builder` and single-pass substitution.
- [ ] Implement `testResultsPool` in `scripting` to recycle test result slices.
