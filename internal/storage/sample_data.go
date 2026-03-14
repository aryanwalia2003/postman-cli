package storage

// SampleCollectionJSON is a professional-grade, deep-dive template.
const SampleCollectionJSON = `{
  "name": "🚀 ReqX Deep-Dive Template",
  "description": "This collection showcases every core feature of ReqX: Auth, Variables, Scripting, and Socket.IO.",
  "auth": {
    "type": "bearer",
    "token": "{{api_token}}"
  },
  "requests": [
    {
      "name": "1. HTTP: Variables & Auth Inheritance",
      "method": "GET",
      "url": "{{base_url}}/get?type=test",
      "headers": {
        "Accept": "application/json",
        "X-Request-ID": "req-{{$timestamp}}"
      }
    },
    {
      "name": "2. HTTP: POST with Body & Specific Auth",
      "method": "POST",
      "url": "{{base_url}}/post",
      "auth": {
        "type": "basic",
        "username": "admin",
        "password": "{{admin_password}}"
      },
      "body": "{\"user_id\": 123, \"status\": \"active\", \"note\": \"{{custom_note}}\"}"
    },
    {
      "name": "3. Scripting: pm.env and pm.response",
      "method": "GET",
      "url": "https://httpbin.org/json",
      "pre_request_script": "pm.env.set('temp_id', 'dynamic-123'); console.log('Setting temp_id...');",
      "test_script": "pm.test('Status is 200', () => pm.response.to.have.status(200)); pm.test('Body has slideshow', () => pm.expect(pm.response.json().slideshow).to.exist);"
    },
    {
      "name": "4. Socket.IO: Background Listener (Async)",
      "protocol": "SOCKETIO",
      "async": true,
      "url": "http://localhost:7879",
      "headers": { "Cookie": "session={{session_id}}" },
      "events": [
        { "type": "listen", "name": "UPDATE_RECEIVED" }
      ]
    },
    {
      "name": "5. Socket.IO: Emitting Events (Sync)",
      "protocol": "SOCKETIO",
      "url": "http://localhost:7879",
      "events": [
        { "type": "emit", "name": "TRIGGER_UPDATE", "data": "{\"id\": 101}" }
      ]
    }
  ]
}`

// SampleEnvJSON is a detailed template for environment variables.
const SampleEnvJSON = `{
  "name": "Example Environment",
  "variables": {
    "base_url": "https://httpbin.org",
    "api_token": "your-secret-token",
    "admin_password": "super-secret-password",
    "custom_note": "Request sent via ReqX",
    "session_id": "sess-99b32"
  }
}`
