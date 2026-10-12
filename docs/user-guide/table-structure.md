# Table Structure

A **read-only** inspection of one relation. Open it from the table context menu
(**Structure**) to view canonical metadata — DataDeck never reads a row of data and
never mutates anything here.

## What is shown

- **Columns** — name, native database type, normalized type, nullability, default,
  ordinal position, and insertable/updatable flags.
- **Primary key** — columns in key order.
- **Indexes** — name, uniqueness, method, partial flag, and columns.
- **Constraints** — PRIMARY KEY, UNIQUE, CHECK; CHECK/UNIQUE carry their definition
  text.
- **Foreign keys** — columns, referenced schema/table/columns, and `ON UPDATE` /
  `ON DELETE` actions.

Constraint `definition` text is rendered as **plain text only** (never HTML).

## Consistency with row editing

Structure metadata reuses the same derivation as row editing: generated columns are
not insertable/updatable, identity/primary-key columns are not updatable in v0.1.0,
and non-base relations are read-only. So what you see here matches what the CRUD
forms allow.

## Engine differences

- **PostgreSQL** — has a schema level; native types like `character varying(255)`,
  `numeric(10,2)`.
- **MySQL** — no schema level; types reflect MySQL's vocabulary.
- **SQLite** — no schema level; `INTEGER PRIMARY KEY` is the rowid alias.

## Read-only objects

Views, materialized views, and foreign tables are shown as read-only.

## Not a schema designer

There is no schema editing, no `ALTER TABLE`, and no table designer in v0.1.0.

## Errors

Missing relation → `TABLE_NOT_FOUND`; unknown connection → `NOT_FOUND`;
introspection timeout/unreachable → `INTROSPECTION_TIMEOUT` / `INTROSPECTION_ERROR`.
