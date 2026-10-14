# Database Architecture & Migration Guide

This directory contains the PostgreSQL infrastructure, schema definitions, versioned migrations, and verification procedures for `go-rest-ws`, following the [PostgreSQL DBA skill](../.agents/skills/postgresql-dba/SKILL.md) and [AGENTS.md](../AGENTS.md).

---

## 1. Architecture & Role Separation

The database model separates administrative cluster setup, object ownership, schema migrations, and runtime application queries.

| Object / Role | Type | Attributes | Responsibilities & Privilege Boundaries |
| :--- | :--- | :--- | :--- |
| **`go_rest_ws`** | Database | UTF-8, libc | Isolated application database. Access revoked from `PUBLIC`. |
| **`app`** | Schema | Owned by `app_owner` | Dedicated schema outside `public`. `PUBLIC` schema creation is disabled. |
| **`app_owner`** | Role | `NOLOGIN`, `NOINHERIT` | Owns schema `app`, tables, and sequences. Cannot log in directly. |
| **`app_migrator`**| Role | `LOGIN`, `NOINHERIT` | Member of `app_owner`. Connects to apply migrations via `SET ROLE app_owner`. |
| **`appuser`** | Role | `LOGIN`, `NOINHERIT` | Least-privilege runtime role used by the Go API service. Privileges restricted strictly to `USAGE` on schema `app`, `SELECT` and `INSERT` on `app.users`, and sequence usage. Prohibited from DDL, `CREATE`, `UPDATE`, `DELETE`, `TRUNCATE`, or role escalation. |

---

## 2. Data Contract & Table Specification (`app.users`)

The `app.users` table implements the contract agreed for [Issue #4](https://github.com/manujkathuria/go-rest-ws/issues/4) and consumed by [Issue #3](https://github.com/manujkathuria/go-rest-ws/issues/3):

| Column | Type | Constraints & Defaults | Description |
| :--- | :--- | :--- | :--- |
| `id` | `BIGINT` | `GENERATED ALWAYS AS IDENTITY PRIMARY KEY` | Database-generated monotonically increasing unique identifier. |
| `name` | `TEXT` | `NOT NULL`, `CHECK (char_length(name) BETWEEN 1 AND 100)`, `CHECK (name = btrim(name))` | Trimmed display name; whitespace-only and untrimmed values are rejected. |
| `email` | `TEXT` | `NOT NULL`, `CHECK (char_length(email) BETWEEN 3 AND 254)`, `CHECK (email = lower(btrim(email)))`, `CHECK (email LIKE '%@%')`, `CONSTRAINT users_email_unique UNIQUE` | Normalized canonical email. Lowercase and trimmed; duplicate emails are rejected by the named constraint `users_email_unique`. |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT now()` | Database-generated timestamp recording row creation time. |

### Shared Normalization Policy (API & DB Agreement)
- **Name:** The API trims leading and trailing whitespace before insertion. The database enforces that `name` is identical to `btrim(name)` and has a length between 1 and 100 characters.
- **Email:** The API normalizes email addresses by trimming whitespace and lowercasing (`strings.ToLower(strings.TrimSpace(email))`). The database enforces canonical form with `email = lower(btrim(email))` and a length of 3 to 254 characters. Conflict errors on insertion map reliably to HTTP 409 Conflict via the named constraint `users_email_unique`.

---

## 3. Provisioning & Migration Instructions

### Prerequisites
- PostgreSQL 16+ running locally or accessible via network.
- Command-line client `psql`.

### Step 1: Cluster & Role Provisioning (Administrator / Superuser)
Run against your cluster (e.g. `postgres` maintenance database) using your administrative role:

```bash
# 1. Create database if it does not exist
psql -d postgres -c "CREATE DATABASE go_rest_ws;"

# 2. Provision roles and grant memberships
psql -d postgres -f db/scripts/provision_roles.sql

# 3. Configure database access, schema 'app', and default privileges
psql -d go_rest_ws -f db/scripts/init_database.sql
```

> **Note on Credentials:** `provision_roles.sql` assigns default development passwords (`app_migrator_dev_pw` and `appuser_dev_pw`). In production/staging environments, passwords must be supplied securely and kept out of version control.

### Step 2: Apply Migrations (as `app_migrator`)

```bash
PGPASSWORD=app_migrator_dev_pw psql -h localhost -p 5432 -U app_migrator -d go_rest_ws -f db/migrations/000001_create_users.up.sql
```

### Rollback (if needed)

```bash
PGPASSWORD=app_migrator_dev_pw psql -h localhost -p 5432 -U app_migrator -d go_rest_ws -f db/migrations/000001_create_users.down.sql
```

---

## 4. Verification

The automated verification suite in `db/scripts/verify.sh` runs real PostgreSQL checks against `go_rest_ws` strictly as the runtime role `appuser`:

```bash
./db/scripts/verify.sh
```

The script verifies:
1. **Schema placement:** `app.users` exists in schema `app` and not `public`.
2. **Positive path:** `appuser` can successfully `INSERT` returning identity and `created_at`, and `SELECT` users.
3. **Negative constraint enforcement:**
   - Empty, untrimmed, oversized (>100 chars), and NULL `name` values are rejected.
   - Empty, un-normalized (uppercase or untrimmed), undersized (<3 chars), format-invalid (missing `@`), and oversized (>254 chars) `email` values are rejected.
   - Duplicate normalized emails are rejected by named constraint `users_email_unique`.
4. **Least-privilege security boundaries:**
   - Prohibits DDL (`CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`).
   - Prohibits unauthorized DML (`UPDATE`, `DELETE`, `TRUNCATE`).
   - Prohibits role escalation (`SET ROLE app_owner` and `SET ROLE app_migrator` are denied).
