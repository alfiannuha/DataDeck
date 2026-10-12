# Database Explorer

The **Schema Explorer** in the sidebar shows the connection tree and provides
navigation and context actions.

## Navigation

- **Databases** (server-level PostgreSQL) — expand a database to lazily load its
  schemas and tables. Metadata is loaded per database, so opening a connection does
  not introspect every database.
- **Schemas / Tables** — expand schemas to reveal tables; expand a table to see its
  columns, primary key, foreign keys, and indexes.
- **Double-click a table** to open a **Table Data** tab bound to that exact
  database/schema/table.

## Selection vs. tab binding

Selecting a database changes the **Explorer selection only**. It never rebinds an
existing tab and never retargets a running action. A tab keeps the connection and
database it was opened with, even if you later select a different database.

## Refresh

- **Refresh** (table menu / header) re-fetches explorer metadata only; it does not
  run queries or reload tab data.
- **Refresh Database** (database menu) re-fetches that database's discovery and
  schema metadata.

## Context menus

**Table menu**: View Data · New Query · Structure · Refresh · Export · Import ·
Copy Table Name · Copy Qualified Name.

**Database menu**: New Query · Refresh Database · Reconnect · Export SQL ·
Import SQL (Restore) · Copy Database Name · Delete Database.

Menus open via the `⋯` trigger, right-click, or the keyboard (`Shift+F10` /
Context-Menu key). Disabled items show an inline reason.

## Read-only objects

Views, materialized views, and foreign tables are browsable but read-only. Import
into them is disabled, and row Edit/Delete is disabled.

## Loading and errors

Each database/schema node shows its own loading and error state; a failing database
does not block its siblings. Errors surface the backend message and code with a
retry control.
