# Quick Start

A first-run walkthrough. It uses the real UI labels in DataDeck v0.1.0.

## 1. Start DataDeck

```bash
export ENCRYPTION_KEY="$(openssl rand -hex 32)"
./datadeck
```

The daemon listens on `http://127.0.0.1:8080` by default
([configuration](configuration.md)).

## 2. Open the application

Open `http://127.0.0.1:8080`. You land in an **empty workspace**. Nothing is
opened automatically — connection and database selection never create a query tab.

## 3. Create a connection profile

Click **New Connection** (top bar / sidebar). Provide a name, driver
(PostgreSQL/MySQL/SQLite), and connection details, then **Test** and **Save**.

- PostgreSQL: host, port, username, password; `database_name` is **optional**
  (a server-level profile discovers databases).
- MySQL: a database name is required.
- SQLite: a filesystem path to a `.db`/`.sqlite` file.

## 4. Explore databases and tables

Select the connection in the sidebar **Schema Explorer**. For a server-level
PostgreSQL profile, databases are listed; expand one to load its schemas/tables
lazily. Double-click a table to open a **Table Data** tab.

## 5. Browse table data

In the Table Data tab, page through rows (server-side pagination), **Filter**, and
**Sort**. Rows render with `NULL` shown distinctly; 64-bit integers are shown as
strings to preserve precision.

## 6. Open a query tab explicitly

Click **New Query** (top bar) to create a **Query tab** bound to the active
connection/database. Type SQL and press **Cmd/Ctrl+Enter** to run the selection or
the current statement.

> Raw SQL can modify or delete data. DataDeck does not make arbitrary SQL
> reversible — review statements before running them.

## 7. Inspect results

Results appear in the virtualized grid below the editor. Errors show the backend
message and code. Successful executions are recorded in **History**.

## 8. Save a query

Use **Saved Queries** to persist a snippet (title + SQL). Queries are stored
locally in the embedded app store.

## 9. Where to go next

- [Connections](../user-guide/connections.md)
- [Database Explorer](../user-guide/database-explorer.md)
- [Query Editor](../user-guide/query-editor.md)
- [Table Data](../user-guide/table-data.md)
- [Row CRUD](../user-guide/row-crud.md)
- [Table Import/Export](../user-guide/table-import-export.md)
- [Database Management](../user-guide/database-management.md)
