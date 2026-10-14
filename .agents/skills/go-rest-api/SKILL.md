---
name: go-rest-api
description: Design, implement, and review Go REST endpoints, OpenAPI contracts, interactive documentation, HTTP errors, request IDs, health/readiness, and request metrics. Apply when changing these API boundaries; exclude pure business logic and unrelated transport work.
---

# Go REST APIs

Make HTTP behavior predictable for clients and observable for operators. Apply the guidance relevant to the authorized slice; this skill does not require adding every capability, selecting a framework, or implementing deferred authentication.

## Contract and interactive documentation

- Define methods, paths, schemas, required and optional fields, validation, response statuses, and examples in OpenAPI. Document content types and distinguish omitted fields from explicit nulls where behavior differs.
- Keep the specification and handlers consistent. Choose the OpenAPI version, authoring approach, validator, and interactive documentation tooling through project decisions rather than prescribing a library here.
- Serve or provide interactive documentation with a usable specification URL and correct server/base-path configuration. Document and verify interactive operations appropriately without conflating documentation serving with complete contract verification.
- Document pagination bounds, filtering, sorting, and stable ordering for collections. Specify retry/idempotency behavior for writes when relevant; a request ID is not an idempotency key.

## Input, recovery, and error boundaries

- Bound bodies and collection limits; validate identifiers, query parameters, media types, and payloads before performing work. Define unknown-field, malformed-input, and trailing-data behavior deliberately.
- Use one documented error representation with a stable machine-readable code, useful message, request ID where applicable, and field-level validation details when helpful. Choose its exact schema in the project contract.
- Map errors to appropriate HTTP statuses consistently. Do not expose SQL, stack traces, internal addresses, credentials, or sensitive payloads in client errors.
- Preserve HTTP semantics for methods, response headers, content types, and statuses. Test not-found and unsupported-method behavior as well as application errors.
- Enforce response commitment rules in recovery: before response headers or status are committed, recovery can return an unhandled error response (such as JSON `500`); after commitment, abort the response/stream rather than appending trailing error data or returning a misleading success status.
- Preserve response-writer capabilities: wrappers that capture status or track byte counts must preserve caller-required capabilities such as flushing (e.g. `http.Flusher` and response controller operations), hijacking, or push, while maintaining accurate commitment and status tracking.

## Request IDs and middleware ordering

- Generate an ID when missing. Validate and bound accepted client IDs, or replace invalid values; treat caller-provided IDs as untrusted correlation data.
- Return the effective ID in a documented response header and make it available in contextual logs. Choose header naming and propagation policy explicitly.
- Request IDs correlate work; they do not establish identity, authorization, uniqueness, or safe retries. Do not use them as metric labels.
- Order middleware so correlation context (such as request IDs) reaches recovery handlers, and unhandled or recovered failures are reflected accurately in operational logs and metrics. Test the actual assembled stack rather than isolated middleware functions alone.

## Health and readiness

- Liveness reports whether the process is alive; avoid failing it merely because an external dependency is temporarily unavailable.
- Readiness reports whether the instance can serve its intended traffic. Define required dependencies, initialization state, and draining behavior according to the service contract.
- Keep dependency probes cheap, bounded, and cancelable. Account for probe frequency; avoid expensive checks on every probe. Document success/failure statuses without exposing sensitive diagnostics.

## Configuration and graceful shutdown

Follow [service foundations](../../standards/01-service-foundations.md) for typed settings, structured logging, contexts, and lifecycle ownership.

- Configure appropriate HTTP read/header/write/idle limits, body limits, operation deadlines, and shutdown deadlines. Values follow the endpoint contract; streamed responses may need different limits.
- Propagate request cancellation into dependency calls. Bound expensive work, queued work, and resource pools; select overload behavior deliberately instead of assuming goroutines make capacity unlimited.
- Coordinate readiness and intake when draining, allow in-flight work a bounded completion period, and close owned dependencies afterward. Test cancellation and shutdown. Upgraded connections need their own lifecycle handling if present.

## Basic metrics

- Measure request counts, duration distributions, errors, and in-flight requests with explicit units and timing boundaries. Select the metrics library/export format through project decisions.
- Bound metric cardinality: bound HTTP methods (normalizing unrecognized or arbitrary client verbs to a fixed label) as well as route labels. Avoid raw paths, query strings, user IDs, request IDs, and unbounded error strings as labels.
- Record failures, cancellations, and rejected work consistently and avoid double counting through middleware. Decide whether probes and documentation traffic are included.
- For latency claims record the workload, histogram resolution, and aggregation method. Do not average percentiles across replicas; consult [profiling and benchmarks](../go-profiling-benchmarks/SKILL.md) for performance measurement.

## Verification

Use [unit testing](../go-unit-testing/SKILL.md) and checks appropriate to the change:

- Exercise real HTTP handlers with representative success, invalid input, not-found/method errors, and dependency-failure cases.
- Scope documentation and contract verification deliberately: distinguish serving raw documentation files, checking YAML/JSON syntax and structure, validating against the full OpenAPI specification schema, and exercising interactive requests.
- Verify middleware ordering and recovery against the actual assembled stack: verify that request IDs reach recovery handlers, recovered errors produce correct completion logs and metric entries, and post-commitment failures abort cleanly rather than returning deceptive success codes.
- Verify effective request IDs, structured errors, bounded readiness failures, cancellation, and draining where changed.
- Check interactive documentation loads the specification and uses correct URLs; document anything not verified.
- Check metric label bounds and accounting for changed middleware; use race detection for shared state. Handler tests do not establish end-to-end capacity.

Report verified behavior and limitations. Keep authentication, deployment exposure, schemas, and library choices in project decisions, not hidden assumptions in this reusable skill.
