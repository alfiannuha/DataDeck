"use client";

import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
  type CompletionSource,
} from "@codemirror/autocomplete";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { bracketMatching, indentOnInput } from "@codemirror/language";
import { Compartment, EditorState } from "@codemirror/state";
import {
  drawSelection,
  dropCursor,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  placeholder as cmPlaceholder,
  rectangularSelection,
} from "@codemirror/view";
import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";

import { buildCompletions, filterCompletions } from "@/lib/sql/autocomplete";
import { languageForDialect, type SqlDialect } from "@/lib/sql/dialect";
import { executableSql } from "@/lib/sql/statements";
import {
  dataDeckEditorTheme,
  dataDeckSyntaxHighlighting,
} from "@/lib/sql/sql-theme";
import type { DatabaseSchemaTree } from "@/types/api";

export interface SqlEditorHandle {
  /** Move the cursor to a 1-based character position and focus the editor. */
  focusPosition: (position: number) => void;
}

export interface SqlEditorProps {
  value: string;
  onChange: (value: string) => void;
  /** Called with the SQL to run (selection, active statement, or document). */
  onExecute: (sql: string) => void;
  /** Cmd/Ctrl+S handler (DataDeck save action; prevents browser save). */
  onSave?: () => void;
  onSelectionChange?: (selection: { from: number; to: number }) => void;
  dialect?: SqlDialect;
  /** Introspection data used for table/column completion; may be undefined. */
  schema?: DatabaseSchemaTree[];
  placeholder?: string;
}

/** Completion source combining dialect keywords with connection-scoped schema. */
function createCompletionSource(
  getSchema: () => DatabaseSchemaTree[] | undefined,
  getDialect: () => SqlDialect,
): CompletionSource {
  return (context) => {
    const word = context.matchBefore(/[\w.]*/);
    if (!word || (word.from === word.to && !context.explicit)) return null;
    const options = filterCompletions(
      buildCompletions(getSchema(), getDialect()),
      word.text,
    );
    if (options.length === 0) return null;
    return {
      from: word.from,
      options: options.map((completion) => ({
        label: completion.label,
        type: completion.type,
        detail: completion.detail,
      })),
    };
  };
}

/**
 * Controlled CodeMirror 6 SQL editor. The view is created once; external value,
 * dialect and schema changes are applied without rebuilding the editor.
 */
export const SqlEditor = forwardRef<SqlEditorHandle, SqlEditorProps>(
  function SqlEditor(
    { value, onChange, onExecute, onSave, onSelectionChange, dialect = "postgres", schema, placeholder },
    ref,
  ) {
    const containerRef = useRef<HTMLDivElement>(null);
    const viewRef = useRef<EditorView | null>(null);
    const languageCompartment = useRef(new Compartment());

    const onChangeRef = useRef(onChange);
    const onExecuteRef = useRef(onExecute);
    const onSaveRef = useRef(onSave);
    const onSelectionChangeRef = useRef(onSelectionChange);
    const schemaRef = useRef(schema);
    const dialectRef = useRef(dialect);
    const initialValue = useRef(value);
    const initialDialect = useRef(dialect);
    const initialPlaceholder = useRef(placeholder);

    onChangeRef.current = onChange;
    onExecuteRef.current = onExecute;
    onSaveRef.current = onSave;
    onSelectionChangeRef.current = onSelectionChange;
    schemaRef.current = schema;
    dialectRef.current = dialect;

    useImperativeHandle(ref, () => ({
      focusPosition(position: number) {
        const view = viewRef.current;
        if (!view) return;
        const target = Math.min(
          Math.max(position - 1, 0),
          view.state.doc.length,
        );
        view.dispatch({
          selection: { anchor: target },
          scrollIntoView: true,
        });
        view.focus();
      },
    }));

    useEffect(() => {
      if (!containerRef.current) return;

      const view = new EditorView({
        parent: containerRef.current,
        state: EditorState.create({
          doc: initialValue.current,
          extensions: [
            lineNumbers(),
            highlightActiveLineGutter(),
            history(),
            drawSelection(),
            dropCursor(),
            EditorState.allowMultipleSelections.of(true),
            indentOnInput(),
            bracketMatching(),
            closeBrackets(),
            rectangularSelection(),
            highlightActiveLine(),
            autocompletion({
              override: [
                createCompletionSource(
                  () => schemaRef.current,
                  () => dialectRef.current,
                ),
              ],
            }),
            languageCompartment.current.of(
              languageForDialect(initialDialect.current),
            ),
            dataDeckEditorTheme,
            dataDeckSyntaxHighlighting,
            ...(initialPlaceholder.current
              ? [cmPlaceholder(initialPlaceholder.current)]
              : []),
            keymap.of([
              {
                key: "Mod-s",
                run: () => {
                  onSaveRef.current?.();
                  return true;
                },
              },
              {
                key: "Mod-Enter",
                run: (editorView) => {
                  const selection = editorView.state.selection.main;
                  const sql = executableSql(
                    editorView.state.doc.toString(),
                    { from: selection.from, to: selection.to },
                  );
                  if (sql.length > 0) onExecuteRef.current(sql);
                  return true;
                },
              },
              ...closeBracketsKeymap,
              ...defaultKeymap,
              ...historyKeymap,
              ...completionKeymap,
              indentWithTab,
            ]),
            EditorView.updateListener.of((update) => {
              if (update.docChanged) {
                onChangeRef.current(update.state.doc.toString());
              }
              if (update.selectionSet || update.docChanged) {
                const selection = update.state.selection.main;
                onSelectionChangeRef.current?.({
                  from: selection.from,
                  to: selection.to,
                });
              }
            }),
          ],
        }),
      });

      viewRef.current = view;
      return () => {
        view.destroy();
        viewRef.current = null;
      };
    }, []);

    useEffect(() => {
      const view = viewRef.current;
      if (!view) return;
      if (view.state.doc.toString() !== value) {
        view.dispatch({
          changes: { from: 0, to: view.state.doc.length, insert: value },
        });
      }
    }, [value]);

    useEffect(() => {
      viewRef.current?.dispatch({
        effects: languageCompartment.current.reconfigure(
          languageForDialect(dialect),
        ),
      });
    }, [dialect]);

    return (
      <div
        ref={containerRef}
        data-testid="sql-editor"
        className="h-full overflow-hidden"
      />
    );
  },
);
