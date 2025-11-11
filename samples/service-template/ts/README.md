# TypeScript Sample Service Template

This Node.js service mirrors the Go sample and will eventually integrate the shared TypeScript libraries (configuration, observability, data access, errors, auth).

## Commands

- `npm install` – install dependencies
- `npm run dev` – start the service in development mode
- `npm run build` / `npm start` – build and run the compiled output
- `npm test` – execute Vitest suites (placeholder)

## Endpoints

- `GET /healthz` – JSON health response (`{ "status": "ok" }`)

Configuration is controlled via environment variables `SERVICE_HOST` and `SERVICE_PORT`.

