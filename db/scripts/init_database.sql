-- =============================================================================
-- Database & Schema Provisioning Script for go-rest-ws
-- Executed inside the target database: go_rest_ws
-- Executed by PostgreSQL superuser / cluster administrator
-- =============================================================================

\set ON_ERROR_STOP on

-- 1. Database Access Lockdown
REVOKE ALL ON DATABASE go_rest_ws FROM PUBLIC;
GRANT CONNECT ON DATABASE go_rest_ws TO app_migrator, appuser;

-- 2. Lockdown public schema
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- 3. Create dedicated application schema
CREATE SCHEMA IF NOT EXISTS app AUTHORIZATION app_owner;

-- 4. Schema privileges
GRANT USAGE ON SCHEMA app TO appuser;
GRANT USAGE, CREATE ON SCHEMA app TO app_owner;

-- 5. Default Privileges for future objects created by app_owner
-- Ensure appuser receives SELECT and INSERT on all future tables created by app_owner in schema app
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA app
    GRANT SELECT, INSERT ON TABLES TO appuser;

-- Ensure appuser receives USAGE and SELECT on all future sequences created by app_owner in schema app
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA app
    GRANT USAGE, SELECT ON SEQUENCES TO appuser;
