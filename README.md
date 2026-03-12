# Postman CLI - Usage Guide

A lightweight, powerful CLI for executing API collections, similar to Newman but built in Go for speed and flexibility.

## Implemented Features

### 1. Request Execution
- **HTTP Requests:** Full support for `GET`, `POST`, `PUT`, `DELETE`, etc.
- **Protocols:** Support for standard HTTP/HTTPS and **Socket.IO**.
- **Body Support:** Raw JSON body handling with variable replacement.

### 2. Authentication System
Supports authentication at both the **Collection** and **Request** levels.
- **Bearer Token:** `Authorization: Bearer <token>`
- **Basic Auth:** `Authorization: Basic <base64(user:pass)>`
- **API Key:** Custom header or query parameter.
- **Cookie Auth:** Pre-defined cookie injection.
- **Inheritance:** Requests automatically use collection-level auth unless overridden.

### 3. Managed Cookie Jar
- **Persistence:** Cookies from responses are automatically stored and sent in subsequent requests.
- **CLI Control:**
    - `--no-cookies`: Disable cookie persistence entirely.
    - `--clear-cookies`: Wipe the cookie jar before *every* request.

### 4. Logic & Variable Replacement
- **Environment Variables:** Use `{{variable_name}}` in URLs, headers, bodies, and auth fields.
- **Dynamic Context:** Variables are resolved at runtime from environment files.

### 5. Scripting Engine (Engine: Goja)
- **Pre-request Scripts:** Run JavaScript code before a request is sent.
- **Test Scripts:** Run JavaScript code after response arrival.
- **API Access:** Access response data via `pm.response`.

---

## JSON Structures

### Collection File (`collection.json`)
```json
{
  "name": "Production Suite",
  "auth": {
    "type": "bearer",
    "token": "{{api_token}}"
  },
  "requests": [
    {
      "name": "Get Profile",
      "method": "GET",
      "url": "{{base_url}}/user/profile"
    },
    {
      "name": "Refresh Token",
      "method": "POST",
      "url": "{{base_url}}/auth/refresh",
      "auth": { "type": "none" }
    }
  ]
}
```

### Environment File (`env.json`)
```json
{
  "name": "Staging",
  "variables": {
    "base_url": "https://api.staging.example.com",
    "api_token": "secret-123"
  }
}
```

---

## How to Use

### Build the CLI
```bash
go build -o postman-cli main.go
```

### Run a Collection
```bash
./postman-cli run test-collection.json
```

### Run with Environment Variables
```bash
./postman-cli run test-collection.json -e test-env.json
```

### Cookie Management
```bash
# Disable cookies entirely
./postman-cli run test-collection.json --no-cookies

# New session for every request
./postman-cli run test-collection.json --clear-cookies
```
