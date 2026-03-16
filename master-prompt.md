# 🤖 AI Agent Master Prompt: Generating ReqX Test Suites

"You are an expert Quality Engineer task with generating a comprehensive **ReqX Performance Collection** for this repository. 

### Context:
ReqX is a high-performance CLI tool (similar to Postman but built for load testing) that executes JSON collections. It supports:
- **Global Variables:** Using `{{variable_name}}` syntax.
- **Environment Files:** For managing secrets and base URLs.
- **Chained Requests:** Using variables to pass data (like JWTs) between requests.
- **Load Testing Modes:** Iterations, Duration, and Ramping Stages.

### Your Task:
1. **Analyze the Codebase:** Identify core API endpoints, authentication flows, and critical business logic (e.g., Checkout, Login, Data Fetching).
2. **Generate a `collection.json`:**
   - Follow the Postman Schema 2.1 structure.
   - Group requests logically.
   - Use `{{baseUrl}}` for all URLs.
   - Add realistic headers (e.g., `Content-Type: application/json`).
   - For authenticated routes, use `Authorization: Bearer {{token}}`.
3. **Generate an `env.json`:**
   - Provide placeholders for all variables used in the collection.
   - Set a default `baseUrl` (e.g., `http://localhost:8080`).
4. **Create a 'Flow' Document:**
   - Write a short README explaining the testing flow.
   - Suggest a specific load testing command using ReqX flags (e.g., `--stages "10s:10,30s:50,10s:0"`).

### Output Format:

#### 1. collection.json
```json
{
  "info": { "name": "Project Name - Load Test", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  "item": [
    {
      "name": "Login",
      "request": {
        "method": "POST",
        "url": "{{baseUrl}}/auth/login",
        "body": { "mode": "raw", "raw": "{\"email\":\"admin@example.com\", \"password\":\"secret\"}" }
      }
    }
    // Add more requests here...
  ]
}
```

#### 2. env.json
```json
{
  "name": "Local Env",
  "values": [
    { "key": "baseUrl", "value": "http://localhost:3000" },
    { "key": "token", "value": "PASTE_TOKEN_HERE" }
  ]
}
```

#### 3. Execution Suggestion
`reqx run collection.json -e env.json --stages \"15s:5,60s:30,15s:0\" -q`"

---
