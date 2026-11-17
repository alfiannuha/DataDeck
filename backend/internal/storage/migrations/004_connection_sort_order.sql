-- 004_connection_sort_order.sql — manual connection ordering (v0.4.x).
--
-- Additive and non-destructive: existing rows are backfilled with a unique,
-- deterministic position matching the previous default order (name, id) so the
-- sidebar does not visibly change on upgrade. Credentials/metadata are untouched.

ALTER TABLE connection_profiles ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- Backfill 0..n-1 by (name, id): the count of rows that sort before this one.
UPDATE connection_profiles SET sort_order = (
    SELECT COUNT(*) FROM connection_profiles AS earlier
    WHERE earlier.name < connection_profiles.name
       OR (earlier.name = connection_profiles.name AND earlier.id < connection_profiles.id)
);

CREATE INDEX IF NOT EXISTS idx_connection_profiles_sort
    ON connection_profiles(sort_order, name, id);
