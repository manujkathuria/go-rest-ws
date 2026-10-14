# Go pair-programming guide

This file routes reusable Go engineering guidance. Application decisions are recorded below as they are agreed; planned work must not be presented as implemented behavior.

## Collaboration

Follow the [pair-programming agreement](.agents/standards/pair-programming.md). The user and lead reviewer develop requirements and architecture; the implementation agent completes and verifies agreed slices. Resolve routine details within the authorized scope and discuss material contract or architecture changes.

Distinguish proposed decisions, agreed decisions, and verified implementation. Preserve existing work. Do not commit or push unless requested. Maintain guidance and navigation as work evolves; discuss material new rules before adopting them.

## Standards and skills

Read the [standards index](.agents/standards/engineering-standards.md) by task trigger. Reuse unchanged guidance; the skills below are cumulative when their triggers apply, not a requirement to load every skill for every task.

| Work involving | Skill |
| --- | --- |
| PostgreSQL schema, roles/grants, migrations, indexes, database correctness/performance | [PostgreSQL DBA](.agents/skills/postgresql-dba/SKILL.md) |
| REST endpoints, OpenAPI/docs, HTTP errors, request IDs, probes, request metrics | [REST APIs](.agents/skills/go-rest-api/SKILL.md) |
| Branches, staging, commits, PRs, milestone tags, CI/CD source versions | [Git workflow](.agents/skills/git-workflow/SKILL.md) |
| Typed records, slices, buffers, batches, codecs, ownership | [Data ownership](.agents/skills/go-data-ownership/SKILL.md) |
| Storage lifetimes, value/pointer choices, allocation diagnosis | [Stack and heap](.agents/skills/go-stack-vs-heap/SKILL.md) |
| Repeatedly stored struct layouts and memory footprint | [Struct alignment](.agents/skills/go-struct-alignment/SKILL.md) |
| Network/file loops, bulk writes, batch sizing | [I/O batching](.agents/skills/go-io-batching/SKILL.md) |
| Go behavior changes, bug fixes, meaningful tests | [Unit testing](.agents/skills/go-unit-testing/SKILL.md) |
| Performance claims, load tests, profiles, allocations | [Profiling and benchmarks](.agents/skills/go-profiling-benchmarks/SKILL.md) |

Use concrete types and explicit ownership. Select storage, concurrency controls, and resource limits from actual contracts and measurements. Examples illustrate decisions; they do not prescribe an application architecture. Add focused guidance when recurring needs justify it.

See [.agents/README.md](.agents/README.md) for navigation.

## Development workflow

Follow the agreed [development workflow](.agents/standards/development-workflow.md) for issue structure, branches, optional draft PRs, verification, and merging. This standard is the confirmed baseline; merge strategy and release policy remain project decisions. Existing Git skill suggestions do not finalize those policies.

## User directory — agreed database direction

- Issue #3 covers PostgreSQL-backed user creation and retrieval.
- Use database/sql with github.com/lib/pq and explicit SQL. Pool controls belong to database/sql; query/index/transaction choices and workload measurements determine performance. Pin the driver version when implementation begins.
- Application objects belong in a dedicated non-public schema. The runtime login is appuser with only required data privileges; database ownership and migrations use separate privileges.
- A scoped DBA role owns the schema/grants/migration proposal and database verification, using the PostgreSQL DBA skill. We review its contract with the API implementation before applying migrations.
- Schema/database names, migration tooling, ID format, field limits, and email uniqueness/normalization remain to be agreed. No tables, roles, or database infrastructure have been provisioned by this guidance update.
