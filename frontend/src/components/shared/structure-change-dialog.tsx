"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useApplyStructure, type TableStructureTarget } from "@/hooks/use-table-structure";
import { ApiClientError } from "@/lib/api-client";
import type {
  StructureCapabilities,
  StructureChangeRequest,
  StructureColumn,
} from "@/types/api";

export type StructureDialogSpec =
  | { kind: "rename_table"; table: string }
  | { kind: "add_column" }
  | { kind: "drop_column"; column: string }
  | { kind: "alter_column"; column: StructureColumn }
  | { kind: "create_index" }
  | { kind: "drop_index"; index: string }
  | { kind: "add_constraint" }
  | { kind: "drop_constraint"; name: string; type: string };

const REFERENTIAL_ACTIONS = [
  "",
  "NO ACTION",
  "RESTRICT",
  "CASCADE",
  "SET NULL",
  "SET DEFAULT",
] as const;

type RefAction = Exclude<(typeof REFERENTIAL_ACTIONS)[number], "">;

const CONSTRAINT_TYPES = ["PRIMARY KEY", "UNIQUE", "CHECK", "FOREIGN KEY"] as const;

function csv(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

/**
 * One schema-change dialog (PRF-04/Phase 2). It builds the engine-neutral
 * StructureChangeRequest for the chosen operation and delegates SQL generation,
 * quoting, validation and execution to the backend. Capability flags hide
 * engine-specific fields (index method, partial index).
 */
export function StructureChangeDialog({
  open,
  onOpenChange,
  target,
  spec,
  capabilities,
  onApplied,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  target: TableStructureTarget;
  spec: StructureDialogSpec | null;
  capabilities?: StructureCapabilities;
  onApplied?: () => void;
}) {
  const apply = useApplyStructure();
  const [error, setError] = useState<string | null>(null);

  const [newName, setNewName] = useState("");
  const [colName, setColName] = useState("");
  const [colType, setColType] = useState("");
  const [colNullable, setColNullable] = useState(true);
  const [colDefault, setColDefault] = useState("");
  const [colDefaultExpr, setColDefaultExpr] = useState(false);
  const [defaultAction, setDefaultAction] = useState<"keep" | "set" | "drop">("keep");

  const [indexName, setIndexName] = useState("");
  const [indexUnique, setIndexUnique] = useState(false);
  const [indexMethod, setIndexMethod] = useState("");
  const [indexColumns, setIndexColumns] = useState("");
  const [indexWhere, setIndexWhere] = useState("");

  const [conType, setConType] = useState<(typeof CONSTRAINT_TYPES)[number]>("UNIQUE");
  const [conName, setConName] = useState("");
  const [conColumns, setConColumns] = useState("");
  const [conExpression, setConExpression] = useState("");
  const [refTable, setRefTable] = useState("");
  const [refColumns, setRefColumns] = useState("");
  const [onUpdate, setOnUpdate] = useState<RefAction | "">("");
  const [onDelete, setOnDelete] = useState<RefAction | "">("");

  useEffect(() => {
    if (!open) return;
    setError(null);
    setNewName(spec?.kind === "rename_table" ? spec.table : "");
    setColName(spec?.kind === "alter_column" ? (spec.column.name ?? "") : "");
    setColType(
      spec?.kind === "alter_column"
        ? (spec.column.database_type ?? "")
        : "",
    );
    setColNullable(spec?.kind === "alter_column" ? Boolean(spec.column.nullable) : true);
    setColDefault("");
    setColDefaultExpr(false);
    setDefaultAction("keep");
    setIndexName("");
    setIndexUnique(false);
    setIndexMethod("");
    setIndexColumns("");
    setIndexWhere("");
    setConType("UNIQUE");
    setConName("");
    setConColumns("");
    setConExpression("");
    setRefTable("");
    setRefColumns("");
    setOnUpdate("");
    setOnDelete("");
  }, [open, spec]);

  if (!spec) return null;

  function buildBody(): StructureChangeRequest {
    const base = { schema: target.schema ?? undefined, table: target.table };
    switch (spec!.kind) {
      case "rename_table":
        return { ...base, operation: "rename_table", new_name: newName.trim() };
      case "add_column":
        return {
          ...base,
          operation: "add_column",
          column: {
            name: colName.trim(),
            type: colType.trim(),
            nullable: colNullable,
            default: colDefault === "" ? undefined : colDefault,
            is_expression: colDefaultExpr,
          },
        };
      case "drop_column":
        return { ...base, operation: "drop_column", column_name: spec!.column };
      case "alter_column":
        return {
          ...base,
          operation: "alter_column",
          column_name: spec!.column.name ?? "",
          default_action: defaultAction === "keep" ? undefined : defaultAction,
          new_column: {
            name: colName.trim(),
            type: colType.trim(),
            nullable: colNullable,
            default: defaultAction === "set" ? colDefault : undefined,
            is_expression: colDefaultExpr,
          },
        };
      case "create_index":
        return {
          ...base,
          operation: "create_index",
          index: {
            name: indexName.trim(),
            unique: indexUnique,
            method: indexMethod.trim() || undefined,
            where: indexWhere.trim() || undefined,
            columns: csv(indexColumns).map((name) => ({ name })),
          },
        };
      case "drop_index":
        return { ...base, operation: "drop_index", index_name: spec!.index };
      case "add_constraint":
        return {
          ...base,
          operation: "add_constraint",
          constraint: {
            name: conName.trim() || undefined,
            type: conType,
            columns: conType === "CHECK" ? undefined : csv(conColumns),
            expression: conType === "CHECK" ? conExpression.trim() : undefined,
            reference:
              conType === "FOREIGN KEY"
                ? {
                    table: refTable.trim(),
                    columns: csv(refColumns),
                    on_update: onUpdate || undefined,
                    on_delete: onDelete || undefined,
                  }
                : undefined,
          },
        };
      case "drop_constraint":
        return {
          ...base,
          operation: "drop_constraint",
          constraint_name: spec!.name,
          constraint_type: spec!.type,
        };
    }
  }

  async function submit() {
    setError(null);
    try {
      await apply.mutateAsync({ target, body: buildBody() });
      onApplied?.();
      onOpenChange(false);
    } catch (err) {
      setError(
        err instanceof ApiClientError ? `${err.message} (${err.code})` : "Schema change failed.",
      );
    }
  }

  const title = TITLES[spec.kind];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-labelledby="structure-change-title">
        <DialogHeader>
          <DialogTitle id="structure-change-title">{title}</DialogTitle>
          <DialogDescription>
            {target.schema ? `${target.schema}.${target.table}` : target.table}
          </DialogDescription>
        </DialogHeader>

        <form
          className="grid grid-cols-2 gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          {spec.kind === "rename_table" ? (
            <Field label="New table name" id="chg-new-name" className="col-span-2">
              <Input
                id="chg-new-name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                autoFocus
              />
            </Field>
          ) : null}

          {spec.kind === "add_column" || spec.kind === "alter_column" ? (
            <>
              <Field label="Column name" id="chg-col-name" className="sm:col-span-1">
                <Input
                  id="chg-col-name"
                  value={colName}
                  onChange={(e) => setColName(e.target.value)}
                  autoFocus
                />
              </Field>
              <Field label="Type" id="chg-col-type" className="sm:col-span-1">
                <Input
                  id="chg-col-type"
                  placeholder="varchar(255)"
                  value={colType}
                  onChange={(e) => setColType(e.target.value)}
                />
              </Field>
              <label className="col-span-2 flex items-center gap-2 text-xs text-foreground">
                <input
                  type="checkbox"
                  aria-label="Nullable"
                  checked={colNullable}
                  onChange={(e) => setColNullable(e.target.checked)}
                />
                Nullable
              </label>
              {spec.kind === "alter_column" ? (
                <Field label="Default" id="chg-def-action" className="col-span-2">
                  <Select
                    id="chg-def-action"
                    value={defaultAction}
                    onChange={(e) => setDefaultAction(e.target.value as typeof defaultAction)}
                  >
                    <option value="keep">Keep existing</option>
                    <option value="set">Set default</option>
                    <option value="drop">Drop default</option>
                  </Select>
                </Field>
              ) : null}
              {spec.kind === "add_column" || defaultAction === "set" ? (
                <>
                  <Field label="Default value" id="chg-col-default" className="sm:col-span-1">
                    <Input
                      id="chg-col-default"
                      value={colDefault}
                      onChange={(e) => setColDefault(e.target.value)}
                    />
                  </Field>
                  <label className="sm:col-span-1 flex items-end gap-2 pb-1 text-xs text-foreground">
                    <input
                      type="checkbox"
                      aria-label="Default is a SQL expression"
                      checked={colDefaultExpr}
                      onChange={(e) => setColDefaultExpr(e.target.checked)}
                    />
                    SQL expression
                  </label>
                </>
              ) : null}
            </>
          ) : null}

          {spec.kind === "create_index" ? (
            <>
              <Field label="Index name" id="chg-idx-name" className="sm:col-span-1">
                <Input
                  id="chg-idx-name"
                  value={indexName}
                  onChange={(e) => setIndexName(e.target.value)}
                  autoFocus
                />
              </Field>
              <Field label="Columns (comma-separated)" id="chg-idx-cols" className="sm:col-span-1">
                <Input
                  id="chg-idx-cols"
                  placeholder="created_at, id"
                  value={indexColumns}
                  onChange={(e) => setIndexColumns(e.target.value)}
                />
              </Field>
              <label className="col-span-2 flex items-center gap-2 text-xs text-foreground">
                <input
                  type="checkbox"
                  aria-label="Unique index"
                  checked={indexUnique}
                  onChange={(e) => setIndexUnique(e.target.checked)}
                />
                Unique
              </label>
              {capabilities?.index_method ? (
                <Field label="Method" id="chg-idx-method" className="col-span-2">
                  <Input
                    id="chg-idx-method"
                    placeholder="btree"
                    value={indexMethod}
                    onChange={(e) => setIndexMethod(e.target.value)}
                  />
                </Field>
              ) : null}
              {capabilities?.partial_index ? (
                <Field label="Partial predicate (WHERE)" id="chg-idx-where" className="col-span-2">
                  <Input
                    id="chg-idx-where"
                    placeholder="status = 'open'"
                    value={indexWhere}
                    onChange={(e) => setIndexWhere(e.target.value)}
                  />
                </Field>
              ) : null}
            </>
          ) : null}

          {spec.kind === "add_constraint" ? (
            <>
              <Field label="Type" id="chg-con-type" className="sm:col-span-1">
                <Select
                  id="chg-con-type"
                  value={conType}
                  onChange={(e) => setConType(e.target.value as typeof conType)}
                >
                  {CONSTRAINT_TYPES.map((type) => (
                    <option key={type} value={type}>
                      {type}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="Name (optional)" id="chg-con-name" className="sm:col-span-1">
                <Input
                  id="chg-con-name"
                  value={conName}
                  onChange={(e) => setConName(e.target.value)}
                />
              </Field>
              {conType === "CHECK" ? (
                <Field label="Expression" id="chg-con-expr" className="col-span-2">
                  <Input
                    id="chg-con-expr"
                    placeholder="qty > 0"
                    value={conExpression}
                    onChange={(e) => setConExpression(e.target.value)}
                  />
                </Field>
              ) : (
                <Field label="Columns (comma-separated)" id="chg-con-cols" className="col-span-2">
                  <Input
                    id="chg-con-cols"
                    value={conColumns}
                    onChange={(e) => setConColumns(e.target.value)}
                  />
                </Field>
              )}
              {conType === "FOREIGN KEY" ? (
                <>
                  <Field label="Referenced table" id="chg-ref-table" className="sm:col-span-1">
                    <Input
                      id="chg-ref-table"
                      value={refTable}
                      onChange={(e) => setRefTable(e.target.value)}
                    />
                  </Field>
                  <Field label="Referenced columns" id="chg-ref-cols" className="sm:col-span-1">
                    <Input
                      id="chg-ref-cols"
                      value={refColumns}
                      onChange={(e) => setRefColumns(e.target.value)}
                    />
                  </Field>
                  <Field label="ON UPDATE" id="chg-on-update" className="sm:col-span-1">
                    <Select
                      id="chg-on-update"
                      value={onUpdate}
                      onChange={(e) => setOnUpdate(e.target.value as RefAction | "")}
                    >
                      {REFERENTIAL_ACTIONS.map((action) => (
                        <option key={action || "none"} value={action}>
                          {action || "(default)"}
                        </option>
                      ))}
                    </Select>
                  </Field>
                  <Field label="ON DELETE" id="chg-on-delete" className="sm:col-span-1">
                    <Select
                      id="chg-on-delete"
                      value={onDelete}
                      onChange={(e) => setOnDelete(e.target.value as RefAction | "")}
                    >
                      {REFERENTIAL_ACTIONS.map((action) => (
                        <option key={action || "none"} value={action}>
                          {action || "(default)"}
                        </option>
                      ))}
                    </Select>
                  </Field>
                </>
              ) : null}
            </>
          ) : null}

          {spec.kind === "drop_column" || spec.kind === "drop_index" || spec.kind === "drop_constraint" ? (
            <p className="col-span-2 text-xs text-destructive">
              This operation cannot be undone automatically. Confirm to proceed.
            </p>
          ) : null}

          {error ? (
            <p role="alert" className="col-span-2 text-xs text-destructive">
              {error}
            </p>
          ) : null}

          <DialogFooter className="col-span-2">
            <Button variant="outline" type="button" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant={DESTRUCTIVE.has(spec.kind) ? "destructive" : "default"}
              disabled={apply.isPending}
            >
              {apply.isPending ? "Applying…" : DESTRUCTIVE.has(spec.kind) ? "Apply change" : "Apply"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

const TITLES: Record<StructureDialogSpec["kind"], string> = {
  rename_table: "Rename table",
  add_column: "Add column",
  drop_column: "Drop column",
  alter_column: "Alter column",
  create_index: "Create index",
  drop_index: "Drop index",
  add_constraint: "Add constraint",
  drop_constraint: "Drop constraint",
};

const DESTRUCTIVE = new Set<StructureDialogSpec["kind"]>([
  "drop_column",
  "drop_index",
  "drop_constraint",
]);

function Field({
  label,
  id,
  className,
  children,
}: {
  label: string;
  id: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={className}>
      <Label htmlFor={id} className="mb-1 block">
        {label}
      </Label>
      {children}
    </div>
  );
}
