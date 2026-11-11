"use client";

import { Check, ChevronRight, Copy, Database, Eye, Folder, Layers, MoreHorizontal, Table2 } from "lucide-react";
import { Fragment, memo, useState, type ReactNode } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuHint,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { copyText } from "@/lib/clipboard";
import { getTableActions, type TableActionId } from "@/lib/table-actions";
import { cn } from "@/lib/utils";
import type {
  DatabaseSchemaTree,
  IntrospectedColumn,
  IntrospectedSchema,
  IntrospectedTable,
} from "@/types/api";

import { DatabaseMenu, type DatabaseMenuHandler } from "./database-menu";

export type TableActionHandler = (schema: string, table: string) => void;

/**
 * Table context menu dispatch (PRF-02/T12). `database` is the explicit target
 * captured at the node that owns the table (schema name for engines without a
 * schema level), so the menu never resolves its target from the Explorer's
 * current selection or the active tab.
 */
export type TableMenuHandler = (
  action: TableActionId,
  schema: string,
  table: string,
  database: string,
  objectType?: string,
) => void;

/** A table-menu dispatch already closed over its owning database. */
type TableMenuDispatch = (
  action: TableActionId,
  schema: string,
  table: string,
  objectType?: string,
) => void;

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
  engine,
  onCountRows,
  onCopyDdl,
  onOpenTableData,
  onTableMenu,
  onDatabaseMenu,
}: {
  databases: DatabaseSchemaTree[];
  connectionId: string;
  engine?: string;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
  /** Opens a Table Data tab; receives the owning database context. */
  onOpenTableData?: (schema: string, table: string, database: string) => void;
  /** Table context menu dispatch; receives the owning database context. */
  onTableMenu?: TableMenuHandler;
  /** Database context menu dispatch; receives the owning database (T16). */
  onDatabaseMenu?: DatabaseMenuHandler;
}) {
  return (
    <ul aria-label="Schema tree" className="flex flex-col gap-0.5 py-1">
      {databases.map((database, index) => (
        <DatabaseNode
          key={`${connectionId}:${database.name ?? index}`}
          database={database}
          depth={0}
          engine={engine}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
          onOpenTableData={onOpenTableData}
          onTableMenu={onTableMenu}
          onDatabaseMenu={onDatabaseMenu}
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
  engine,
  onCountRows,
  onCopyDdl,
  onOpenTableData,
  onTableMenu,
  onDatabaseMenu,
}: {
  database: DatabaseSchemaTree;
  depth: number;
  engine?: string;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
  onOpenTableData?: (schema: string, table: string, database: string) => void;
  onTableMenu?: TableMenuHandler;
  onDatabaseMenu?: DatabaseMenuHandler;
}) {
  const [open, setOpen] = useState(true);
  const [menuOpen, setMenuOpen] = useState(false);
  const schemas = database.schemas ?? [];
  const tables = database.tables ?? [];
  const childCount = schemas.length + tables.length;
  const databaseName = database.name ?? "";
  const secondary =
    schemas.length > 0
      ? `${schemas.length} schema${schemas.length === 1 ? "" : "s"}`
      : `${tables.length} table${tables.length === 1 ? "" : "s"}`;

  function openMenu(event: { preventDefault: () => void }) {
    event.preventDefault();
    setMenuOpen(true);
  }

  return (
    <li>
      <TreeRow
        depth={depth}
        label={database.name ?? "database"}
        icon={<Database size={12} className="mt-0.5 shrink-0 text-muted-foreground" aria-hidden="true" />}
        secondary={secondary}
        expandable={childCount > 0}
        open={open}
        onToggle={() => setOpen((value) => !value)}
        onContextMenu={onDatabaseMenu ? openMenu : undefined}
        onKeyDown={
          onDatabaseMenu
            ? (event) => {
                if (
                  event.key === "ContextMenu" ||
                  (event.key === "F10" && event.shiftKey)
                ) {
                  openMenu(event);
                }
              }
            : undefined
        }
        actions={
          onDatabaseMenu ? (
            <DatabaseMenu
              database={databaseName}
              engine={engine}
              open={menuOpen}
              onOpenChange={setMenuOpen}
              onAction={(action) => onDatabaseMenu(action, databaseName)}
            />
          ) : undefined
        }
      />
      {open && childCount > 0 && (
        <DatabaseChildren
          schemas={schemas}
          tables={tables}
          depth={depth}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
          onOpenTableData={
            onOpenTableData
              ? (schema, table) =>
                  onOpenTableData(schema, table, database.name ?? "")
              : undefined
          }
          onTableMenu={
            onTableMenu
              ? (action, schema, table, objectType) =>
                  onTableMenu(
                    action,
                    schema,
                    table,
                    database.name ?? "",
                    objectType,
                  )
              : undefined
          }
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
  onCountRows,
  onCopyDdl,
  onOpenTableData,
  onTableMenu,
}: {
  schemas: IntrospectedSchema[];
  tables: IntrospectedTable[];
  depth: number;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
  onOpenTableData?: TableActionHandler;
  onTableMenu?: TableMenuDispatch;
}) {
  return (
    <ul>
      {schemas.map((schema, index) => (
        <SchemaNode
          key={`${schema.name ?? index}`}
          schema={schema}
          depth={depth + 1}
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
          onOpenTableData={onOpenTableData}
          onTableMenu={onTableMenu}
        />
      ))}
      <TableList
        tables={tables}
        depth={depth + 1}
        onCountRows={onCountRows}
        onCopyDdl={onCopyDdl}
        onOpenTableData={onOpenTableData}
        onTableMenu={onTableMenu}
      />
    </ul>
  );
}

const SchemaNode = memo(function SchemaNode({
  schema,
  depth,
  onCountRows,
  onCopyDdl,
  onOpenTableData,
  onTableMenu,
}: {
  schema: IntrospectedSchema;
  depth: number;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
  onOpenTableData?: TableActionHandler;
  onTableMenu?: TableMenuDispatch;
}) {
  const [open, setOpen] = useState(false);
  const tables = schema.tables ?? [];
  return (
    <li>
      <TreeRow
        depth={depth}
        label={schema.name ?? "schema"}
        icon={<Folder size={12} className="mt-0.5 shrink-0 text-muted-foreground" aria-hidden="true" />}
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
            onCountRows={onCountRows}
            onCopyDdl={onCopyDdl}
            onOpenTableData={onOpenTableData}
            onTableMenu={onTableMenu}
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
  onCountRows,
  onCopyDdl,
  onOpenTableData,
  onTableMenu,
}: {
  tables: IntrospectedTable[];
  depth: number;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
  onOpenTableData?: TableActionHandler;
  onTableMenu?: TableMenuDispatch;
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
          onCountRows={onCountRows}
          onCopyDdl={onCopyDdl}
          onOpenTableData={onOpenTableData}
          onTableMenu={onTableMenu}
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
  onCountRows,
  onCopyDdl,
  onOpenTableData,
  onTableMenu,
}: {
  table: IntrospectedTable;
  depth: number;
  onCountRows?: TableActionHandler;
  onCopyDdl?: TableActionHandler;
  onOpenTableData?: TableActionHandler;
  onTableMenu?: TableMenuDispatch;
}) {
  const [open, setOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const columns = table.columns ?? [];
  const foreignKeys = table.foreign_keys ?? [];
  const indexes = table.indexes ?? [];

  // Capabilities are computed from the captured tree metadata at render time;
  // opening the menu never triggers a network fetch (T12 §33).
  const tableActions = getTableActions({ objectType: table.type });

  function openMenu(event: { preventDefault: () => void }) {
    event.preventDefault();
    setMenuOpen(true);
  }

  return (
    <li>
      <TreeRow
        depth={depth}
        label={table.name ?? "table"}
        icon={objectTypeIcon(table.type)}
        expandable
        open={open}
        onToggle={() => setOpen((value) => !value)}
        onOpen={
          onOpenTableData
            ? () => onOpenTableData(table.schema ?? "", table.name ?? "")
            : undefined
        }
        onContextMenu={onTableMenu ? openMenu : undefined}
        onKeyDown={
          onTableMenu
            ? (event) => {
                if (event.key === "ContextMenu" || (
                  event.key === "F10" && event.shiftKey
                )) {
                  openMenu(event);
                }
              }
            : undefined
        }
        actions={
          onTableMenu ? (
            <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen} modal={false}>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  aria-label={`More actions for ${table.name ?? ""}`}
                  title={`Actions for ${table.name ?? ""} (Shift+F10)`}
                  className={menuTriggerClass}
                >
                  <MoreHorizontal size={13} aria-hidden="true" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                {tableActions.map((action) => (
                  <Fragment key={action.id}>
                    {(action.id === "refresh" || action.id === "import") && (
                      <DropdownMenuSeparator />
                    )}
                    <DropdownMenuItem
                      disabled={action.disabled}
                      data-testid={`table-menu-${action.id}`}
                      onSelect={() => {
                        onTableMenu(
                          action.id,
                          table.schema ?? "",
                          table.name ?? "",
                          table.type,
                        );
                      }}
                    >
                        {action.label}
                        {action.reason ? (
                          <DropdownMenuHint>{action.reason}</DropdownMenuHint>
                        ) : null}
                      </DropdownMenuItem>
                    </Fragment>
                  ))}
                {(onCountRows || onCopyDdl) && <DropdownMenuSeparator />}
                {onCountRows ? (
                  <DropdownMenuItem
                    data-testid="table-menu-count-rows"
                    onSelect={() => onCountRows(table.schema ?? "", table.name ?? "")}
                  >
                    Count Rows
                  </DropdownMenuItem>
                ) : null}
                {onCopyDdl ? (
                  <DropdownMenuItem
                    data-testid="table-menu-copy-ddl"
                    onSelect={() => onCopyDdl(table.schema ?? "", table.name ?? "")}
                  >
                    Copy DDL
                  </DropdownMenuItem>
                ) : null}
              </DropdownMenuContent>
              </DropdownMenu>
          ) : null
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
  icon,
  secondary,
  expandable,
  open,
  onToggle,
  onOpen,
  actions,
  selected,
  onContextMenu,
  onKeyDown,
}: {
  depth: number;
  label: string;
  /** Leading object-type icon (PRF-04/Phase 5). */
  icon?: ReactNode;
  secondary?: string;
  expandable: boolean;
  open?: boolean;
  onToggle?: () => void;
  /** Activation (double-click) — e.g. open a Table Data tab. */
  onOpen?: () => void;
  actions?: ReactNode;
  /** Marks the explorer's currently selected database (not a query binding). */
  selected?: boolean;
  /** Right-click on the row (e.g. table context menu). */
  onContextMenu?: (event: React.MouseEvent<HTMLDivElement>) => void;
  /** Keyboard context-menu keys (Shift+F10 / Menu). */
  onKeyDown?: (event: React.KeyboardEvent<HTMLDivElement>) => void;
}) {
  return (
    <div
      style={indent(depth)}
      aria-current={selected ? "true" : undefined}
      onContextMenu={onContextMenu}
      onKeyDown={onKeyDown}
      className={cn(
        "group flex items-start gap-1 rounded py-0.5 pr-1 hover:bg-panel-raised",
        selected && "bg-panel-raised",
      )}
    >
      {expandable ? (
        <button
          type="button"
          aria-expanded={open}
          onClick={onToggle}
          onDoubleClick={onOpen}
          className="flex min-w-0 flex-1 items-start gap-1 rounded text-left text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
        >
          <ChevronRight
            size={12}
            aria-hidden="true"
            className={cn(
              "mt-0.5 shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
          {icon}
          <span
            title={label}
            className={cn(
              "min-w-0 flex-1 whitespace-normal break-words font-medium leading-snug",
              selected && "text-accent",
            )}
          >
            {label}
          </span>
          {secondary && (
            <span className="shrink-0 pt-0.5 text-[10px] text-subtle-foreground">
              {secondary}
            </span>
          )}
        </button>
      ) : (
        <div className="flex min-w-0 flex-1 items-start gap-1 pl-[14px] text-xs text-foreground">
          {icon}
          <span
            title={label}
            className="min-w-0 flex-1 whitespace-normal break-words font-medium leading-snug"
          >
            {label}
          </span>
          {secondary && (
            <span className="shrink-0 pt-0.5 text-[10px] text-subtle-foreground">
              {secondary}
            </span>
          )}
        </div>
      )}
      {actions}
    </div>
  );
}

// Always-visible row action trigger (PRF-04/Phase 5): the 3-dot menu replaces
// the old hover-only shortcuts, so opening it never shifts the layout.
const menuTriggerClass =
  "shrink-0 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]";

/** Leading icon for a relation, replacing redundant type labels (PRF-04/Phase 5). */
function objectTypeIcon(type?: string): ReactNode {
  const upper = (type ?? "").toUpperCase();
  const className = "mt-0.5 shrink-0 text-muted-foreground";
  if (upper.includes("MATERIALIZED")) {
    return <Layers size={12} className={className} aria-hidden="true" />;
  }
  if (upper.includes("VIEW")) {
    return <Eye size={12} className={className} aria-hidden="true" />;
  }
  return <Table2 size={12} className={className} aria-hidden="true" />;
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
      className={menuTriggerClass}
    >
      {copied ? (
        <Check size={12} aria-hidden="true" />
      ) : (
        <Copy size={12} aria-hidden="true" />
      )}
    </button>
  );
}
