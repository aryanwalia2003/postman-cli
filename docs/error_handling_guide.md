# Robust Error Handling Guide

Welcome to the `errs` package! This system is designed to provide consistent, robust, and developer-friendly error handling across the entire `postman-cli` project.

You no longer need to use standard `fmt.Errorf()` blindly. Instead, this system allows you to easily enforce safety, attach metadata, automatically capture stack traces, and translate errors directly to clean, client-safe JSON HTTP responses!

## 1. When to Use What Constructor

Whenever you encounter a failure in your logic, use the `errs` constructors. They are designed to be extremely ergonomic so that utilizing the system feels effortless.

### `errs.New(kind, msg)`
Use this when you are generating a *brand new* error.
```go
if user == nil {
    return errs.New(errs.KindNotFound, "User profile could not be found.")
}
```

### `errs.Wrap(err, kind, msg)`
Use this when you receive an error from a *third-party library* or an *internal package* and you want to pass it up the stack with more context. 
```go
bytes, err := os.ReadFile("config.json")
if err != nil {
    return errs.Wrap(err, errs.KindInvalidInput, "Failed to read configuration file.")
}
```

### Ergonomic Shorthands
To make it even faster, we've provided domains-specific shorthands!
- `errs.NotFound("User missing")`
- `errs.InvalidInput("Invalid JSON body")`
- `errs.Internal("Failed to calculate stats")`
- `errs.Database(err, "Failed to connect to Postgres")`

## 2. Attaching Metadata (Context)

When an error occurs, the standard message (`"Failed to load user"`) isn't always enough to debug *why* it failed. You can heavily enrich your errors with Key-Value Metadata so that your logs expose the exact state of the universe when things broke!

```go
appErr := errs.Database(sqlErr, "Query execution failed")

// Attach rich context for logs
appErr = errs.AddMetadata(appErr, errs.Metadata{
    "user_id": 42,
    "query": "SELECT * FROM users WHERE active = true",
    "retries": 3,
})
return appErr
```
> **Note:** Metadata is *never* sent to external clients. It is only logged internally to protect your system's data!

## 3. HTTP Server Translation
The real magic is that API handlers no longer need to format `http.Error` manually! 

If you are writing an HTTP service, you only need to call ONE translation function, and the `errs` system will figure out the rest. It will determine the proper HTTP status code based on the `Kind` (e.g. `KindNotFound` = 404, `KindInvalidInput` = 400), send a safe JSON response, and dump the full stack trace and metadata to the server logs.

```go
func GetUserHandler(w http.ResponseWriter, r *http.Request) {
    user, err := fetchUserFromDatabase()
    
    if err != nil {
        // Just pass it to WriteHTTPError!
        errs.WriteHTTPError(w, err)
        return
    }
    
    json.NewEncoder(w).Encode(user)
}
```

## 4. Recovering from Panics!

To guarantee the backend server **never crashes**, simply inject `errs.RecoverHTTP(w)` into any HTTP middleware/handler via a defer statement:

```go
func CriticalHandler(w http.ResponseWriter, r *http.Request) {
    defer errs.RecoverHTTP(w) // <--- Catches crashes, logs stack trace, returns HTTP 500
    
    // Simulate a crash
    var ptr *int
    fmt.Println(*ptr)
}
```

---
**Summary Rule of Thumb:** 
- Stop using `fmt.Errorf()`.
- Use `errs.Wrap()` to add context.
- Use `errs.AddMetadata()` when variables matter.
- Always use `errs.WriteHTTPError()` and `errs.RecoverHTTP()` whenever outputting to HTTP!
