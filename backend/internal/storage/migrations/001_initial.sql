-- 001_initial.sql — DataDeck embedded application store (PRD §4).

CREATE TABLE IF NOT EXISTS connection_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    driver TEXT NOT NULL CHECK(driver IN ('postgres', 'mysql', 'sqlite')),
    host TEXT,
    port INTEGER,
    database_name TEXT NOT NULL,
    username TEXT,
    encrypted_password TEXT,
    ssl_mode TEXT DEFAULT 'disable',
    ssh_enabled INTEGER DEFAULT 0,
    ssh_host TEXT,
    ssh_port INTEGER,
    ssh_username TEXT,
    ssh_encrypted_password TEXT,
    ssh_key_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS query_history (
    id TEXT PRIMARY KEY,
    connection_id TEXT NOT NULL,
    sql_text TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('SUCCESS', 'ERROR')),
    execution_time_ms INTEGER NOT NULL,
    rows_affected INTEGER DEFAULT 0,
    error_message TEXT,
    executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(connection_id) REFERENCES connection_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS saved_queries (
    id TEXT PRIMARY KEY,
    connection_id TEXT,
    title TEXT NOT NULL,
    sql_text TEXT NOT NULL,
    tags TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(connection_id) REFERENCES connection_profiles(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_query_history_conn ON query_history(connection_id, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_saved_queries_conn ON saved_queries(connection_id);
