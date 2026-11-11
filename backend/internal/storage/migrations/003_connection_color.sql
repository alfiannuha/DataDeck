-- 003_connection_color.sql — per-connection color (PRF-03).
--
-- Additive only: existing rows keep NULL ("no color assigned"), which the API
-- maps to a backward-compatible default. No row is rewritten. The value is a
-- presentation hint (a #RRGGBB hex string); it carries no credentials.

ALTER TABLE connection_profiles ADD COLUMN color TEXT;
