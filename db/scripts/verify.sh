#!/usr/bin/env bash
# =============================================================================
# Database Verification Script for go-rest-ws (Issue #4)
# Verifies data contracts, constraints, and least-privilege boundaries as appuser
# =============================================================================

set -euo pipefail

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-go_rest_ws}"
APPUSER="${APPUSER:-appuser}"
APPUSER_PASSWORD="${APPUSER_PASSWORD:-appuser_dev_pw}"

export PGPASSWORD="$APPUSER_PASSWORD"

PSQL="psql -h $DB_HOST -p $DB_PORT -U $APPUSER -d $DB_NAME -v ON_ERROR_STOP=1 -X -q"

echo "================================================================="
echo "Starting Database Verification for $DB_NAME"
echo "Target: $DB_HOST:$DB_PORT | Database: $DB_NAME | Role: $APPUSER"
echo "PostgreSQL Server Version: $($PSQL -t -A -c 'SELECT version();')"
echo "================================================================="

PASSED_COUNT=0
FAILED_COUNT=0

assert_success() {
    local desc="$1"
    local sql="$2"
    echo -n "  [TEST] $desc ... "
    if $PSQL -c "$sql" > /dev/null 2>&1; then
        echo "PASS"
        PASSED_COUNT=$((PASSED_COUNT + 1))
    else
        echo "FAIL (Expected success, but query failed)"
        FAILED_COUNT=$((FAILED_COUNT + 1))
    fi
}

assert_failure() {
    local desc="$1"
    local sql="$2"
    local expected_err="${3:-}"
    echo -n "  [TEST] $desc ... "
    local output
    if output=$($PSQL -c "$sql" 2>&1); then
        echo "FAIL (Expected query to fail, but it succeeded)"
        FAILED_COUNT=$((FAILED_COUNT + 1))
    else
        if [[ -n "$expected_err" ]] && [[ "$output" != *"$expected_err"* ]]; then
            echo "FAIL (Failed as expected, but missing expected keyword '$expected_err')"
            echo "         Actual output: $output"
            FAILED_COUNT=$((FAILED_COUNT + 1))
        else
            echo "PASS (Rejected as expected)"
            PASSED_COUNT=$((PASSED_COUNT + 1))
        fi
    fi
}

echo ""
echo "--- 1. Schema Placement & Identity ---"
assert_success "Table exists in 'app' schema and not 'public'" \
    "SELECT 1 FROM information_schema.tables WHERE table_schema = 'app' AND table_name = 'users';"

echo ""
echo "--- 2. Positive Operations (INSERT & SELECT as appuser) ---"
assert_success "Insert valid user returning id and created_at" \
    "INSERT INTO app.users (name, email) VALUES ('Ada Lovelace', 'ada_' || floor(extract(epoch from clock_timestamp())) || '@example.com') RETURNING id, created_at;"

assert_success "SELECT user as appuser" \
    "SELECT id, name, email, created_at FROM app.users LIMIT 1;"

echo ""
echo "--- 3. Negative Constraint Checks ---"
assert_failure "Reject empty name" \
    "INSERT INTO app.users (name, email) VALUES ('', 'test_empty_name@example.com');" \
    "users_name_length"

assert_failure "Reject untrimmed name" \
    "INSERT INTO app.users (name, email) VALUES ('  Ada Lovelace  ', 'test_untrimmed_name@example.com');" \
    "users_name_trimmed"

assert_failure "Reject oversized name (>100 characters)" \
    "INSERT INTO app.users (name, email) VALUES (repeat('a', 101), 'test_oversized_name@example.com');" \
    "users_name_length"

assert_failure "Reject NULL name" \
    "INSERT INTO app.users (name, email) VALUES (NULL, 'test_null_name@example.com');" \
    "null value in column \"name\""

assert_failure "Reject empty email" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', '');" \
    "violates check constraint"

assert_failure "Reject undersized email (<3 characters)" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', 'a@');" \
    "users_email_length"

assert_failure "Reject uppercase email (must be normalized lowercase)" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', 'Uppercase@Example.com');" \
    "users_email_canonical"

assert_failure "Reject untrimmed email" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', ' trimmed@example.com ');" \
    "users_email_canonical"

assert_failure "Reject email without '@'" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', 'invalid-email-no-at.com');" \
    "users_email_format"

assert_failure "Reject oversized email (>254 characters)" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', repeat('a', 245) || '@example.com');" \
    "users_email_length"

assert_failure "Reject duplicate email (named constraint users_email_unique)" \
    "DO \$\$
    DECLARE
        v_email text := 'dup_' || floor(extract(epoch from clock_timestamp())) || '@example.com';
    BEGIN
        INSERT INTO app.users (name, email) VALUES ('User 1', v_email);
        INSERT INTO app.users (name, email) VALUES ('User 2', v_email);
    END;
    \$\$;" \
    "users_email_unique"

echo ""
echo "--- 4. Least-Privilege Authorization Boundaries as appuser ---"
assert_failure "Prohibit DDL: CREATE TABLE in schema app" \
    "CREATE TABLE app.unauthorized (id INT);" \
    "permission denied"

assert_failure "Prohibit DDL: ALTER TABLE on app.users" \
    "ALTER TABLE app.users ADD COLUMN unauthorized_col TEXT;" \
    "must be owner"

assert_failure "Prohibit DDL: DROP TABLE on app.users" \
    "DROP TABLE app.users;" \
    "must be owner"

assert_failure "Prohibit DML: UPDATE on app.users" \
    "UPDATE app.users SET name = 'Modified' WHERE email = 'ada@example.com';" \
    "permission denied"

assert_failure "Prohibit DML: DELETE on app.users" \
    "DELETE FROM app.users WHERE email = 'ada@example.com';" \
    "permission denied"

assert_failure "Prohibit DML: TRUNCATE on app.users" \
    "TRUNCATE app.users;" \
    "permission denied"

assert_failure "Prohibit Privilege Escalation: SET ROLE app_owner" \
    "SET ROLE app_owner;" \
    "permission denied"

assert_failure "Prohibit Privilege Escalation: SET ROLE app_migrator" \
    "SET ROLE app_migrator;" \
    "permission denied"

echo ""
echo "================================================================="
echo "Verification Summary: $PASSED_COUNT passed, $FAILED_COUNT failed."
echo "================================================================="

if [[ $FAILED_COUNT -ne 0 ]]; then
    exit 1
fi
