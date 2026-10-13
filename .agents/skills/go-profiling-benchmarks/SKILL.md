---
name: go-profiling-benchmarks
description: Measure and diagnose Go throughput, latency, allocations, CPU, and memory with benchmarks and profiles. Use for performance claims, hot-path optimization, load testing, or suspected bottlenecks; exclude routine correctness tests and documentation-only edits.
---

# Go profiling and benchmarks

Establish reproducible evidence before optimizing. Preserve delivery, ordering, ownership, and transaction contracts.

## Measurement boundaries

- Benchmark request handling, business logic, codecs, and batch construction separately when diagnosing them. State setup exclusions and report allocations with `-benchmem`.
- Distinguish offered work, accepted work, successful responses, and durable effects. Retries and duplicate submissions may increase traffic without increasing completed unique operations.
- Record toolchain, GOOS/GOARCH, hardware, topology, dataset and payload sizes, offered rate, concurrency, traffic mix, and relevant pool/batch/durability settings. Use isolated fixtures or run identities to exclude previous work.
- Confirm the generator supplies the workload. Client limits, connection pools, pacing, and backpressure can limit results independently of the service. Distinguish active users, connections, in-flight operations, and requests per second.

## Benchmarks and load tests

Use representative fixtures, observable results, and focused selectors. Replace the illustrative package path with the actual benchmark package:

```sh
go test ./path/to/package -run '^$' -bench . -benchmem -count 5
```

Compare baseline and candidate using the same workload/environment and repeated samples; use `benchstat` when available. Disclose variance rather than inventing universal thresholds. Do not use race-instrumented timings as normal throughput results.

Load tests need warmup, a bounded measurement interval, queue growth, errors, and drain/commit completion. Define latency endpoints before reporting percentiles and account for cross-host clocks. Distinguish short bursts from sustained capacity and explain backpressure in closed-loop workloads.

## Profiling

Choose evidence from the symptom:

- CPU profiles locate execution cost.
- Allocation profiles (`alloc_space` / `alloc_objects`) measure churn; in-use heap samples measure retained memory.
- Block/mutex profiles diagnose synchronization; traces help investigate scheduling and blocking. Bound higher-cost instrumentation to diagnostic runs.

```sh
go test ./path/to/package -run '^$' -bench . -benchmem -cpuprofile /tmp/go-cpu.pprof -memprofile /tmp/go-mem.pprof
go tool pprof -top /tmp/go-cpu.pprof
go tool pprof -sample_index=alloc_space -top /tmp/go-mem.pprof
```

Profile the actual service for network/database bottlenecks; codec profiles cannot establish pipeline behavior. Any diagnostic endpoint should be opt-in and localhost-only. Keep generated profiles and test binaries outside tracked source or explicitly ignore them.

## Decisions and reporting

Use profiles to justify changes, rerun comparable measurements, and verify correctness/races where relevant. Read the ownership, stack/heap, alignment, and I/O-batching skills when their triggers apply. Escape diagnostics support analysis but do not replace allocation measurements.

Report baseline/candidate results, workload, scope, variance, and correctness checks. Separate local measurements from multi-instance or distributed deployment claims. State inconclusive results honestly. Never disable GC or weaken validation, durability, or consistency guarantees to improve headline throughput.
