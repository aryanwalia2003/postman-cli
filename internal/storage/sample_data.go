package storage

// SampleCollectionJSON is a well-commented template for users.
const SampleCollectionJSON = `{
  "name": "Sample Collection",
  "auth": {
    "type": "bearer",
    "token": "{{api_token}}"
  },
  "requests": [
    {
      "name": "Get User Profile",
      "method": "GET",
      "url": "{{base_url}}/get",
      "headers": {
        "Accept": "application/json"
      }
    },
    {
      "name": "Create Resource",
      "method": "POST",
      "url": "{{base_url}}/post",
      "auth": {
        "type": "apikey",
        "key": "X-API-Key",
        "value": "secret-123",
        "in": "header"
      },
      "body": "{\"name\": \"test object\"}"
    }
  ]
}`

// SampleEnvJSON is a template for environment variables.
const SampleEnvJSON = `{
  "name": "Development",
  "variables": {
    "base_url": "https://httpbin.org",
    "api_token": "your-bearer-token-here"
  }
}`
