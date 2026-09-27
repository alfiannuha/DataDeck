"use client";

import { useQueryClient } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/use-connections";
import { useSchema } from "@/hooks/use-schema";
import { ApiClientError } from "@/lib/api-client";
import { executeQuery } from "@/lib/api/endpoints";
import { copyText } from "@/lib/clipboard";
import { queryKeys } from "@/lib/query-keys";
import {
  buildCopyDdlQuery,
  buildCountRows,
  buildSelectTop100,
  extractDdl,
  identifierQuoteFor,
  supportsCopyDdl,
} from "@/lib/sql/identifiers";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { SchemaTree } from "./schema-tree";

const DDL_TIMEOUT_SECONDS = 30;

interface Notice {
  kind: "success" | "error";
  text: string;
}

/**
 * Schema explorer container. Fetches the real introspection endpoint for the
 * active connection and owns loading/error/empty states plus refresh.
 *
 * Table context actions generate driver-aware SQL: Select Top 100 and Count
 * Rows insert statements into a query tab (never auto-execute). Copy DDL runs
 * only an engine-native DDL retrieval statement, on explicit user action, and
 * is hidden for drivers without an accurate mechanism (PostgreSQL).
 */
export function SchemaExplorer({
  connectionId,
}: {
  connectionId: string | null;
}) {
  const query = useSchema(connectionId);
  const queryClient = useQueryClient();
  const insertQuerySql = useWorkspaceStore((state) => state.insertQuerySql);
  const { data: connections } = useConnections();
  const connection = connections?.find((candidate) => candidate.id === connectionId);
  const driver = connection?.driver ?? null;
  const identifierQuote = identifierQuoteFor(driver);

  const [notice, setNotice] = useState<Notice | null>(null);
  const noticeTimer = useRef<number | null>(null);

  // MySQL has no schema level: tables live in the connection's database, which
  // is the correct qualifier. PostgreSQL uses the table's schema; SQLite is
  // unqualified (schema is empty).
  function namespaceFor(schema: string): string {
    return driver === "mysql" ? connection?.database_name ?? schema : schema;
  }

  function showNotice(kind: Notice["kind"], text: string) {
    setNotice({ kind, text });
    if (noticeTimer.current) window.clearTimeout(noticeTimer.current);
    noticeTimer.current = window.setTimeout(() => setNotice(null), 2500);
  }

  function refresh() {
    void query.refetch();
  }

  async function copyDdl(schema: string, table: string) {
    const ddlQuery = buildCopyDdlQuery(driver, namespaceFor(schema), table);
    if (!ddlQuery || !connectionId) return;
    try {
      const result = await executeQuery({
        connection_id: connectionId,
        sql: ddlQuery.sql,
        timeout_seconds: DDL_TIMEOUT_SECONDS,
      });
      const ddl = extractDdl(result.rows, ddlQuery.ddlColumnIndex);
      if (!ddl) {
        showNotice("error", "No DDL returned for this object.");
        return;
      }
      const copied = await copyText(ddl);
      showNotice(
        copied ? "success" : "error",
        copied ? "DDL copied" : "Copy failed",
      );
      void queryClient.invalidateQueries({
        queryKey: queryKeys.queryHistoryRoot,
      });
    } catch (error) {
      const message =
        error instanceof ApiClientError ? error.message : "DDL retrieval failed";
      showNotice("error", message);
    }
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
              className={
                notice.kind === "error"
                  ? "text-[10px] text-destructive"
                  : "text-[10px] text-emerald-400"
              }
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
              disabled={query.isFetching}
            >
              <RefreshCw
                size={13}
                aria-hidden="true"
                className={query.isFetching ? "animate-spin" : undefined}
              />
            </Button>
          )}
        </div>
      </div>

      <div className="px-2 pb-2">
        {!connectionId ? (
          <p className="px-1 text-xs text-muted-foreground">
            Select a connection to browse its schema.
          </p>
        ) : query.isLoading ? (
          <p className="px-1 text-xs text-muted-foreground">
            Loading schema…
          </p>
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
            onSelectTop100={(schema, table) =>
              insertQuerySql(
                buildSelectTop100(
                  namespaceFor(schema),
                  table,
                  100,
                  identifierQuote,
                ),
                connectionId,
              )
            }
            onCountRows={(schema, table) =>
              insertQuerySql(
                buildCountRows(namespaceFor(schema), table, identifierQuote),
                connectionId,
              )
            }
            onCopyDdl={
              supportsCopyDdl(driver)
                ? (schema, table) => void copyDdl(schema, table)
                : undefined
            }
          />
        ) : (
          <p className="px-1 text-xs text-muted-foreground">
            No schema objects found.
          </p>
        )}
      </div>
    </div>
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
      (database.schemas ?? []).some(
        (schema) => (schema.tables ?? []).length > 0,
      ),
  );
}
