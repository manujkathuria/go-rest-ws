# Go pair-programming guide

This file routes reusable Go engineering guidance. Application scope, architecture, dependencies, and performance targets have not been defined here.

## Collaboration

Follow the [pair-programming agreement](.agents/standards/pair-programming.md). The user and lead reviewer develop requirements and architecture; the implementation agent completes and verifies agreed slices. Resolve routine details within the authorized scope and discuss material contract or architecture changes.

Distinguish proposed decisions, agreed decisions, and verified implementation. Preserve existing work. Do not commit or push unless requested. Maintain guidance and navigation as work evolves; discuss material new rules before adopting them.

## Standards and skills

Read the [standards index](.agents/standards/engineering-standards.md) by task trigger. Reuse unchanged guidance; the skills below are cumulative when their triggers apply, not a requirement to load every skill for every task.

| Work involving | Skill |
| --- | --- |
| Branches, staging, commits, PRs, milestone tags, CI/CD source versions | [Git workflow](.agents/skills/git-workflow/SKILL.md) |
| Typed records, slices, buffers, batches, codecs, ownership | [Data ownership](.agents/skills/go-data-ownership/SKILL.md) |
| Storage lifetimes, value/pointer choices, allocation diagnosis | [Stack and heap](.agents/skills/go-stack-vs-heap/SKILL.md) |
| Repeatedly stored struct layouts and memory footprint | [Struct alignment](.agents/skills/go-struct-alignment/SKILL.md) |
| Network/file loops, bulk writes, batch sizing | [I/O batching](.agents/skills/go-io-batching/SKILL.md) |
| Go behavior changes, bug fixes, meaningful tests | [Unit testing](.agents/skills/go-unit-testing/SKILL.md) |
| Performance claims, load tests, profiles, allocations | [Profiling and benchmarks](.agents/skills/go-profiling-benchmarks/SKILL.md) |

Use concrete types and explicit ownership. Select storage, concurrency controls, and resource limits from actual contracts and measurements. Examples illustrate decisions; they do not prescribe an application architecture. Add focused guidance when recurring needs justify it.

See [.agents/README.md](.agents/README.md) for navigation.
