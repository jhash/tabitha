import { filterChords } from "./chords";
import { positionPopup, type PopupPositioner } from "./popupPosition";

interface OpenChordPickerOptions {
  anchor: DOMRect | (() => DOMRect);
  initialValue?: string;
  onSelect: (chord: string) => void;
  onCancel: () => void;
}

let closeActivePicker: (() => void) | null = null;

// Vanilla DOM popup (the editor's surface is an imperatively-managed
// ProseMirror EditorView, not a React tree, so node views and toolbar
// commands share this rather than mounting a React component per use).
export function openChordPicker(opts: OpenChordPickerOptions): () => void {
  closeActivePicker?.();

  const getAnchorRect = typeof opts.anchor === "function" ? opts.anchor : () => opts.anchor as DOMRect;

  const root = document.createElement("div");
  root.className = "chord-picker";
  root.style.position = "fixed";

  const input = document.createElement("input");
  input.type = "text";
  input.className = "chord-picker-input";
  input.placeholder = "Search chords…";
  input.value = opts.initialValue ?? "";
  root.appendChild(input);

  const list = document.createElement("ul");
  list.className = "chord-picker-list";
  root.appendChild(list);

  let highlighted = 0;
  let positioner: PopupPositioner;

  function renderList() {
    const matches = filterChords(input.value);
    list.innerHTML = "";
    matches.slice(0, 50).forEach((chord, i) => {
      const item = document.createElement("li");
      item.textContent = chord;
      item.className = i === highlighted ? "chord-picker-item highlighted" : "chord-picker-item";
      item.addEventListener("mousedown", (e) => {
        e.preventDefault();
        select(chord);
      });
      list.appendChild(item);
    });
    positioner?.reposition();
  }

  function select(chord: string) {
    cleanup();
    opts.onSelect(chord);
  }

  function cancel() {
    cleanup();
    opts.onCancel();
  }

  function cleanup() {
    document.removeEventListener("mousedown", onDocumentMouseDown, true);
    positioner.cleanup();
    root.remove();
    closeActivePicker = null;
  }

  function onDocumentMouseDown(e: MouseEvent) {
    if (!root.contains(e.target as Node)) cancel();
  }

  input.addEventListener("input", () => {
    highlighted = 0;
    renderList();
  });

  input.addEventListener("keydown", (e) => {
    const matches = filterChords(input.value).slice(0, 50);
    if (e.key === "ArrowDown") {
      e.preventDefault();
      highlighted = Math.min(highlighted + 1, matches.length - 1);
      renderList();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      highlighted = Math.max(highlighted - 1, 0);
      renderList();
    } else if (e.key === "Enter") {
      e.preventDefault();
      const chosen = matches[highlighted] ?? input.value.trim();
      if (chosen) select(chosen);
    } else if (e.key === "Escape") {
      e.preventDefault();
      cancel();
    }
  });

  document.body.appendChild(root);
  positioner = positionPopup(root, getAnchorRect);
  renderList();
  input.focus();
  input.select();

  // Deferred so the click that triggered the picker doesn't immediately close it.
  setTimeout(() => document.addEventListener("mousedown", onDocumentMouseDown, true), 0);

  closeActivePicker = cancel;
  return cancel;
}
