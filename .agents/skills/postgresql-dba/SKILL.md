---
name: postgresql-dba
description: Design and review PostgreSQL schemas, constraints, indexes, migrations, roles/grants, and query performance. Apply to database infrastructure or persistence changes; do not provision or alter live databases merely because a design is discussed.
---

# PostgreSQL DBA workflow

Provide a reviewable database design tied to the application's data contracts. Keep confirmed choices in project documentation; keep this skill reusable. Inspect the installed PostgreSQL version and use its official documentation for version-dependent behavior.

## Responsibilities and handoff

- The DBA role proposes schema, ownership, grants, constraints, indexes, migration steps, and database verification. Application developers own driver adapters, request cancellation, API error mapping, and pool lifecycle. Review the boundary together.
- Agree on identity, field limits, nullability, normalization, uniqueness, concurrent-update rules, and expected queries before writing schema migrations. Do not silently decide business semantics.
- A DBA task is a role for a scoped agent or engineer, not permission to start an agent or modify a database. Implement agreed migrations when requested; applying them needs authorization for the target environment.

## Schema and access control

- Use an explicit application schema rather than public for application objects. Qualify table references or use a controlled search_path containing only trusted schemas, with pg_catalog lookup considered explicitly.
- Separate object ownership/migration privileges from application runtime privileges. Runtime login must not be a superuser, database/schema owner, role administrator, or member able to assume an owner role.
- Grant runtime access only to its database, schema USAGE, and required table operations. For identity/sequence-generated values grant only needed sequence privileges. Do not grant schema CREATE, DDL, TRUNCATE, or unrestricted function execution as convenience defaults.
- Inspect PUBLIC grants, inherited role membership, default database privileges, schema permissions, and function privileges. A dedicated schema alone is not an authorization boundary.
- Revoke unwanted PUBLIC privileges in the dedicated application database only after checking existing consumers. Do not change an unrelated/shared cluster or drop its public schema as cleanup.
- Apply grants to existing objects and ALTER DEFAULT PRIVILEGES for the actual object-creating role, including when migrations use SET ROLE. Default privileges affect future objects and do not retrofit existing ones.
- Keep credentials out of migrations and logs. Document connection authentication and transport requirements for each environment; insecure local-only settings are not deployment defaults.

## Tables, migrations, and concurrency

- Use primary keys, required fields, unique/check/foreign-key constraints to enforce actual contracts. Name important constraints for stable error mapping; do not rely on application pre-checks for uniqueness under concurrency.
- Index demonstrated access paths. Avoid unnecessary indexes on write-heavy tables. Evaluate foreign-key lookup/deletion needs explicitly; PostgreSQL does not automatically index referencing columns.
- Keep transaction boundaries short and justify isolation/locking against the invariants. Inspect SQLSTATE and constraint identity when mapping errors; do not parse localized error messages.
- Use versioned migrations with an explicit application procedure. Separate database/role provisioning from table migrations, and keep runtime credentials out of the migration path.
- Review lock duration, rewrites, data backfills, and rollback/recovery implications for changes to populated tables. Not all DDL can run in a transaction; document exceptions.
- Parameterize values. Dynamic identifiers require validated allowlists and proper identifier quoting; placeholders cannot bind identifiers.

## Performance and service integration

- Choose connection limits from database capacity and total replicas, not user counts. Track pool waits, query latency, lock waits, and statement failures before tuning.
- Set bounded connection establishment, pool acquisition/query, lock, and transaction-idle behavior as appropriate. Context cancellation does not replace driver-specific connection timeouts.
- Use representative data and EXPLAIN plans; EXPLAIN ANALYZE executes statements, so assess side effects and use safe test fixtures. Do not infer capacity from an index or a single timing.
- Preserve consistency and durability when tuning. Batch only when operation semantics and latency permit it; consult [I/O batching](../go-io-batching/SKILL.md) when relevant.

## Verification and evidence

- Test migrations on an isolated real PostgreSQL database and perform application operations as the runtime role, not a superuser.
- Verify allowed reads/writes succeed and prohibited DDL, schema creation, owner-role assumption, and unauthorized object access fail. Check both existing-object grants and future-object defaults.
- Test duplicate/concurrent writes, rollback, cancellation, dependency failure, and restart persistence according to the application's contract.
- Report the PostgreSQL version, migration commands, role used, permission checks, and unverified boundaries. Substitute repositories/mocks cannot prove database constraints or grants.

References: [PostgreSQL schemas](https://www.postgresql.org/docs/current/ddl-schemas.html), [default privileges](https://www.postgresql.org/docs/current/sql-alterdefaultprivileges.html), [roles](https://www.postgresql.org/docs/current/user-manag.html). Use documentation for the actual target version.
