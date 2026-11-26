-- 005_connection_groups.sql — optional connection grouping (v0.4.2).
--
-- Additive and non-destructive: a new groups table plus a nullable group_id on
-- connections. Legacy connections keep group_id = NULL (ungrouped) so the
-- sidebar is unchanged until the user creates/assigns groups. Deleting a group
-- sets its members' group_id to NULL (connections are never deleted).

CREATE TABLE IF NOT EXISTS connection_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_expanded INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE connection_profiles ADD COLUMN group_id TEXT
    REFERENCES connection_groups(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_connection_groups_name
    ON connection_groups(name);
CREATE INDEX IF NOT EXISTS idx_connection_profiles_group
    ON connection_profiles(group_id, sort_order);
