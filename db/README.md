# Database Architecture & Migration Guide

This directory contains the PostgreSQL infrastructure, schema definitions, versioned migrations, and verification procedures for `go-rest-ws`, following the [PostgreSQL DBA skill](../.agents/skills/postgresql-dba/SKILL.md) and [AGENTS.md](../AGENTS.md).

---

## 1. Architecture & Role Separation

The database model separates administrative cluster setup, object ownership, schema migrations, and runtime application queries.

| Object / Role | Type | Attributes | Responsibilities & Privilege Boundaries |
| :--- | :--- | :--- | :--- |
| **`go_rest_ws`** | Database | UTF-8, libc | Dedicated application database. Access revoked from `PUBLIC`. |
| **`app`** | Schema | Owned by `app_owner` | Dedicated schema outside `public`. `PUBLIC` schema creation is disabled. |
| **`app_owner`** | Role | `NOLOGIN`, `NOINHERIT`, unprivileged admin | Owns schema `app`, tables, and sequences. Cannot log in directly. |
| **`app_migrator`**| Role | `LOGIN`, `NOINHERIT`, unprivileged admin | Member of `app_owner`. Connects to apply migrations via `SET ROLE app_owner`. |
| **`appuser`** | Role | `LOGIN`, `NOINHERIT`, unprivileged admin | Least-privilege runtime role used by the Go API service. Privileges restricted strictly to `USAGE` on schema `app`, `SELECT` and `INSERT` on `app.users`, and sequence usage. Prohibited from DDL, `CREATE`, `UPDATE`, `DELETE`, `TRUNCATE`, or role escalation. |

---

## 2. Data Contract & Table Specification (`app.users`)

The `app.users` table implements the contract agreed for [Issue #4](https://github.com/manujkathuria/go-rest-ws/issues/4) and consumed by [Issue #3](https://github.com/manujkathuria/go-rest-ws/issues/3):

| Column | Type | Constraints & Defaults | Description |
| :--- | :--- | :--- | :--- |
| `id` | `BIGINT` | `GENERATED ALWAYS AS IDENTITY PRIMARY KEY` | Database-generated monotonically increasing unique identifier. |
| `name` | `TEXT` | `NOT NULL`, `CHECK (char_length(name) BETWEEN 1 AND 100)`, `CHECK (name = btrim(name, E' \t\n\r\v\f') AND name !~ '^[[:space:]]' AND name !~ '[[:space:]]$')` | Trimmed display name; whitespace-only, tabs, newlines, and untrimmed values are rejected, matching Go's `strings.TrimSpace`. |
| `email` | `TEXT` | `NOT NULL`, `CHECK (char_length(email) BETWEEN 3 AND 254)`, `CHECK (email = lower(btrim(email, E' \t\n\r\v\f')) AND email !~ '^[[:space:]]' AND email !~ '[[:space:]]$')`, `CHECK (email LIKE '%@%')`, `CONSTRAINT users_email_unique UNIQUE` | Normalized canonical email. Lowercase, trimmed of tabs/spaces/newlines; duplicate emails are rejected by the named constraint `users_email_unique`. |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT now()` | Database-generated timestamp recording row creation time. |

### Shared Normalization Policy (API & DB Agreement)
- **Name:** The API trims surrounding whitespace using Go's `strings.TrimSpace(name)`. The database enforces that `name` is identical to `btrim(name, E' \t\n\r\v\f')`, does not start or end with POSIX whitespace (`!~ '^[[:space:]]'`, `!~ '[[:space:]]$'`), and has a length between 1 and 100 characters.
- **Email:** The API normalizes email addresses by trimming whitespace and lowercasing (`strings.ToLower(strings.TrimSpace(email))`). The database enforces canonical lowercase form without whitespace, length of 3 to 254 characters, and presence of `@`. Conflict errors on insertion map reliably to HTTP 409 Conflict via the named constraint `users_email_unique`.

---

## 3. Provisioning & Migration Instructions

### Prerequisites
- PostgreSQL 16+ running locally or accessible via network.
- Command-line client `psql`.

### Step 1: Cluster & Role Provisioning (Administrator / Superuser)
Credentials are never committed to version control; they must be supplied externally via `psql -v` parameters or environment variables.

```bash
# 1. Create database if it does not exist
psql -d postgres -c "CREATE DATABASE go_rest_ws;"

# 2. Provision and sanitize roles (fails if passwords are not passed)
psql -d postgres \
     -v ON_ERROR_STOP=1 \
     -v app_migrator_password="<secret>" \
     -v appuser_password="<secret>" \
     -f db/scripts/provision_roles.sql

# 3. Configure database access, schema 'app', and default privileges
psql -d go_rest_ws -f db/scripts/init_database.sql
```

> **Security & Cluster Re-use:** `provision_roles.sql` enforces baseline attributes (`NOSUPERUSER`, `NOCREATEDB`, `NOCREATEROLE`, `NOINHERIT`, `NOREPLICATION`, `NOBYPASSRLS`), revokes unauthorized memberships (ensuring `appuser` never inherits `app_owner` or superuser), and validates all role boundaries before exiting.

### Step 2: Apply Migrations (as `app_migrator`)

```bash
PGPASSWORD="<migrator_secret>" psql -h localhost -p 5432 -U app_migrator -d go_rest_ws -f db/migrations/000001_create_users.up.sql
```

### Rollback (if needed)

```bash
PGPASSWORD="<migrator_secret>" psql -h localhost -p 5432 -U app_migrator -d go_rest_ws -f db/migrations/000001_create_users.down.sql
```

---

## 4. Verification

The automated verification suite in `db/scripts/verify.sh` tests real PostgreSQL behavior against `go_rest_ws`:

```bash
APPUSER_PASSWORD="<appuser_secret>" APP_MIGRATOR_PASSWORD="<migrator_secret>" ./db/scripts/verify.sh
```

The script verifies:
1. **Schema placement & absence from public:** Proves `public.users` does not exist (count = 0) and `app.users` exists in schema `app` (count = 1).
2. **Positive path & value assertion (isolated transaction):** Executes an insert within a transaction, captures returned values, asserts `id > 0`, non-null `created_at`, exact field match, and rolls back cleanly with zero orphan rows left behind.
3. **Negative constraint enforcement:**
   - Empty, tab-only, space-only, leading tab, trailing newline, oversized (>100 chars), and NULL `name` values are rejected.
   - Empty, un-normalized (uppercase, leading tab, trailing space), undersized (<3 chars), format-invalid (missing `@`), and oversized (>254 chars) `email` values are rejected.
   - Duplicate normalized emails are rejected by named constraint `users_email_unique`.
4. **Race conditions & concurrency:** Spawns concurrent insert jobs with identical emails; asserts exactly one succeeds and one is rejected by `users_email_unique`.
5. **Future-object default privileges:** Proves that future tables created by `app_owner` grant `SELECT` and `INSERT` to `appuser` without manual grants, while withholding `UPDATE`, `DELETE`, and `DROP`.
6. **Least-privilege security boundaries:**
   - Prohibits schema creation (`CREATE SCHEMA`).
   - Prohibits DDL in `public` and `app` (`CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`).
   - Prohibits unauthorized DML (`UPDATE`, `DELETE`, `TRUNCATE`).
   - Prohibits role escalation (`SET ROLE app_owner` and `SET ROLE app_migrator` fail with permission denied).
