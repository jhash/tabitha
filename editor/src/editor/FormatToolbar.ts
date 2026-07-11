import { positionPopup } from "./popupPosition";

export type ToggleableMark = "strong" | "em" | "underline";

interface FormatToolbarOptions {
  anchor: () => DOMRect;
  isActive: (mark: ToggleableMark) => boolean;
  onToggle: (mark: ToggleableMark) => void;
  onToggleHeader: () => void;
}

let closeActive: (() => void) | null = null;

// Floating toolbar shown above a non-empty text selection inside a lyric
// line (see formatToolbarPlugin.ts) — bold/italic/underline plus a toggle
// to turn the current line into a section header and back. Vanilla DOM,
// same pattern as ChordActionBar/ChordPicker (the editor surface is an
// imperatively-managed EditorView, not a React tree).
export function openFormatToolbar(opts: FormatToolbarOptions): () => void {
  closeActive?.();

  const root = document.createElement("div");
  root.className = "format-toolbar";
  root.style.position = "fixed";

  function addButton(label: string, title: string, active: boolean, onClick: () => void) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "format-toolbar-btn" + (active ? " format-toolbar-active" : "");
    btn.textContent = label;
    btn.title = title;
    // mousedown + preventDefault (not click) so the browser never collapses
    // the text selection before onToggle runs against it.
    btn.addEventListener("mousedown", (e) => {
      e.preventDefault();
      onClick();
    });
    root.appendChild(btn);
  }

  addButton("B", "Bold (Mod-B)", opts.isActive("strong"), () => opts.onToggle("strong"));
  addButton("I", "Italic (Mod-I)", opts.isActive("em"), () => opts.onToggle("em"));
  addButton("U", "Underline (Mod-U)", opts.isActive("underline"), () => opts.onToggle("underline"));
  addButton("H", "Toggle section header", false, opts.onToggleHeader);

  document.body.appendChild(root);
  const positioner = positionPopup(root, opts.anchor, { preferAbove: true });

  function close() {
    positioner.cleanup();
    root.remove();
    closeActive = null;
  }

  closeActive = close;
  return close;
}
