---
name: go-service
description: Build and operate a Go HTTP service with structured logging, retries, and tests.
---

# Go Service

Use this skill when adding endpoints, changing configuration, or debugging a Go
service in this repository.

## Quick Start

Create the handler, wire it into the router, and run the service locally:

```bash
go run ./cmd/server --config ./config/local.yaml
curl -s localhost:8080/healthz
```

Every handler returns a typed response struct. Do not write to the response
writer directly outside the shared `respond` helper, because it sets the
content type, request ID, and error envelope.

## Configuration

Configuration is layered: defaults, then the YAML file, then environment
variables, then flags. Add a new setting to the struct, give it a default, and
document the environment name in the table below. Never read `os.Getenv`
outside `internal/config`, because tests replace the loader.

The loader rejects unknown keys. When you add a field, update the example
config, the README table, and the config test in the same change. A setting
that only exists in code is a bug.

## Error Handling

Wrap errors with context using `fmt.Errorf("doing x: %w", err)`. Map them to
HTTP status codes in one place. Client errors use 400, missing resources use
404, conflicts use 409, and everything unexpected uses 500 with a generated
request ID in the log line.

Never return the raw database error to the caller. Log it with the request ID,
return a stable message, and keep the detail in the log only.

## Retries

Retry only idempotent calls. Use the shared backoff helper, cap attempts at
three, and add jitter. Do not retry validation failures, authentication
failures, or any 4xx response other than 429. Log each retry with the attempt
number and the reason.

## Testing

Table-driven tests live next to the code. Use `httptest` for handlers and a
real temporary directory for anything touching disk. Every new branch needs a
case. Run `go test -race ./...` before you push and treat a flaky test as a
defect, not a nuisance.

## Deployment

Builds are static and cross-compiled. The release pipeline runs tests, builds
the binary, and publishes an archive per platform. Roll out to staging first,
watch error rate and latency for ten minutes, then promote. A rollback is one
command and must be rehearsed before the first production deploy.

## Troubleshooting

Start with the request ID. Grep the logs for it to see the full path. If the
service is healthy but slow, check the database pool before touching the
handler. If the service refuses to start, read the configuration error first;
it names the exact key and source that failed validation.
