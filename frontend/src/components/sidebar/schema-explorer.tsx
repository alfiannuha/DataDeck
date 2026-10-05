"use client";

import { RefreshCw } from "lucide-react";
import { useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/use-connections";
import { useDatabases } from "@/hooks/use-databases";
import { useOnline } from "@/hooks/use-online";
import { useSchema } from "@/hooks/use-schema";
import { ApiClientError } from "@/lib/api-client";
import { buildCopyDdlQuery, buildCountRows, buildSelectTop100, extractDdl, identifierQuoteFor, supportsCopyDdl } from "@/lib/sql/identifiers";
import { executeQuery } from "@/lib/api/endpoints";
import { copyText } from "@/lib/clipboard";
import { queryKeys } from "@/lib/query-keys";
import { useQueryClient } from "@tanstack/react-query";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { DatabaseChildren, SchemaTree, TreeRow } from "./schema-tree";

/** A table action that also carries the database it belongs to (PRF-01). */
type DatabaseTableAction = (schema: string, table: string, database: string) => void;

/**
 * Schema explorer container. Two modes:
 *
 * - Server-level PostgreSQL profile (PRF-01): lists databases from the real
 *   discovery endpoint, then lazily loads each database's schema tree only when
 *   the database node is expanded. Metadata is keyed by connection + database.
 * - MySQL/SQLite (and legacy PostgreSQL profiles): the original single-database
 *   tree, fetched once for the connection.
 */
export function SchemaExplorer({
  connectionId,
}: {
  connectionId: string | null;
}) {
  const queryClient = useQueryClient();
  const insertQuerySql = useWorkspaceStore((state) => state.insertQuerySql);
  const openTableData = useWorkspaceStore((state) => state.openTableData);
  const setActiveDatabase = useConnectionStore(
    (state) => state.setActiveDatabase,
  );
  const { data: connections } = useConnections();
  const online = useOnline();
  const connection = connections?.find((candidate) => candidate.id === connectionId);
  const driver = connection?.driver ?? null;
  // A PostgreSQL profile with no default database is a server-level connection
  // (PRF-01): it uses database discovery. A PostgreSQL profile that carries a
  // database_name keeps the legacy single-database tree (backward compatible).
  const serverLevel = driver === "postgres" && !connection?.database_name;
  const identifierQuote = identifierQuoteFor(driver);

  // Legacy/MySQL/SQLite connections use the original single-database query.
  const query = useSchema(connectionId, null, {
    enabled: Boolean(connectionId) && !serverLevel,
  });

  const [notice, setNotice] = useState<{ kind: "success" | "error"; text: string } | null>(null);
  const noticeTimer = useRef<number | null>(null);

  function namespaceFor(schema: string, database: string): string {
    return driver === "mysql" ? database : schema;
  }

  // Bind the tab to the database the action came from. Server-level PostgreSQL
  // binds the explicit database; legacy PostgreSQL binds the profile default;
  // MySQL/SQLite leave it null so the backend uses the profile database.
  function bindDatabase(database: string): string | null {
    if (driver !== "postgres") return null;
    return database || connection?.database_name || null;
  }

  function showNotice(kind: "success" | "error", text: string) {
    setNotice({ kind, text });
    if (noticeTimer.current) window.clearTimeout(noticeTimer.current);
    noticeTimer.current = window.setTimeout(() => setNotice(null), 2500);
  }

  const handleSelectTop100: DatabaseTableAction = (schema, table, database) =>
    insertQuerySql(
      buildSelectTop100(namespaceFor(schema, connection?.database_name ?? ""), table, 100, identifierQuote),
      connectionId,
      bindDatabase(database),
    );

  const handleCountRows: DatabaseTableAction = (schema, table, database) =>
    insertQuerySql(
      buildCountRows(namespaceFor(schema, connection?.database_name ?? ""), table, identifierQuote),
      connectionId,
      bindDatabase(database),
    );

  // Double-click a table opens a Table Data tab bound to the explicit target.
  // A server-level PostgreSQL profile without a resolved database never falls
  // back to a bootstrap database — the open is refused with feedback instead.
  const handleOpenTableData: DatabaseTableAction = (schema, table, database) => {
    if (!connectionId) return;
    if (driver === "postgres") {
      const target = bindDatabase(database);
      if (!target) {
        showNotice("error", "Select a database before opening table data.");
        return;
      }
      openTableData(connectionId, target, schema || null, table);
      return;
    }
    openTableData(connectionId, null, schema || null, table);
  };

  const handleCopyDdl: DatabaseTableAction | undefined = supportsCopyDdl(driver)
    ? (schema, table) => {
        const ddlQuery = buildCopyDdlQuery(
          driver,
          namespaceFor(schema, connection?.database_name ?? ""),
          table,
        );
        if (!ddlQuery || !connectionId) return;
        void executeQuery({
          connection_id: connectionId,
          sql: ddlQuery.sql,
          timeout_seconds: 30,
        })
          .then(async (result) => {
            const ddl = extractDdl(result.rows, ddlQuery.ddlColumnIndex);
            if (!ddl) {
              showNotice("error", "No DDL returned for this object.");
              return;
            }
            const copied = await copyText(ddl);
            showNotice(copied ? "success" : "error", copied ? "DDL copied" : "Copy failed");
            void queryClient.invalidateQueries({ queryKey: queryKeys.queryHistoryRoot });
          })
          .catch((error) => {
            showNotice(
              "error",
              error instanceof ApiClientError ? error.message : "DDL retrieval failed",
            );
          });
      }
    : undefined;

  function refresh() {
    if (serverLevel) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.databases(connectionId ?? "") });
      return;
    }
    void query.refetch();
  }

  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between px-3 py-2">
        <span className="text-xs font-medium uppercase tracking-wide text-subtle-foreground">
          Schema
        </span>
        <div className="flex items-center gap-2">
          {notice && (
            <span
              role={notice.kind === "error" ? "alert" : "status"}
              className={notice.kind === "error" ? "text-[10px] text-destructive" : "text-[10px] text-emerald-400"}
            >
              {notice.text}
            </span>
          )}
          {connectionId && (
            <Button
              variant="ghost"
              size="icon"
              aria-label="Refresh schema"
              onClick={refresh}
              disabled={serverLevel ? false : query.isFetching}
            >
              <RefreshCw size={13} aria-hidden="true" />
            </Button>
          )}
        </div>
      </div>

      <div className="px-2 pb-2">
        {!connectionId ? (
          <p className="px-1 text-xs text-muted-foreground">
            Select a connection to browse its schema.
          </p>
        ) : serverLevel ? (
          <DatabaseExplorer
            connectionId={connectionId}
            onSelectDatabase={setActiveDatabase}
            onSelectTop100={handleSelectTop100}
            onCountRows={handleCountRows}
            onCopyDdl={handleCopyDdl}
            onOpenTableData={handleOpenTableData}
          />
        ) : query.isLoading ? (
          <p className="px-1 text-xs text-muted-foreground">Loading schema…</p>
        ) : query.isError ? (
          <div role="alert" className="flex flex-col gap-2 px-1">
            <p className="text-xs text-destructive">
              {query.error instanceof ApiClientError
                ? `${query.error.message} (${query.error.code})`
                : "Could not load schema."}
            </p>
            <Button variant="outline" size="sm" onClick={refresh}>
              Retry
            </Button>
          </div>
        ) : hasIntrospectableTables(query.data) ? (
          <SchemaTree
            key={connectionId}
            databases={query.data ?? []}
            connectionId={connectionId}
            onSelectTop100={(schema, table) => handleSelectTop100(schema, table, "")}
            onCountRows={(schema, table) => handleCountRows(schema, table, "")}
            onOpenTableData={(schema, table, database) =>
              handleOpenTableData(schema, table, database)
            }
            onCopyDdl={
              handleCopyDdl
                ? (schema, table) => handleCopyDdl(schema, table, "")
                : undefined
            }
          />
        ) : (
          <p className="px-1 text-xs text-muted-foreground">No schema objects found.</p>
        )}
        {!online && serverLevel && (
          <p className="px-1 pt-1 text-[10px] text-amber-400">
            Offline: database discovery is unavailable.
          </p>
        )}
      </div>
    </div>
  );
}

/** Number of database nodes rendered before a "show more" control. */
export const DATABASE_BATCH_SIZE = 50;

/** Lists databases for a server-level connection (PRF-01) with lazy expansion. */
function DatabaseExplorer({
  connectionId,
  onSelectDatabase,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
  onOpenTableData,
}: {
  connectionId: string;
  onSelectDatabase: (connectionId: string, database: string | null) => void;
  onSelectTop100?: DatabaseTableAction;
  onCountRows?: DatabaseTableAction;
  onCopyDdl?: DatabaseTableAction;
  onOpenTableData?: DatabaseTableAction;
}) {
  const databases = useDatabases(connectionId, true);
  const activeDatabase = useConnectionStore(
    (state) => state.activeDatabaseByConnection[connectionId] ?? null,
  );
  const [visible, setVisible] = useState(DATABASE_BATCH_SIZE);

  if (databases.isLoading) {
    return <p className="px-1 text-xs text-muted-foreground">Loading databases…</p>;
  }
  if (databases.isError) {
    return (
      <div role="alert" className="flex flex-col gap-2 px-1">
        <p className="text-xs text-destructive">
          {databases.error instanceof ApiClientError
            ? `${databases.error.message} (${databases.error.code})`
            : "Could not list databases."}
        </p>
        <Button variant="outline" size="sm" onClick={() => void databases.refetch()}>
          Retry
        </Button>
      </div>
    );
  }
  const list = databases.data ?? [];
  if (list.length === 0) {
    return <p className="px-1 text-xs text-muted-foreground">No databases found.</p>;
  }

  const shown = list.slice(0, visible);
  const remaining = list.length - shown.length;
  return (
    <ul aria-label="Databases" className="flex flex-col gap-0.5 py-1">
      {shown.map((database) => (
        <LazyDatabaseNode
          key={database.name}
          connectionId={connectionId}
          database={database.name ?? ""}
          selected={activeDatabase === database.name}
          onSelect={onSelectDatabase}
          onRefreshDatabases={() => void databases.refetch()}
          onSelectTop100={onSelectTop100}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
          onOpenTableData={onOpenTableData}
        />
      ))}
      {remaining > 0 && (
        <li>
          <div className="px-1 py-0.5">
            <button
              type="button"
              onClick={() => setVisible((value) => value + DATABASE_BATCH_SIZE)}
              className="rounded px-1 text-[10px] text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
            >
              Show {Math.min(DATABASE_BATCH_SIZE, remaining)} more ({remaining} remaining)
            </button>
          </div>
        </li>
      )}
    </ul>
  );
}

/**
 * One database node. Schemas are fetched only when the node is expanded
 * (`enabled: open`), so opening a connection never introspects every database.
 * A failing database shows a localized error and does not affect siblings.
 */
function LazyDatabaseNode({
  connectionId,
  database,
  selected,
  onSelect,
  onRefreshDatabases,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
  onOpenTableData,
}: {
  connectionId: string;
  database: string;
  selected: boolean;
  onSelect: (connectionId: string, database: string | null) => void;
  onRefreshDatabases: () => void;
  onSelectTop100?: DatabaseTableAction;
  onCountRows?: DatabaseTableAction;
  onCopyDdl?: DatabaseTableAction;
  onOpenTableData?: DatabaseTableAction;
}) {
  const [open, setOpen] = useState(false);
  const schema = useSchema(connectionId, database, { enabled: open });
  const tree = schema.data?.[0];

  function toggle() {
    setOpen((value) => {
      const next = !value;
      // Selecting (opening) a database only changes the EXPLORER selection; it
      // never rebinds an existing query tab (PRF-01/T09).
      if (next) onSelect(connectionId, database);
      return next;
    });
  }

  return (
    <li>
      <TreeRow
        depth={0}
        label={database}
        secondary={open && schema.isSuccess ? undefined : "database"}
        expandable
        open={open}
        selected={selected}
        onToggle={toggle}
      />
      {open &&
        (schema.isLoading ? (
          <p className="px-1 py-0.5 text-[10px] text-muted-foreground">
            Loading schemas…
          </p>
        ) : schema.isError ? (
          <div role="alert" className="flex flex-col gap-1 px-1 py-0.5">
            <p className="text-[10px] text-destructive">
              {schema.error instanceof ApiClientError
                ? `${schema.error.message} (${schema.error.code})`
                : "Could not load this database."}
            </p>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => void schema.refetch()}
                className="rounded px-1 text-[10px] text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
              >
                Retry
              </button>
              {(schema.error instanceof ApiClientError &&
                ["DATABASE_NOT_FOUND", "CONNECTION_ERROR", "BOOTSTRAP_DATABASE_UNAVAILABLE"].includes(
                  schema.error.code,
                )) && (
                <button
                  type="button"
                  onClick={onRefreshDatabases}
                  className="rounded px-1 text-[10px] text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                >
                  Refresh databases
                </button>
              )}
            </div>
          </div>
        ) : (
          <DatabaseChildren
            schemas={tree?.schemas ?? []}
            tables={tree?.tables ?? []}
            depth={0}
            onSelectTop100={
              onSelectTop100
                ? (schema, table) => onSelectTop100(schema, table, database)
                : undefined
            }
            onCountRows={
              onCountRows
                ? (schema, table) => onCountRows(schema, table, database)
                : undefined
            }
            onCopyDdl={
              onCopyDdl
                ? (schema, table) => onCopyDdl(schema, table, database)
                : undefined
            }
            onOpenTableData={
              onOpenTableData
                ? (schema, table) =>
                    onOpenTableData(schema, table, database)
                : undefined
            }
          />
        ))}
    </li>
  );
}

function hasIntrospectableTables(
  databases:
    | { schemas?: { tables?: unknown[] }[]; tables?: unknown[] }[]
    | undefined,
): boolean {
  if (!databases) return false;
  return databases.some(
    (database) =>
      (database.tables ?? []).length > 0 ||
      (database.schemas ?? []).some((schema) => (schema.tables ?? []).length > 0),
  );
}
