-- =============================================================================
-- Cluster Provisioning Script for go-rest-ws
-- Executed by PostgreSQL superuser / cluster administrator
-- Target: local PostgreSQL 16+ instance
--
-- Usage:
--   psql -d postgres -v ON_ERROR_STOP=1 \
--        -v app_migrator_password="<secret>" \
--        -v appuser_password="<secret>" \
--        -f db/scripts/provision_roles.sql
-- =============================================================================

\set ON_ERROR_STOP on

-- 1. Validate required password variables are supplied externally
\if :{?app_migrator_password}
\else
    DO $$ BEGIN RAISE EXCEPTION 'app_migrator_password variable is required. Pass via psql -v app_migrator_password="<secret>"'; END $$;
\endif

\if :{?appuser_password}
\else
    DO $$ BEGIN RAISE EXCEPTION 'appuser_password variable is required. Pass via psql -v appuser_password="<secret>"'; END $$;
\endif

-- 2. Create Roles (if not existing)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_owner') THEN
        CREATE ROLE app_owner WITH NOLOGIN;
    END IF;

    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_migrator') THEN
        CREATE ROLE app_migrator WITH LOGIN;
    END IF;

    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'appuser') THEN
        CREATE ROLE appuser WITH LOGIN;
    END IF;
END
$$;

-- 3. Enforce strict role attributes (re-asserting attributes even for existing roles)
ALTER ROLE app_owner WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
ALTER ROLE app_migrator WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD :'app_migrator_password';
ALTER ROLE appuser WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD :'appuser_password';

-- 4. Set search_path
ALTER ROLE appuser SET search_path TO app, public;
ALTER ROLE app_migrator SET search_path TO app, public;
ALTER ROLE app_owner SET search_path TO app, public;

-- 5. Sanitize and establish role memberships
-- app_migrator assumes app_owner via SET ROLE, but does not inherit privileges automatically
GRANT app_owner TO app_migrator;

-- Ensure appuser has no elevated memberships
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_auth_members m
        JOIN pg_roles r ON m.member = r.oid
        JOIN pg_roles g ON m.roleid = g.oid
        WHERE r.rolname = 'appuser' AND g.rolname = 'app_owner'
    ) THEN
        REVOKE app_owner FROM appuser;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_auth_members m
        JOIN pg_roles r ON m.member = r.oid
        JOIN pg_roles g ON m.roleid = g.oid
        WHERE r.rolname = 'appuser' AND g.rolname = 'app_migrator'
    ) THEN
        REVOKE app_migrator FROM appuser;
    END IF;
END
$$;

-- 6. Audit & validate role attributes and memberships
DO $$
DECLARE
    r RECORD;
    v_has_illegal_member BOOLEAN;
BEGIN
    -- Check attributes for all three roles
    FOR r IN (
        SELECT rolname, rolsuper, rolinherit, rolcreaterole, rolcreatedb, rolcanlogin, rolreplication, rolbypassrls
        FROM pg_roles
        WHERE rolname IN ('app_owner', 'app_migrator', 'appuser')
    ) LOOP
        IF r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls THEN
            RAISE EXCEPTION 'Role % has illegal elevated administrative privileges (superuser/createrole/createdb/etc.)', r.rolname;
        END IF;

        IF r.rolname = 'app_owner' AND r.rolcanlogin THEN
            RAISE EXCEPTION 'app_owner must have NOLOGIN attribute';
        END IF;

        IF r.rolname IN ('app_migrator', 'appuser') THEN
            IF NOT r.rolcanlogin THEN
                RAISE EXCEPTION '% must have LOGIN attribute', r.rolname;
            END IF;
            IF r.rolinherit THEN
                RAISE EXCEPTION '% must have NOINHERIT attribute', r.rolname;
            END IF;
        END IF;
    END LOOP;

    -- Ensure appuser has no membership in app_owner or superuser roles
    SELECT EXISTS (
        SELECT 1 FROM pg_auth_members m
        JOIN pg_roles member_role ON m.member = member_role.oid
        JOIN pg_roles granted_role ON m.roleid = granted_role.oid
        WHERE member_role.rolname = 'appuser'
          AND (granted_role.rolname = 'app_owner' OR granted_role.rolsuper)
    ) INTO v_has_illegal_member;

    IF v_has_illegal_member THEN
        RAISE EXCEPTION 'appuser must not have membership in app_owner or superuser roles';
    END IF;
END
$$;
