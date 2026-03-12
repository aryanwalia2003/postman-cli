# Terminal Usage Guide (`req` command)

Is chote se change ke baad, aapka tool ek powerful **curl alternative** ban jayega jo JSON environments ko natively support karta hai!

## 1. Simple GET Request
```bash
./postman-cli req https://jsonplaceholder.typicode.com/todos/1
```

## 2. POST Request with Headers & Body
```bash
./postman-cli req https://httpbin.org/post -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my-token" \
  -d '{"name": "Aryan"}'
```

## 3. Single Request with Environment Variables (The Best Feature!)
Agar aapke paas `env.json` mein `"base_url": "https://api.dev.mycompany.com"` set hai:

```bash
./postman-cli req "{{base_url}}/users/1" -X GET -e test-env.json
```