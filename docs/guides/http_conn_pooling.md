
# 🌐 ReqX Architecture: High-Performance Connection Pooling

## 1. The Current Problem: "Short-Lived Clients"
Abhi ReqX ka execution model har iteration ya har worker spawn par ek naya `http.Client` aur `http_executor` create karta hai. 

### Current Logic Flow:
1. Worker starts iteration.
2. `http_executor.NewDefaultExecutor()` is called.
3. Inside, a fresh `&http.Client{}` is instantiated.
4. Request is sent.
5. Iteration ends, the executor and client are garbage collected.

**The Reality:** Go mein `http.Client` ke andar ek **Connection Pool** hota hai. Jab aap naya client banate hain, toh woh pool khali hota hai. Jab client destroy hota hai, toh saare active TCP connections band ho jaate hain.

---

## 2. The Risks of "No Pooling" ⚠️

### A. The Handshake Tax (Latency)
Har baar jab worker request bhejta hai, use:
- **TCP 3-Way Handshake** (1.5 RTT) karna padta hai.
- **TLS Handshake** (1-2 RTT) karna padta hai agar HTTPS hai.
**Result:** Agar aapka server 50ms mein response de raha hai, toh aapka tool handshake mein hi extra 100ms barbaad kar raha hai. Aap asli performance test kar hi nahi pa rahe.

### B. Ephemeral Port Exhaustion (OS Level Crash)
Jab ek TCP connection "Gracefully" close hota hai, toh OS us socket ko **`TIME_WAIT`** state mein daal deta hai (usually for 60-120 seconds). 
- Windows/Linux mein "Ephemeral Ports" ki limit hoti hai (approx 16k to 64k).
- Agar aap 5,000 workers ke saath high RPS load testing kar rahe hain aur pooling nahi hai, toh aap 10 seconds mein saare ports "Consume" kar lenge.
- **Symptoms:** `dial tcp: lookup: socket: too many open files` ya `connection refused` errors.

### C. CPU Overhead
Handshakes computationally expensive hote hain (especially TLS/Encryption). Server aur Client dono ka CPU faltu ke handshakes mein waste hota hai.

---

## 3. The Solution: Shared `http.Transport` Architecture 🏗️

Humein **"Engine"** (Networking Layer) aur **"Identity"** (User Session) ko decouple karna hoga.

- **`http.Transport` (The Engine):** Yeh global hoga aur saare workers isse share karenge. Iska kaam hai TCP connections ka "Pool" maintain karna (Keep-Alive).
- **`http.Client` (The Identity):** Har worker ka apna client aur **CookieJar** hoga, lekin woh background mein usi global engine (Transport) ko use karega.

### Revised Architecture Snippet:

```go
// internal/http_executor/default_executor_struct.go

var (
    // Shared Transport: Ek baar banta hai, poora tool use karta hai.
    globalTransport = &http.Transport{
        // Max connections in the pool
        MaxIdleConns:        10000, 
        // Max connections per host (Crucial for load testing!)
        MaxIdleConnsPerHost: 2000,  
        // How long to keep an idle connection alive
        IdleConnTimeout:     90 * time.Second,
        // Performance tuning
        DisableCompression:  false,
        ForceAttemptHTTP2:   true,
    }
)

type DefaultExecutor struct {
    client *http.Client
    jar    *ManagedCookieJar
}
```

```go
// internal/http_executor/default_executor_ctor.go

func NewDefaultExecutor() *DefaultExecutor {
    jar := NewManagedCookieJar()
    return &DefaultExecutor{
        jar: jar,
        client: &http.Client{
            Timeout:   60 * time.Second,
            Jar:       jar,
            // Sabse important line: Saare clients same transport use karenge
            Transport: globalTransport, 
        },
    }
}
```

---

## 4. Key Benefits of this Refactor 🌟

1. **Sub-Millisecond Overhead:** Handshakes sirf tabhi honge jab connection pool khali ho. Connection reuse hone par network latency zero ho jayegi.
2. **True RPS:** Server ko actual "Pressure" mehsoos hoga kyunki connections pehle se "Warm" honge.
3. **Resource Efficiency:** Ports exhaust nahi honge. Ek worker 1,000 requests bhej sakta hai sirf ek hi TCP socket ka use karke.
4. **Realistic Simulation:** Modern browsers aur mobile apps bhi connection pooling use karte hain. Yeh unke behavior ko accurately simulate karta hai.

---

## 5. Future Prospects (The "Pro" Roadmap) 🚀

### A. HTTP/2 and Multiplexing
Shared transport ke saath, ReqX automatically **HTTP/2** use karne lagega (agar server support kare). Iska matlab ek hi TCP connection ke andar parallel streams chalengi, jo performance ko 10x badha sakti hain.

### B. Connection Warm-up
Run shuru hone se pehle, hum "Dry-run" karke pool ko pehle se connections se "Fill" (warm) kar sakte hain taaki Iteration 1 se hi maximum speed mile.

### C. Dynamic Transport Switching
Different test scenarios ke liye alag transport profiles:
- `Fast-Ramping`: High `MaxIdleConnsPerHost`.
- `Mobile-Simulation`: Simulation of high packet loss and slow handshakes.

---