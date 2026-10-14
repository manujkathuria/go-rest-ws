---
name: go-unit-testing
description: Add or review meaningful Go unit tests for changed behavior, input/output contracts, ownership, and concurrent behavior. Use for Go implementation, bug fixes, or explicit test requests; exclude documentation-only edits and performance measurement alone.
---

# Go unit testing

Test observable contracts and failure behavior rather than mirroring implementation. Scale verification to the change; avoid tests for trivial edits that add no meaningful confidence.

## Design

- Define acceptance criteria before tests. For bug fixes, reproduce the defect and confirm failure before fixing it when practical.
- Prefer standard-library tests and table-driven cases with independent expected values. Do not compute expectations using the algorithm under test.
- Test isolated decoding and business logic without external services. Substitute narrow external boundaries where needed; avoid broad mock frameworks or redesigning production code solely for mocks.
- Isolate test fixtures and restore surrounding environment state: configuration tests must restore modified environment variables and clean up fixture artifacts so tests do not leak state or depend on ambient execution order.
- Distinguish unit tests, protocol/service integration tests, and end-to-end checks. Test doubles cannot establish external transaction semantics or durability.
- Use controlled identities and clocks where behavior depends on them. Synchronize explicitly instead of using arbitrary sleeps; bound blocking operations with deadlines.
- Register resource cleanup immediately upon creation: listener and network tests must guarantee cleanup even after failed assertions (such as with `t.Cleanup`), release blocked handlers, close response bodies and listeners, and bound server shutdowns with deadlines and force-close fallbacks to avoid orphaned goroutines.

## Contract cases

Select cases relevant to the change:

- Codec validity, truncation, versions, counts, lengths, trailing bytes, capacity boundaries, and rejection without publishing partially decoded events.
- Identity and idempotency across retries, distinct operations, restarts, and out-of-order requests or events, when these are part of the contract.
- Consecutive buffer reuse, retained references, asynchronous completion, cancellation, and shutdown. Output must remain correct after input storage is reused.
- Concurrent updates, duplicate operations, commit failures, rollback, recovery, and final partial batches where applicable. External-system behavior belongs in integration tests.

Fuzz parsers when useful, seeding valid and malformed input and checking bounded resource use, no panic, and contract invariants. Finite tests do not prove correctness under every possible failure.

## Verification

Run affected tests first and required project checks. Use race detection for concurrent or ownership changes. Broaden or repeat checks when new changes or failures justify it.

Report verified behavior and remaining integration gaps. Coverage identifies untested paths; it is not a correctness target. Never weaken assertions just to pass tests.

Read [go-data-ownership](../go-data-ownership/SKILL.md) for shared or reusable storage.
