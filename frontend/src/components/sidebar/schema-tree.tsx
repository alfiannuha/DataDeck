"use client";

import { Check, ChevronRight, Copy, FileCode, Hash, ListOrdered } from "lucide-react";
import { memo, useState, type ReactNode } from "react";

import { copyText } from "@/lib/clipboard";
import { cn } from "@/lib/utils";
import type {
  DatabaseSchemaTree,
  IntrospectedColumn,
  IntrospectedSchema,
  IntrospectedTable,
} from "@/types/api";

export type TableActionHandler = (schema: string, table: string) => void;

/**
 * Schema tree rendered from the M1 introspection response:
 * database → schema → table → columns / primary key / foreign keys / indexes.
 *
 * Children are only rendered when a node is expanded, so large schemas never
 * mount every descendant at once.
 */
export function SchemaTree({
  databases,
  connectionId,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
}: {
  databases: DatabaseSchemaTree[];
  connectionId: string;
  onSelectTop100?: TableActionHandler;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
}) {
  return (
    <ul aria-label="Schema tree" className="flex flex-col gap-0.5 py-1">
      {databases.map((database, index) => (
        <DatabaseNode
          key={`${connectionId}:${database.name ?? index}`}
          database={database}
          depth={0}
          onSelectTop100={onSelectTop100}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
        />
      ))}
    </ul>
  );
}

function indent(depth: number) {
  return { paddingLeft: `${depth * 12 + 4}px` };
}

const DatabaseNode = memo(function DatabaseNode({
  database,
  depth,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
}: {
  database: DatabaseSchemaTree;
  depth: number;
  onSelectTop100?: TableActionHandler;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
}) {
  const [open, setOpen] = useState(true);
  const schemas = database.schemas ?? [];
  const tables = database.tables ?? [];
  const childCount = schemas.length + tables.length;
  const secondary =
    schemas.length > 0
      ? `${schemas.length} schema${schemas.length === 1 ? "" : "s"}`
      : `${tables.length} table${tables.length === 1 ? "" : "s"}`;
  return (
    <li>
      <TreeRow
        depth={depth}
        label={database.name ?? "database"}
        secondary={secondary}
        expandable={childCount > 0}
        open={open}
        onToggle={() => setOpen((value) => !value)}
      />
      {open && childCount > 0 && (
        <DatabaseChildren
          schemas={schemas}
          tables={tables}
          depth={depth}
          onSelectTop100={onSelectTop100}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
        />
      )}
    </li>
  );
});

/**
 * Renders the contents of a database node (schemas first, then any tables that
 * hang directly off the database). Exported so the PRF-01 database explorer can
 * reuse it for lazily-loaded per-database subtrees.
 */
export function DatabaseChildren({
  schemas,
  tables,
  depth,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
}: {
  schemas: IntrospectedSchema[];
  tables: IntrospectedTable[];
  depth: number;
  onSelectTop100?: TableActionHandler;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
}) {
  return (
    <ul>
      {schemas.map((schema, index) => (
        <SchemaNode
          key={`${schema.name ?? index}`}
          schema={schema}
          depth={depth + 1}
          onSelectTop100={onSelectTop100}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
        />
      ))}
      <TableList
        tables={tables}
        depth={depth + 1}
        onSelectTop100={onSelectTop100}
        onCountRows={onCountRows}
        onCopyDdl={onCopyDdl}
      />
    </ul>
  );
}

const SchemaNode = memo(function SchemaNode({
  schema,
  depth,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
}: {
  schema: IntrospectedSchema;
  depth: number;
  onSelectTop100?: TableActionHandler;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
}) {
  const [open, setOpen] = useState(false);
  const tables = schema.tables ?? [];
  return (
    <li>
      <TreeRow
        depth={depth}
        label={schema.name ?? "schema"}
        secondary={`${tables.length} table${tables.length === 1 ? "" : "s"}`}
        expandable={tables.length > 0}
        open={open}
        onToggle={() => setOpen((value) => !value)}
      />
      {open && tables.length > 0 && (
        <ul>
          <TableList
            tables={tables}
            depth={depth + 1}
            onSelectTop100={onSelectTop100}
            onCountRows={onCountRows}
            onCopyDdl={onCopyDdl}
          />
        </ul>
      )}
    </li>
  );
});

// Number of tables rendered before an incremental "show more" control. Keeps
// very large table-level schemas bounded without changing expand/collapse,
// context actions or keyboard behavior.
export const TABLE_BATCH_SIZE = 100;

const TableList = memo(function TableList({
  tables,
  depth,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
}: {
  tables: IntrospectedTable[];
  depth: number;
  onSelectTop100?: TableActionHandler;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
}) {
  const [visible, setVisible] = useState(TABLE_BATCH_SIZE);
  const shown = tables.slice(0, visible);
  const remaining = tables.length - shown.length;

  return (
    <>
      {shown.map((table, index) => (
        <TableNode
          key={`${table.name ?? index}`}
          table={table}
          depth={depth}
          onSelectTop100={onSelectTop100}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
        />
      ))}
      {remaining > 0 && (
        <li>
          <div style={indent(depth)} className="flex items-center py-0.5 pr-1">
            <button
              type="button"
              onClick={() => setVisible((value) => value + TABLE_BATCH_SIZE)}
              className="rounded px-1 text-[10px] text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
            >
              Show {Math.min(TABLE_BATCH_SIZE, remaining)} more ({remaining}{" "}
              remaining)
            </button>
          </div>
        </li>
      )}
    </>
  );
});

const TableNode = memo(function TableNode({
  table,
  depth,
  onSelectTop100,
  onCountRows,
  onCopyDdl,
}: {
  table: IntrospectedTable;
  depth: number;
  onSelectTop100?: TableActionHandler;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
}) {
  const [open, setOpen] = useState(false);
  const columns = table.columns ?? [];
  const foreignKeys = table.foreign_keys ?? [];
  const indexes = table.indexes ?? [];
  const qualifiedName = table.schema
    ? `${table.schema}.${table.name ?? ""}`
    : table.name ?? "";

  return (
    <li>
      <TreeRow
        depth={depth}
        label={table.name ?? "table"}
        secondary={table.type}
        expandable
        open={open}
        onToggle={() => setOpen((value) => !value)}
        actions={
          <>
            {onSelectTop100 && (
              <ActionButton
                label={`Select top 100 from ${table.name ?? ""}`}
                title="Insert SELECT * … LIMIT 100 into a query tab"
                icon={<ListOrdered size={12} aria-hidden="true" />}
                onClick={() =>
                  onSelectTop100(table.schema ?? "", table.name ?? "")
                }
              />
            )}
            {onCountRows && (
              <ActionButton
                label={`Count rows in ${table.name ?? ""}`}
                title="Insert SELECT COUNT(*) into a query tab"
                icon={<Hash size={12} aria-hidden="true" />}
                onClick={() =>
                  onCountRows(table.schema ?? "", table.name ?? "")
                }
              />
            )}
            {onCopyDdl && (
              <ActionButton
                label={`Copy DDL for ${table.name ?? ""}`}
                title="Copy the table's CREATE statement"
                icon={<FileCode size={12} aria-hidden="true" />}
                onClick={() =>
                  onCopyDdl(table.schema ?? "", table.name ?? "")
                }
              />
            )}
            <CopyButton
              label={`Copy table name ${table.name ?? ""}`}
              text={qualifiedName}
            />
          </>
        }
      />
      {open && (
        <ul>
          {table.primary_key && (table.primary_key.columns ?? []).length > 0 && (
            <li>
              <div
                style={indent(depth + 1)}
                className="flex items-center gap-1 py-0.5 pr-1 text-xs text-muted-foreground"
              >
                <span className="font-medium text-accent">PK</span>
                <span className="truncate">
                  {(table.primary_key.columns ?? []).join(", ")}
                </span>
              </div>
            </li>
          )}
          {columns.map((column, index) => (
            <ColumnRow
              key={`${column.name ?? index}`}
              column={column}
              depth={depth + 1}
            />
          ))}
          {foreignKeys.map((foreignKey, index) => (
            <li key={`fk-${foreignKey.name ?? index}`}>
              <div
                style={indent(depth + 1)}
                className="flex items-center gap-1 py-0.5 pr-1 text-xs text-muted-foreground"
              >
                <span className="font-medium text-accent">FK</span>
                <span className="truncate">
                  {foreignKey.referenced_schema}.
                  {foreignKey.referenced_table}(
                  {(foreignKey.columns ?? []).join(", ")})
                </span>
              </div>
            </li>
          ))}
          {indexes.map((index, i) => (
            <li key={`ix-${index.name ?? i}`}>
              <div
                style={indent(depth + 1)}
                className="flex items-center gap-1 py-0.5 pr-1 text-xs text-muted-foreground"
              >
                <span className="font-medium text-accent">IX</span>
                <span className="truncate">
                  {index.name}
                  {index.unique ? " (unique)" : ""} [{(index.columns ?? []).join(", ")}]
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
});

const ColumnRow = memo(function ColumnRow({
  column,
  depth,
}: {
  column: IntrospectedColumn;
  depth: number;
}) {
  return (
    <li>
      <div
        style={indent(depth)}
        className="group flex items-center gap-1 rounded py-0.5 pr-1 hover:bg-panel-raised"
      >
        <span className="min-w-0 flex-1 truncate text-xs text-foreground">
          {column.name}
        </span>
        <span className="shrink-0 text-[10px] text-subtle-foreground">
          {column.data_type}
          {column.nullable ? "" : " · NOT NULL"}
        </span>
        <CopyButton
          label={`Copy column name ${column.name ?? ""}`}
          text={column.name ?? ""}
        />
      </div>
    </li>
  );
});

export function TreeRow({
  depth,
  label,
  secondary,
  expandable,
  open,
  onToggle,
  actions,
}: {
  depth: number;
  label: string;
  secondary?: string;
  expandable: boolean;
  open?: boolean;
  onToggle?: () => void;
  actions?: ReactNode;
}) {
  return (
    <div
      style={indent(depth)}
      className="group flex items-center gap-1 rounded py-0.5 pr-1 hover:bg-panel-raised"
    >
      {expandable ? (
        <button
          type="button"
          aria-expanded={open}
          onClick={onToggle}
          className="flex min-w-0 flex-1 items-center gap-1 rounded text-left text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
        >
          <ChevronRight
            size={12}
            aria-hidden="true"
            className={cn(
              "shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
          <span className="truncate font-medium">{label}</span>
          {secondary && (
            <span className="shrink-0 text-[10px] text-subtle-foreground">
              {secondary}
            </span>
          )}
        </button>
      ) : (
        <div className="flex min-w-0 flex-1 items-center gap-1 pl-[14px] text-xs text-foreground">
          <span className="truncate font-medium">{label}</span>
          {secondary && (
            <span className="shrink-0 text-[10px] text-subtle-foreground">
              {secondary}
            </span>
          )}
        </div>
      )}
      {actions}
    </div>
  );
}

const actionButtonClass =
  "shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-opacity hover:text-foreground focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)] group-hover:opacity-100";

function ActionButton({
  label,
  title,
  icon,
  onClick,
}: {
  label: string;
  title: string;
  icon: ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={title}
      onClick={onClick}
      className={actionButtonClass}
    >
      {icon}
    </button>
  );
}

function CopyButton({ label, text }: { label: string; text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={() => {
        void copyText(text).then((ok) => {
          if (ok) {
            setCopied(true);
            window.setTimeout(() => setCopied(false), 1200);
          }
        });
      }}
      className={actionButtonClass}
    >
      {copied ? (
        <Check size={12} aria-hidden="true" />
      ) : (
        <Copy size={12} aria-hidden="true" />
      )}
    </button>
  );
}
