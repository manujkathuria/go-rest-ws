---
name: go-rest-api
description: Design, implement, and review Go REST endpoints, OpenAPI contracts, interactive documentation, HTTP errors, request IDs, health/readiness, and request metrics. Apply when changing these API boundaries; exclude pure business logic and unrelated transport work.
---

# Go REST APIs

Make HTTP behavior predictable for clients and observable for operators. Apply the guidance relevant to the authorized slice; this skill does not require adding every capability, selecting a framework, or implementing deferred authentication.

## Contract and interactive documentation

- Define methods, paths, schemas, required and optional fields, validation, response statuses, and examples in OpenAPI. Document content types and distinguish omitted fields from explicit nulls where behavior differs.
- Keep the specification and handlers consistent. Choose the OpenAPI version, authoring approach, validator, and interactive documentation tooling through project decisions rather than prescribing a library here.
- Serve or provide interactive documentation with a usable specification URL and correct server/base-path configuration. Test that documented operations can be exercised. Keep deployment-specific docs exposure configurable when needed.
- Document pagination bounds, filtering, sorting, and stable ordering for collections. Specify retry/idempotency behavior for writes when relevant; a request ID is not an idempotency key.

## Input and error boundaries

- Bound bodies and collection limits; validate identifiers, query parameters, media types, and payloads before performing work. Define unknown-field, malformed-input, and trailing-data behavior deliberately.
- Use one documented error representation with a stable machine-readable code, useful message, request ID where applicable, and field-level validation details when helpful. Choose its exact schema in the project contract.
- Map errors to appropriate HTTP statuses consistently. Do not expose SQL, stack traces, internal addresses, credentials, or sensitive payloads in client errors.
- Preserve HTTP semantics for methods, response headers, content types, and statuses. Test not-found and unsupported-method behavior as well as application errors.

## Request IDs and logging

- Generate an ID when missing. Validate and bound accepted client IDs, or replace invalid values; treat caller-provided IDs as untrusted correlation data.
- Return the effective ID in a documented response header and make it available in contextual logs. Choose header naming and propagation policy explicitly.
- Request IDs correlate work; they do not establish identity, authorization, uniqueness, or safe retries. Do not use them as metric labels.

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
- Use bounded labels such as method, registered route pattern, and status class/code. Normalize unmatched routes. Avoid raw paths, query strings, user IDs, request IDs, and error messages as labels.
- Record failures, cancellations, and rejected work consistently and avoid double counting through middleware. Decide whether probes and documentation traffic are included.
- For latency claims record the workload, histogram resolution, and aggregation method. Do not average percentiles across replicas; consult [profiling and benchmarks](../go-profiling-benchmarks/SKILL.md) for performance measurement.

## Verification

Use [unit testing](../go-unit-testing/SKILL.md) and checks appropriate to the change:

- Exercise real HTTP handlers with representative success, invalid input, not-found/method errors, and dependency-failure cases.
- Validate the OpenAPI document and compare implemented request/response behavior with the contract using the selected tooling or meaningful contract tests.
- Verify effective request IDs, structured errors, bounded readiness failures, cancellation, and draining where changed.
- Check interactive documentation loads the specification and uses correct URLs; document anything not verified.
- Check metric label bounds and accounting for changed middleware; use race detection for shared state. Handler tests do not establish end-to-end capacity.

Report verified behavior and limitations. Keep authentication, deployment exposure, schemas, and library choices in project decisions, not hidden assumptions in this reusable skill.
