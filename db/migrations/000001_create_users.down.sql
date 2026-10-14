-- Migration: 000001_create_users.down.sql
-- Description: Drop app.users table
-- Executed as app_migrator assuming role app_owner

\set ON_ERROR_STOP on

SET ROLE app_owner;

DROP TABLE IF EXISTS app.users;

RESET ROLE;
