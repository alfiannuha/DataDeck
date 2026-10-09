"use client";

import { Loader2, RefreshCw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useTableStructure } from "@/hooks/use-table-structure";
import { ApiClientError } from "@/lib/api-client";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Table Structure tab surface (PRF-02/T13). The tab owns its explicit
 * connection/database/schema/table binding; this component only reads that
 * binding and never consults the Explorer's global selection. Constraint
 * definition text is rendered as plain text only — never HTML and never
 * executed.
 */
export function TableStructurePanel() {
  const tab = useWorkspaceStore((state) =>
    state.tabs.find(
      (candidate) =>
        candidate.id === state.activeTabId &&
        candidate.kind === "table-structure",
    ),
  );

  const target = {
    connectionId: tab?.connectionId ?? null,
    database: tab?.database ?? null,
    schema: tab?.kind === "table-structure" ? tab.schema : null,
    table: tab?.kind === "table-structure" ? tab.table : "",
  };
  const query = useTableStructure(target);

  if (tab?.kind !== "table-structure") return null;

  const qualified = tab.schema ? `${tab.schema}.${tab.table}` : tab.table;
  const data = query.data;
  const error = query.error;
  const isApiError = error instanceof ApiClientError;
  const showFirstLoad = query.isLoading && !data;
  const showError = query.isError && !data;

  function refresh() {
    void query.refetch();
  }

  return (
    <section
      aria-label="Table structure"
      data-testid="table-structure-panel"
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
    >
      <header className="flex h-9 shrink-0 items-center justify-between gap-3 border-b border-border bg-panel px-3">
        <div className="flex min-w-0 items-center gap-2 text-xs">
          <span className="truncate font-medium text-foreground">{qualified}</span>
          <span aria-hidden="true" className="text-muted-foreground">
            ·
          </span>
          <span
            data-testid="table-structure-binding"
            className="truncate text-muted-foreground"
          >
            {tab.database ?? "default database"}
          </span>
          {data ? (
            <span className="truncate text-[10px] uppercase text-muted-foreground">
              {data.object_type}
            </span>
          ) : null}
        </div>
        <Button
          variant="ghost"
          size="sm"
          data-testid="table-structure-refresh"
          aria-label="Refresh table structure"
          onClick={refresh}
          disabled={query.isFetching || !data}
        >
          {query.isFetching ? (
            <Loader2 size={12} aria-hidden="true" className="animate-spin" />
          ) : (
            <RefreshCw size={12} aria-hidden="true" />
          )}
          Refresh
        </Button>
      </header>

      <div className="min-h-0 flex-1 overflow-auto bg-result-surface p-3">
        {showError ? (
          <div
            role="alert"
            data-testid="table-structure-error"
            className="flex h-full min-h-40 flex-col items-center justify-center gap-2 p-6 text-center"
          >
            <p className="text-sm font-medium text-foreground">
              Unable to load table structure.
            </p>
            <p className="text-xs text-destructive">
              {isApiError
                ? `${error.message} (${error.code})`
                : "Unexpected error."}
            </p>
            <Button variant="outline" size="sm" onClick={refresh}>
              Retry
            </Button>
          </div>
        ) : showFirstLoad ? (
          <div
            role="status"
            data-testid="table-structure-loading"
            className="flex h-full min-h-40 items-center justify-center gap-2 text-xs text-muted-foreground"
          >
            <Loader2 size={14} aria-hidden="true" className="animate-spin" />
            Loading table structure…
          </div>
        ) : data ? (
          <div className="flex flex-col gap-4">
            <StructureSection title="Columns" empty="No columns.">
              <StructureTable
                testId="table-structure-columns"
                headers={["Name", "Type", "Nullable", "Default", "Insert", "Update"]}
                rows={data.columns ?? []}
                renderRow={(column) => (
                  <>
                    <Cell>
                      {column.primary_key ? (
                        <span className="mr-1 rounded bg-primary/10 px-1 text-[9px] font-semibold uppercase text-primary">
                          PK
                        </span>
                      ) : null}
                      {column.name}
                    </Cell>
                    <Cell>
                      {column.normalized_type || column.database_type}
                    </Cell>
                    <Cell>{column.nullable ? "yes" : "no"}</Cell>
                    <Cell className="font-mono">{column.default ?? "—"}</Cell>
                    <Cell>{column.insertable ? "yes" : "no"}</Cell>
                    <Cell>{column.updatable ? "yes" : "no"}</Cell>
                  </>
                )}
              />
            </StructureSection>

            <StructureSection title="Primary key" empty="No primary key.">
              {data.primary_key ? (
                <p data-testid="table-structure-pk" className="text-xs text-foreground">
                  {data.primary_key.name ? `${data.primary_key.name}: ` : ""}
                  {(data.primary_key.columns ?? []).join(", ")}
                </p>
              ) : null}
            </StructureSection>

            <StructureSection title="Indexes" empty="No indexes.">
              <StructureTable
                testId="table-structure-indexes"
                headers={["Name", "Unique", "Primary", "Method", "Partial", "Columns"]}
                rows={data.indexes ?? []}
                renderRow={(index) => (
                  <>
                    <Cell>{index.name}</Cell>
                    <Cell>{index.unique ? "yes" : "no"}</Cell>
                    <Cell>{index.primary ? "yes" : "no"}</Cell>
                    <Cell>{index.method || "—"}</Cell>
                    <Cell>{index.partial ? "yes" : "no"}</Cell>
                    <Cell>
                      {(index.columns ?? []).map((column, i) => (
                        <span key={i} className="mr-1 inline-block rounded bg-muted px-1 font-mono text-[10px]">
                          {column === "" ? "(expression)" : column}
                        </span>
                      ))}
                    </Cell>
                  </>
                )}
              />
            </StructureSection>

            <StructureSection title="Constraints" empty="No constraints.">
              <StructureTable
                testId="table-structure-constraints"
                headers={["Name", "Type", "Columns", "Definition"]}
                rows={data.constraints ?? []}
                renderRow={(constraint) => (
                  <>
                    <Cell>{constraint.name}</Cell>
                    <Cell>{constraint.type}</Cell>
                    <Cell>{(constraint.columns ?? []).join(", ")}</Cell>
                    <Cell className="font-mono">{constraint.definition || "—"}</Cell>
                  </>
                )}
              />
            </StructureSection>

            <StructureSection title="Foreign keys" empty="No foreign keys.">
              <StructureTable
                testId="table-structure-foreign-keys"
                headers={["Name", "Columns", "References", "On update", "On delete"]}
                rows={data.foreign_keys ?? []}
                renderRow={(fk) => (
                  <>
                    <Cell>{fk.name}</Cell>
                    <Cell>{(fk.columns ?? []).join(", ")}</Cell>
                    <Cell>
                      {[fk.referenced_schema, fk.referenced_table]
                        .filter(Boolean)
                        .join(".")}
                      ({(fk.referenced_columns ?? []).join(", ")})
                    </Cell>
                    <Cell>{fk.on_update || "—"}</Cell>
                    <Cell>{fk.on_delete || "—"}</Cell>
                  </>
                )}
              />
            </StructureSection>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function StructureSection({
  title,
  empty,
  children,
}: {
  title: string;
  empty: string;
  children?: React.ReactNode;
}) {
  return (
    <section className="rounded border border-border bg-background">
      <h2 className="border-b border-border px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
        {title}
      </h2>
      <div className="p-3">{children ?? <p className="text-xs text-muted-foreground">{empty}</p>}</div>
    </section>
  );
}

function StructureTable<T>({
  testId,
  headers,
  rows,
  renderRow,
}: {
  testId: string;
  headers: string[];
  rows: T[];
  renderRow: (row: T) => React.ReactNode;
}) {
  if (rows.length === 0) return null;
  return (
    <table
      data-testid={testId}
      className="w-full border-collapse text-xs"
      aria-label="Structure table"
    >
      <thead>
        <tr>
          {headers.map((header) => (
            <th
              key={header}
              scope="col"
              className="border-b border-border px-2 py-1 text-left text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
            >
              {header}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, index) => (
          <tr key={index} className="align-top">
            {renderRow(row)}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Cell({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <td className={`px-2 py-1 text-foreground ${className ?? ""}`}>{children}</td>;
}