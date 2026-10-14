-- =============================================================================
-- Cluster Provisioning Script for go-rest-ws
-- Executed by PostgreSQL superuser / cluster administrator
-- Target: local PostgreSQL 16+ instance
-- =============================================================================

\set ON_ERROR_STOP on

-- 1. Create Roles (if not existing)
DO $$
BEGIN
    -- Owner role: owns application schema and tables; NOLOGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_owner') THEN
        CREATE ROLE app_owner WITH NOLOGIN NOINHERIT;
    END IF;

    -- Migration role: applies migrations using owner privileges; LOGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_migrator') THEN
        CREATE ROLE app_migrator WITH LOGIN NOINHERIT;
    END IF;

    -- Runtime role: least-privilege application access; LOGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'appuser') THEN
        CREATE ROLE appuser WITH LOGIN NOINHERIT;
    END IF;
END
$$;

-- Allow app_migrator to assume app_owner role via SET ROLE app_owner
GRANT app_owner TO app_migrator;

-- Set default development passwords if not already set (can be overridden externally)
-- In production/CI, set passwords securely via ALTER ROLE ... WITH PASSWORD '...'
ALTER ROLE app_migrator WITH PASSWORD 'app_migrator_dev_pw';
ALTER ROLE appuser WITH PASSWORD 'appuser_dev_pw';

-- Set default search_path for appuser and app_migrator to app, public
ALTER ROLE appuser SET search_path TO app, public;
ALTER ROLE app_migrator SET search_path TO app, public;
ALTER ROLE app_owner SET search_path TO app, public;
