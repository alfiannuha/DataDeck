# Query Editor

The SQL editor (CodeMirror 6) lives in a **Query tab** that you open explicitly.
Opening a connection does not create a query tab ([empty workspace](../getting-started/quick-start.md)).

## Create a query tab

Click **New Query** in the top bar, or use **New Query** from a table/database
context menu (which pre-populates a quoted `SELECT`). A new tab is always created;
existing tabs — including those with unsaved SQL — are never overwritten.

## Run SQL

- Press **Cmd/Ctrl+Enter** to run the selection, or the current statement if there
  is no selection.
- Results appear in the virtualized grid below the editor.
- Executions are recorded in **Query History**.

> **Raw SQL is powerful and can modify or delete data.** DataDeck does not make
> arbitrary SQL reversible. Review statements before running them, and prefer
> read-only credentials for exploration (see [least privilege](../security/security-guide.md#least-privilege)).

## Database binding

Each Query tab is bound to a connection and (for server-level PostgreSQL) a
database. Running a query uses **the tab's binding**, never the current Explorer
selection. Changing the Explorer selection does not retarget the tab.

## Multiple tabs

You can open many Query tabs. Each keeps its own SQL and binding; switching tabs
does not move content between them.

## Timeouts and cancellation

Queries run under a bounded timeout. When supported, a running query can be
cancelled. Database-side timeouts surface as a timeout error (not a syntax error).

## History and Saved Queries

- **History** lists recent executions newest-first, with an optional connection
  filter.
- **Saved Queries** persist title + SQL locally. Saving does not execute anything.

## Errors

Execution errors show a sanitized backend message with a stable code (e.g.
`SQL_SYNTAX_ERROR` with a position, `CONNECTION_ERROR`, `QUERY_TIMEOUT`). No
credentials or DSNs are ever shown.
