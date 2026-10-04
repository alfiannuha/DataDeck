"use client";

import { AlertTriangle, Database, Loader2, Play, Save, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { SqlEditor, type SqlEditorHandle } from "@/components/editor/sql-editor";
import { SaveQueryDialog } from "@/components/shared/save-query-dialog";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/use-connections";
import { useDatabases } from "@/hooks/use-databases";
import { useOnline } from "@/hooks/use-online";
import { useRunQuery } from "@/hooks/use-run-query";
import { useSchema } from "@/hooks/use-schema";
import type { SqlDialect } from "@/lib/sql/dialect";
import { executableSql } from "@/lib/sql/statements";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * SQL editor region. Owns the Run control and error banner. Execution always
 * targets the tab's bound connection (falling back to the active connection
 * only for an unbound tab, which is then bound on first run) — never the
 * global active connection for a tab bound elsewhere.
 */
export function EditorPanel() {
  const activeTab = useWorkspaceStore(
    (state) =>
      state.tabs.find((tab) => tab.id === state.activeTabId) ?? state.tabs[0],
  );
  const updateActiveSql = useWorkspaceStore((state) => state.updateActiveSql);
  const setTabConnection = useWorkspaceStore(
    (state) => state.setTabConnection,
  );
  const setTabDatabase = useWorkspaceStore((state) => state.setTabDatabase);
  const setTabSavedQuery = useWorkspaceStore(
    (state) => state.setTabSavedQuery,
  );
  const markActiveClean = useWorkspaceStore((state) => state.markActiveClean);
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const { data: connections } = useConnections();
  const tabDatabase = activeTab?.database ?? null;
  const schemaConnectionId = activeTab?.connectionId ?? activeConnectionId;
  // Autocomplete/schema must follow the tab's connection AND database, not the
  // global selection, so suggestions never come from another database.
  const { data: schema } = useSchema(schemaConnectionId, tabDatabase);
  const { run, cancel } = useRunQuery();
  const online = useOnline();

  const status = useExecutionStore((state) => state.status);
  const error = useExecutionStore((state) => state.error);
  const dismissError = useExecutionStore((state) => state.dismissError);
  const isRunning = status === "running";

  const selectionRef = useRef({ from: 0, to: 0 });
  const editorRef = useRef<SqlEditorHandle>(null);
  const [saveOpen, setSaveOpen] = useState(false);

  const boundConnectionId = activeTab?.connectionId ?? null;
  const effectiveConnectionId = boundConnectionId ?? activeConnectionId;
  const connection = connections?.find(
    (candidate) => candidate.id === effectiveConnectionId,
  );
  // Server-level PostgreSQL profiles select a database per tab.
  const serverLevel =
    connection?.driver === "postgres" && !connection?.database_name;
  const { data: databases } = useDatabases(
    serverLevel ? schemaConnectionId : null,
    serverLevel,
  );
  const connectionReady =
    Boolean(effectiveConnectionId) && connection !== undefined;
  const hasSql = (activeTab?.sql.trim().length ?? 0) > 0;
  const canRun = !isRunning && connectionReady && hasSql && online;

  // The editor dialect follows the tab's/active connection's driver.
  const dialect: SqlDialect =
    connection?.driver === "mysql"
      ? "mysql"
      : connection?.driver === "sqlite"
        ? "sqlite"
        : "postgres";

  function rejectLocal(code: string, message: string) {
    useExecutionStore.getState().reject({ code, message });
  }

  function runSql(sql: string) {
    if (isRunning) return;
    if (!online) {
      rejectLocal(
        "OFFLINE",
        "Backend unavailable. Reconnect before running a query.",
      );
      return;
    }
    if (!effectiveConnectionId) {
      rejectLocal(
        "NO_ACTIVE_CONNECTION",
        "Select a connection before running a query.",
      );
      return;
    }
    if (!connectionReady) {
      rejectLocal(
        "CONNECTION_NOT_AVAILABLE",
        "This tab's connection is no longer available. Select another connection.",
      );
      return;
    }
    // Bind an unbound tab on first run so later connection switches do not
    // silently change this tab's execution context.
    if (activeTab && !activeTab.connectionId) {
      setTabConnection(activeTab.id, effectiveConnectionId);
    }
    if (sql.trim().length > 0) {
      void run(sql, effectiveConnectionId, activeTab?.id, tabDatabase);
    }
  }

  function runCurrent() {
    runSql(executableSql(activeTab?.sql ?? "", selectionRef.current));
  }

  function openSave() {
    if ((activeTab?.sql.trim().length ?? 0) > 0) {
      setSaveOpen(true);
    }
  }

  // Keep the latest handlers available to the window-level shortcut listener
  // without re-binding on every render.
  const runCurrentRef = useRef(runCurrent);
  const openSaveRef = useRef(openSave);
  const saveOpenRef = useRef(saveOpen);
  runCurrentRef.current = runCurrent;
  openSaveRef.current = openSave;
  saveOpenRef.current = saveOpen;

  // Opening or switching a tab shows a fresh surface: never display the previous
  // tab's stale execution status, result or error.
  const activeTabId = activeTab?.id ?? null;
  useEffect(() => {
    useExecutionStore.getState().reset();
  }, [activeTabId]);

  // Global Mod+Enter / Mod+S consistency when focus is outside the editor (the
  // CodeMirror keymap already owns these while the editor is focused).
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!(event.metaKey || event.ctrlKey)) return;
      const target = event.target as HTMLElement | null;
      if (target?.closest?.('[data-testid="sql-editor"]')) return;
      if (
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.tagName === "SELECT" ||
          target.isContentEditable)
      ) {
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        runCurrentRef.current();
      } else if (event.key.toLowerCase() === "s") {
        event.preventDefault();
        if (!saveOpenRef.current) openSaveRef.current();
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <section
      aria-label="SQL editor"
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
    >
      <div className="flex h-8 shrink-0 items-center justify-between border-b border-border bg-panel pl-3">
        <div className="flex min-w-0 items-center gap-2">
          <span className="text-xs font-medium uppercase tracking-wide text-subtle-foreground">
            Editor
          </span>
          <span
            className="flex min-w-0 items-center gap-1 text-[10px] text-muted-foreground"
            title={
              connection
                ? `This tab runs against ${connection.name}`
                : "No connection bound to this tab"
            }
          >
            <Database size={10} aria-hidden="true" className="shrink-0" />
            <select
              aria-label="Tab connection"
              value={activeTab?.connectionId ?? ""}
              onChange={(event) =>
                activeTab &&
                setTabConnection(activeTab.id, event.target.value || null)
              }
              className="max-w-40 truncate rounded border border-border bg-background px-1 py-0.5 text-[10px] text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-focus-ring)]"
            >
              <option value="">Active connection</option>
              {connections?.map((candidate) => (
                <option key={candidate.id} value={candidate.id}>
                  {candidate.name}
                </option>
              ))}
            </select>
            {serverLevel ? (
              <>
                <span aria-hidden="true" className="text-subtle-foreground">
                  /
                </span>
                <select
                  aria-label="Tab database"
                  value={tabDatabase ?? ""}
                  onChange={(event) =>
                    activeTab &&
                    setTabDatabase(activeTab.id, event.target.value || null)
                  }
                  className="max-w-40 truncate rounded border border-border bg-background px-1 py-0.5 text-[10px] text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-focus-ring)]"
                >
                  <option value="">Select database</option>
                  {databases?.map((database) => (
                    <option key={database.name} value={database.name}>
                      {database.name}
                    </option>
                  ))}
                </select>
              </>
            ) : connection?.database_name ? (
              <span className="truncate text-subtle-foreground">
                / {connection.database_name}
              </span>
            ) : null}
          </span>
          <span
            data-testid="query-context"
            aria-label="Query context"
            title="This is where SQL from this tab will execute"
            className="shrink-0 rounded bg-panel-raised px-1.5 py-0.5 text-[10px] text-subtle-foreground"
          >
            {connection?.name ?? "No connection"}
            {" / "}
            {tabDatabase ?? connection?.database_name ?? (serverLevel ? "no database" : "default")}
          </span>
        </div>
        <div className="flex items-center gap-1 pr-1">
          {isRunning && (
            <span
              role="status"
              className="flex items-center gap-1 text-[10px] text-muted-foreground"
            >
              <Loader2 size={11} className="animate-spin" aria-hidden="true" />
              Running…
            </span>
          )}
          {isRunning && (
            <Button variant="ghost" size="sm" onClick={cancel}>
              Cancel
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            aria-label="Save query"
            title="Save query (Cmd/Ctrl+S)"
            onClick={openSave}
          >
            <Save size={13} aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Run query"
            title="Run query (Cmd/Ctrl+Enter)"
            onClick={runCurrent}
            disabled={!canRun}
          >
            <Play size={13} aria-hidden="true" />
          </Button>
        </div>
      </div>

      {error && (
        <div
          role="alert"
          className="flex items-center gap-2 border-b border-border bg-destructive/10 px-3 py-1.5 text-xs text-destructive"
        >
          <AlertTriangle size={12} aria-hidden="true" />
          <span className="truncate">
            {error.code}: {error.message}
            {error.position ? ` (position ${error.position})` : ""}
          </span>
          {error.position ? (
            <button
              type="button"
              className="ml-auto shrink-0 underline underline-offset-2"
              onClick={() => editorRef.current?.focusPosition(error.position!)}
            >
              Go to error
            </button>
          ) : null}
          <button
            type="button"
            aria-label="Dismiss error"
            className={
              error.position
                ? "shrink-0 rounded p-0.5 hover:bg-destructive/20"
                : "ml-auto shrink-0 rounded p-0.5 hover:bg-destructive/20"
            }
            onClick={dismissError}
          >
            <X size={12} aria-hidden="true" />
          </button>
        </div>
      )}

      <div className="min-h-0 flex-1 bg-editor-surface">
        <SqlEditor
          ref={editorRef}
          value={activeTab?.sql ?? ""}
          onChange={updateActiveSql}
          onExecute={runSql}
          onSelectionChange={(selection) => {
            selectionRef.current = selection;
          }}
          onSave={openSave}
          dialect={dialect}
          schema={schema}
          placeholder="Write SQL here. Press Cmd/Ctrl+Enter to run the selection or current statement."
        />
      </div>

      <SaveQueryDialog
        open={saveOpen}
        onOpenChange={setSaveOpen}
        sql={activeTab?.sql ?? ""}
        connectionId={effectiveConnectionId}
        existing={
          activeTab?.savedQueryId
            ? {
                id: activeTab.savedQueryId,
                title: activeTab.savedTitle ?? "",
                tags: activeTab.savedTags ?? null,
              }
            : null
        }
        onSaved={(saved) => {
          if (activeTab) {
            setTabSavedQuery(activeTab.id, {
              id: saved.id ?? null,
              title: saved.title ?? null,
              tags: saved.tags ?? null,
            });
            markActiveClean();
          }
          setSaveOpen(false);
        }}
      />
    </section>
  );
}
