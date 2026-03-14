# 🚀 ReqX: High-Performance, Scriptable API Client

**ReqX** is a lightweight, terminal-centric API execution engine built in Go. It is designed for developers who value speed, automation, and a clean CLI experience. ReqX allows you to run Postman-style collections, debug real-time Socket.IO streams, and automate complex test flows without ever leaving your terminal.

---

## ⚡ Quick Install (Windows)

Open PowerShell as **Administrator** and run this one-liner to download and install the latest version:
```powershell
iwr -useb https://raw.githubusercontent.com/aryanwalia2003/reqx/main/install.ps1 | iex
```

---

## ✨ Features at a Glance

- **🚀 Blazing Fast**: Built in Go for near-instant execution of large test suites.
- **📡 Protocol Support**: Full HTTP/HTTPS support and interactive Socket.IO v4 REPL.
- **🔐 Stateful Flows**: Automatically handles environment variables and cookie persistence between requests.
- **📜 JS Scripting**: Use Postman-style JavaScript (`pm.env.set`, `pm.test`) for advanced test logic.
- **🔄 Multi-Iteration**: Run load tests with a single command and view aggregated performance summaries.
- **📂 GUI-Free Management**: Add, move, or list requests in your collections directly via the CLI.
- **🛠 Zero Dependencies**: A single binary that works right out of the box.

---

## 📚 Core Command Guide

### 1. `run`: The Execution Engine
Execute full collections with variables, cookies, and iterations.

```bash
# Basic run with environment
reqx run collection.json -e dev.json

# Performance Test: 10 iterations with aggregated summary
reqx run collection.json -n 10

# Targeted Run: Only requests containing "Auth"
reqx run collection.json -f "Auth"

# Verbose Mode: See full request/response headers and bodies
reqx run collection.json -v
```

### 2. `req`: The Quick Requester
For ad-hoc, curl-style calls with the power of ReqX variables.

```bash
# Simple GET
reqx req https://api.github.com/users/aryanwalia2003

# POST with JSON body and environment secrets
reqx req "{{base_url}}/login" -e prod.json -X POST -d '{"user":"test"}'
```

### 3. `sio`: The Socket.IO REPL
Debug real-time streams interactively.

```bash
# Connect with a session cookie
reqx sio http://localhost:7879 -H "Cookie: auth={{token}}"

# Inside the REPL:
> listen NEW_MESSAGE
> emit send_chat {"text": "hello"}
> exit
```

### 4. `collection`: The CLI Editor
Modify your .json test suites without opening a text editor.

```bash
# See request indices
reqx collection list api.json

# Add a health check
reqx collection add api.json -n "Health" -u "{{base_url}}/health"

# Move request #10 to position #2
reqx collection move api.json 10 2
```

---

## 🧬 Advanced Usage & Samples

To see the full power of ReqX, generate the high-depth sample files:
```bash
reqx sample
```

### `sample-collection.json` Deep-Dive
This generated file demonstrates:
- **Variable Injection**: `{{$timestamp}}` for unique IDs and `{{api_token}}` for dynamic auth.
- **Auth Inheritance**: Collection-level auth applied to all requests unless overridden.
- **JavaScript Testing**:
  ```javascript
  pm.test("Status is 200", () => pm.response.to.have.status(200));
  pm.env.set("last_id", pm.response.json().id);
  ```
- **Async Sockets**: Start a background listener in your collection that stays alive while subsequent HTTP requests are fired.

---

## 🛠 Manual Installation (Windows)

1. Download the latest `reqx.exe` and `install.ps1` from the **Releases** page.
2. Open PowerShell as **Administrator**.
3. Run the installer:
   ```powershell
   .\install.ps1
   ```
4. **Restart your terminal** and type `reqx --help`.

---

## 🤝 Contributing
ReqX is built with a focus on **Consistency** and **Velocity**. If you're contributing, please follow the documentation patterns established in the `docs/` folder.

*Developed by Aryan Walia | 2026*