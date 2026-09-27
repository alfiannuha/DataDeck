import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags } from "@lezer/highlight";

/**
 * Dark editor theme wired to the DataDeck design tokens (CSS variables), so the
 * editor follows the same palette as the rest of the workspace.
 */
export const dataDeckEditorTheme = EditorView.theme(
  {
    "&": {
      height: "100%",
      backgroundColor: "var(--color-editor-surface)",
      color: "var(--color-foreground)",
    },
    ".cm-scroller": { fontFamily: "var(--font-mono, monospace)", fontSize: "13px" },
    ".cm-content": { caretColor: "var(--color-accent)" },
    ".cm-gutters": {
      backgroundColor: "var(--color-editor-surface)",
      color: "var(--color-subtle-foreground)",
      border: "none",
    },
    ".cm-activeLine": { backgroundColor: "rgba(255, 255, 255, 0.03)" },
    ".cm-activeLineGutter": { backgroundColor: "rgba(255, 255, 255, 0.03)" },
    "&.cm-focused": { outline: "none" },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
      { backgroundColor: "rgba(59, 130, 246, 0.35) !important" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--color-accent)" },
    ".cm-tooltip": {
      backgroundColor: "var(--color-panel-raised)",
      border: "1px solid var(--color-border)",
      color: "var(--color-foreground)",
    },
    ".cm-tooltip-autocomplete ul li[aria-selected]": {
      backgroundColor: "var(--color-accent)",
      color: "var(--color-accent-foreground)",
    },
  },
  { dark: true },
);

export const dataDeckHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: "#7dd3fc" },
  { tag: tags.string, color: "#86efac" },
  { tag: [tags.number, tags.bool, tags.null], color: "#fca5a5" },
  { tag: tags.comment, color: "#71717a", fontStyle: "italic" },
  { tag: tags.operator, color: "#c4b5fd" },
  { tag: [tags.typeName, tags.className], color: "#fcd34d" },
  { tag: [tags.variableName, tags.propertyName], color: "#e4e4e7" },
]);

export const dataDeckSyntaxHighlighting = syntaxHighlighting(
  dataDeckHighlightStyle,
);
