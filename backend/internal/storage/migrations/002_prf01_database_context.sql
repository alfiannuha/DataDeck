-- 002_prf01_database_context.sql — PRF-01 database attribution.
--
-- Additive only: existing rows keep NULL ("context unknown"). A PostgreSQL
-- server-level connection can execute against different databases, so history
-- and saved queries record which database they belong to. Legacy records are
-- never rewritten or bound to a current selection.

ALTER TABLE query_history ADD COLUMN database_name TEXT;
ALTER TABLE saved_queries ADD COLUMN database_name TEXT;
