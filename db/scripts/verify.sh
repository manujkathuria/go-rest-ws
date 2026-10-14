#!/usr/bin/env bash
# =============================================================================
# Database Verification Script for go-rest-ws (Issue #4)
# Verifies data contracts, constraints, isolation, and least-privilege boundaries
# =============================================================================

set -euo pipefail

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-go_rest_ws}"
APPUSER="${APPUSER:-appuser}"
APPUSER_PASSWORD="${APPUSER_PASSWORD:-appuser_dev_pw}"
APP_MIGRATOR="${APP_MIGRATOR:-app_migrator}"
APP_MIGRATOR_PASSWORD="${APP_MIGRATOR_PASSWORD:-app_migrator_dev_pw}"

PSQL_APPUSER="env PGPASSWORD=$APPUSER_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $APPUSER -d $DB_NAME -v ON_ERROR_STOP=1 -X -q"
PSQL_MIGRATOR="env PGPASSWORD=$APP_MIGRATOR_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $APP_MIGRATOR -d $DB_NAME -v ON_ERROR_STOP=1 -X -q"

echo "================================================================="
echo "Starting Database Verification for $DB_NAME"
echo "Target: $DB_HOST:$DB_PORT | Database: $DB_NAME | Role: $APPUSER"
echo "PostgreSQL Server Version: $($PSQL_APPUSER -t -A -c 'SELECT version();')"
echo "================================================================="

PASSED_COUNT=0
FAILED_COUNT=0

assert_success() {
    local desc="$1"
    local sql="$2"
    echo -n "  [TEST] $desc ... "
    if $PSQL_APPUSER -c "$sql" > /dev/null 2>&1; then
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
    if output=$($PSQL_APPUSER -c "$sql" 2>&1); then
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

assert_equal() {
    local desc="$1"
    local actual="$2"
    local expected="$3"
    echo -n "  [TEST] $desc ... "
    if [[ "$actual" == "$expected" ]]; then
        echo "PASS"
        PASSED_COUNT=$((PASSED_COUNT + 1))
    else
        echo "FAIL (Expected '$expected', got '$actual')"
        FAILED_COUNT=$((FAILED_COUNT + 1))
    fi
}

echo ""
echo "--- 1. Schema Placement & Absence in Public ---"

PUBLIC_USERS_COUNT=$($PSQL_APPUSER -t -A -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'users';")
assert_equal "Table 'users' is ABSENT from schema 'public'" "$PUBLIC_USERS_COUNT" "0"

APP_USERS_COUNT=$($PSQL_APPUSER -t -A -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'app' AND table_name = 'users';")
assert_equal "Table 'users' EXISTS in schema 'app'" "$APP_USERS_COUNT" "1"

echo ""
echo "--- 2. Positive Operations & Value Verification (Isolated Transaction) ---"

# Test positive INSERT returning identity and timestamp, then SELECT, wrapped in ROLLBACK
TEST_UUID=$($PSQL_APPUSER -t -A -c "SELECT gen_random_uuid();")
TEST_NAME="Ada Lovelace"
TEST_EMAIL="ada_${TEST_UUID}@example.com"

# Run inside a transaction that verifies RETURNING values, selects the row, and rolls back
TX_OUTPUT=$($PSQL_APPUSER -t -A -F "|" -c "
BEGIN;
    INSERT INTO app.users (name, email)
    VALUES ('$TEST_NAME', '$TEST_EMAIL')
    RETURNING id, name, email, (created_at IS NOT NULL AND created_at <= clock_timestamp());
ROLLBACK;
")

INSERTED_ID=$(echo "$TX_OUTPUT" | head -n 1 | cut -d '|' -f 1)
INSERTED_NAME=$(echo "$TX_OUTPUT" | head -n 1 | cut -d '|' -f 2)
INSERTED_EMAIL=$(echo "$TX_OUTPUT" | head -n 1 | cut -d '|' -f 3)
INSERTED_TIME_VALID=$(echo "$TX_OUTPUT" | head -n 1 | cut -d '|' -f 4)

echo -n "  [TEST] Returned identity is a positive integer ... "
if [[ "$INSERTED_ID" =~ ^[0-9]+$ ]] && [[ "$INSERTED_ID" -gt 0 ]]; then
    echo "PASS (id: $INSERTED_ID)"
    PASSED_COUNT=$((PASSED_COUNT + 1))
else
    echo "FAIL (Invalid id: '$INSERTED_ID')"
    FAILED_COUNT=$((FAILED_COUNT + 1))
fi

assert_equal "Returned name matches inserted name" "$INSERTED_NAME" "$TEST_NAME"
assert_equal "Returned email matches inserted email" "$INSERTED_EMAIL" "$TEST_EMAIL"
assert_equal "Returned created_at is valid non-null timestamp" "$INSERTED_TIME_VALID" "t"

# Confirm isolation: verify no orphan row was persisted after ROLLBACK
ISOLATION_COUNT=$($PSQL_APPUSER -t -A -c "SELECT count(*) FROM app.users WHERE email = '$TEST_EMAIL';")
assert_equal "Test row rolled back cleanly (zero orphan rows left behind)" "$ISOLATION_COUNT" "0"

echo ""
echo "--- 3. Negative Constraint Checks ---"

assert_failure "Reject empty name" \
    "INSERT INTO app.users (name, email) VALUES ('', 'empty_name@example.com');" \
    "users_name_length"

assert_failure "Reject tab-only name (Go strings.TrimSpace parity)" \
    "INSERT INTO app.users (name, email) VALUES (E'\t', 'tab_only@example.com');" \
    "users_name_trimmed"

assert_failure "Reject space-only name" \
    "INSERT INTO app.users (name, email) VALUES ('   ', 'spaces@example.com');" \
    "users_name_trimmed"

assert_failure "Reject name with leading tab" \
    "INSERT INTO app.users (name, email) VALUES (E'\tAda', 'lead_tab@example.com');" \
    "users_name_trimmed"

assert_failure "Reject name with trailing newline" \
    "INSERT INTO app.users (name, email) VALUES (E'Ada\n', 'trail_nl@example.com');" \
    "users_name_trimmed"

assert_failure "Reject oversized name (>100 characters)" \
    "INSERT INTO app.users (name, email) VALUES (repeat('a', 101), 'oversized_name@example.com');" \
    "users_name_length"

assert_failure "Reject NULL name" \
    "INSERT INTO app.users (name, email) VALUES (NULL, 'null_name@example.com');" \
    "null value in column \"name\""

assert_failure "Reject empty email" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', '');" \
    "violates check constraint"

assert_failure "Reject undersized email (<3 characters)" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', 'a@');" \
    "users_email_length"

assert_failure "Reject uppercase email (must be lowercase normalized)" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', 'Uppercase@example.com');" \
    "users_email_canonical"

assert_failure "Reject email with leading/trailing whitespace or tabs" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', E'\tuser@example.com');" \
    "users_email_canonical"

assert_failure "Reject email without '@'" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', 'invalid-email-no-at.com');" \
    "users_email_format"

assert_failure "Reject oversized email (>254 characters)" \
    "INSERT INTO app.users (name, email) VALUES ('Valid Name', repeat('a', 245) || '@example.com');" \
    "users_email_length"

assert_failure "Reject duplicate normalized email via named constraint" \
    "DO \$\$
    DECLARE
        v_email text := 'dup_' || gen_random_uuid() || '@example.com';
    BEGIN
        INSERT INTO app.users (name, email) VALUES ('User 1', v_email);
        INSERT INTO app.users (name, email) VALUES ('User 2', v_email);
    END;
    \$\$;" \
    "users_email_unique"

echo ""
echo "--- 4. Concurrent Duplicate Insert Verification ---"

CONCURRENT_EMAIL="concurrent_$(uuidgen | tr '[:upper:]' '[:lower:]')@example.com"
TMP_OUT1=$(mktemp)
TMP_OUT2=$(mktemp)

# Launch two simultaneous insert jobs as appuser
( $PSQL_APPUSER -c "INSERT INTO app.users (name, email) VALUES ('Concurrent 1', '$CONCURRENT_EMAIL');" > "$TMP_OUT1" 2>&1 ) &
PID1=$!
( $PSQL_APPUSER -c "INSERT INTO app.users (name, email) VALUES ('Concurrent 2', '$CONCURRENT_EMAIL');" > "$TMP_OUT2" 2>&1 ) &
PID2=$!

wait $PID1 && STATUS1=0 || STATUS1=$?
wait $PID2 && STATUS2=0 || STATUS2=$?

echo -n "  [TEST] Exactly one concurrent insert succeeds and one is rejected by users_email_unique ... "
if { [[ $STATUS1 -eq 0 ]] && [[ $STATUS2 -ne 0 ]] && grep -q "users_email_unique" "$TMP_OUT2"; } || \
   { [[ $STATUS2 -eq 0 ]] && [[ $STATUS1 -ne 0 ]] && grep -q "users_email_unique" "$TMP_OUT1"; }; then
    echo "PASS"
    PASSED_COUNT=$((PASSED_COUNT + 1))
else
    echo "FAIL (Status1=$STATUS1, Status2=$STATUS2)"
    FAILED_COUNT=$((FAILED_COUNT + 1))
fi

rm -f "$TMP_OUT1" "$TMP_OUT2"

# Clean up concurrent fixture record via app_migrator
$PSQL_MIGRATOR -c "SET ROLE app_owner; DELETE FROM app.users WHERE email = '$CONCURRENT_EMAIL';" > /dev/null 2>&1

echo ""
echo "--- 5. Future-Object Default Privileges Verification ---"

# 1. app_migrator (as app_owner) creates a new future table
$PSQL_MIGRATOR -c "
SET ROLE app_owner;
CREATE TABLE IF NOT EXISTS app.future_default_priv_test (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    note TEXT NOT NULL
);
RESET ROLE;
" > /dev/null 2>&1

# 2. appuser (without explicit table grant) must receive SELECT & INSERT via ALTER DEFAULT PRIVILEGES
assert_success "Default privileges: appuser can INSERT into future table created by app_owner" \
    "INSERT INTO app.future_default_priv_test (note) VALUES ('default privilege verification');"

assert_success "Default privileges: appuser can SELECT from future table" \
    "SELECT id, note FROM app.future_default_priv_test;"

# 3. appuser must NOT receive UPDATE, DELETE, or DDL on future table
assert_failure "Default privileges: appuser cannot UPDATE future table" \
    "UPDATE app.future_default_priv_test SET note = 'hacked';" \
    "permission denied"

assert_failure "Default privileges: appuser cannot DROP future table" \
    "DROP TABLE app.future_default_priv_test;" \
    "must be owner"

# Clean up future table via app_migrator
$PSQL_MIGRATOR -c "SET ROLE app_owner; DROP TABLE app.future_default_priv_test; RESET ROLE;" > /dev/null 2>&1

echo ""
echo "--- 6. Least-Privilege Authorization Boundaries as appuser ---"

assert_failure "Prohibit Schema Creation: CREATE SCHEMA unauthorized" \
    "CREATE SCHEMA unauthorized;" \
    "permission denied"

assert_failure "Prohibit DDL: CREATE TABLE in schema public" \
    "CREATE TABLE public.unauthorized_tbl (id INT);" \
    "permission denied"

assert_failure "Prohibit DDL: CREATE TABLE in schema app" \
    "CREATE TABLE app.unauthorized_tbl (id INT);" \
    "permission denied"

assert_failure "Prohibit DDL: ALTER TABLE on app.users" \
    "ALTER TABLE app.users ADD COLUMN unauthorized_col TEXT;" \
    "must be owner"

assert_failure "Prohibit DDL: DROP TABLE on app.users" \
    "DROP TABLE app.users;" \
    "must be owner"

assert_failure "Prohibit DML: UPDATE on app.users" \
    "UPDATE app.users SET name = 'Modified' WHERE email = 'test@example.com';" \
    "permission denied"

assert_failure "Prohibit DML: DELETE on app.users" \
    "DELETE FROM app.users WHERE email = 'test@example.com';" \
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
