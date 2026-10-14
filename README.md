# go-rest-ws

A high-performance, concurrent Go backend service engineered for stateless horizontal scaling, PostgreSQL correctness, and bounded resource utilization. Adopts a REST-first roadmap followed by a WebSockets real-time extension.

---

## Service Foundation (Current State)

The service foundation provides operational endpoints, observability, and contract documentation before business logic:

| Endpoint | Method | Purpose |
| :--- | :--- | :--- |
| `/healthz` | `GET` | Process liveness probe (`200 OK`) |
| `/readyz` | `GET` | Service readiness probe (`200 OK` when ready; `503 Service Unavailable` while draining) |
| `/metrics` | `GET` | Prometheus metrics exposition (`http_requests_total`, `http_request_duration_seconds`, `http_requests_in_flight`) |
| `/openapi.yaml` | `GET` | Embedded OpenAPI 3.1 specification contract |
| `/docs` | `GET` | Interactive API documentation (Scalar API Reference UI) |

---

## Quickstart

### Prerequisites
- Go 1.22+ installed (`go version`)

### Running Locally
```bash
# Copy example configuration (optional)
cp .env.example .env

# Run the API service
go run ./cmd/api
```

By default, the server binds to `0.0.0.0:8080`.

### Verifying Endpoints
```bash
# Liveness probe
curl -i http://localhost:8080/healthz

# Readiness probe
curl -i http://localhost:8080/readyz

# Prometheus metrics
curl http://localhost:8080/metrics

# Raw OpenAPI spec
curl http://localhost:8080/openapi.yaml

# Interactive docs in browser
open http://localhost:8080/docs
```

---

## Configuration

Configuration is loaded with precedence: **environment variables > optional `.env` > safe defaults**.

| Variable | Default | Description |
| :--- | :--- | :--- |
| `HTTP_HOST` | `0.0.0.0` | Listen host interface |
| `HTTP_PORT` | `8080` | Listen port (1–65535) |
| `HTTP_READ_TIMEOUT` | `5s` | Maximum duration for reading the entire request |
| `HTTP_WRITE_TIMEOUT` | `10s` | Maximum duration before timing out writes of the response |
| `HTTP_IDLE_TIMEOUT` | `120s` | Maximum amount of time to wait for the next request on keep-alive |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | Maximum duration allowed for in-flight requests during graceful draining |
| `LOG_FORMAT` | `json` | Structured log output format (`json` or `text`) |
| `LOG_LEVEL` | `info` | Minimum log severity level (`debug`, `info`, `warn`, `error`) |

---

## Verification & Checks

```bash
# Run all tests with Go race detector
go test -count=1 -race ./...

# Static analysis
go vet ./...

# Format verification
gofmt -s -l .

# Compile binary
go build -o bin/api ./cmd/api
```

---

## Guidance & Standards

- [AGENTS.md](AGENTS.md): Collaboration agreements and task routing.
- [Standards](.agents/standards/engineering-standards.md): Reusable Go engineering baseline and invariants.
- [Pair Programming](.agents/standards/pair-programming.md): Collaborative delivery cadence and review expectations.
- [Skills](.agents/skills/README.md): Focused implementation and verification playbooks.
