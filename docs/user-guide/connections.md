# Connections

A **connection profile** is a saved set of connection details. Passwords are
encrypted at rest with the `ENCRYPTION_KEY` and are **never returned** by the API
or shown in the UI.

## Create a connection

1. Click **New Connection**.
2. Choose a driver: **PostgreSQL**, **MySQL**, or **SQLite**.
3. Fill in the details:
   - **PostgreSQL** — host, port, username, password. `database_name` is
     **optional**: leave it empty for a *server-level* profile that can discover
     and browse many databases.
   - **MySQL** — host, port, username, password, and a required database name.
   - **SQLite** — a filesystem path to the database file (no host/credentials).
4. **Test** to validate the parameters (nothing is saved by a test), then **Save**.

## Open / select a connection

Selecting a connection shows it in the Schema Explorer. Selection is **explorer
state only** — it never rebinds an existing tab.

## PostgreSQL server-level profiles

A server-level PostgreSQL profile (no default database) discovers the databases
visible to its credentials. Selecting a database in the Explorer binds *new*
actions (and new tabs) to that database. A default database, if set, is used when
a request does not specify one.

## Test a connection

The **Test** action opens a short-lived connection and reports success or a
specific error (unreachable server, unknown database, denied database access,
authentication failure). Nothing is persisted.

## Reconnect

Use **Reconnect** in a database's context menu to replace the connection pool for
that database. See [Database Management](database-management.md#reconnect).

## Delete a connection profile

**Delete** removes the saved profile and closes its pools. This removes the
**profile**, not any database on the server. Deleting a profile is **different**
from [deleting a database](database-management.md#delete-database).

## Errors you may see

| Situation | Behavior |
|---|---|
| Unreachable host | Connection error (sanitized; no credentials echoed) |
| Unknown database / denied access | Specific database error |
| Wrong password | Authentication/connection error |
| Missing `ENCRYPTION_KEY` | Server fails to start; profiles cannot be decrypted |
