import { positionPopup, type PopupPositioner } from "./popupPosition";

interface ChordActionBarOptions {
  anchorEl: HTMLElement;
  chord: string;
  onChangeChord: () => void;
  onMoveLeft: () => void;
  onMoveRight: () => void;
  onDelete: () => void;
}

export interface ChordActionBarHandle {
  dom: HTMLElement;
  close: () => void;
}

let closeActiveBar: (() => void) | null = null;

// Floating toolbar shown above a selected chordMarker (see nodeViews.ts's
// selectNode/deselectNode) — the primary way to edit an existing chord in
// place, without needing the right-click context menu. Newly-inserted
// chords still go straight to the ChordPicker first (see commands.ts);
// this bar is for chords that already exist and are just selected/clicked.
// Returns its own root element too, so a caller (see nodeViews.ts) can
// anchor the ChordPicker to the bar itself when changing a chord — that
// keeps the bar visible instead of both popups fighting over the same
// small anchor.
export function openChordActionBar(opts: ChordActionBarOptions): ChordActionBarHandle {
  closeActiveBar?.();

  const root = document.createElement("div");
  root.className = "chord-action-bar";
  root.style.position = "fixed";

  const changeBtn = document.createElement("button");
  changeBtn.type = "button";
  changeBtn.className = "chord-action-chord";
  changeBtn.textContent = opts.chord || "+";
  changeBtn.title = "Change chord";
  changeBtn.addEventListener("mousedown", (e) => {
    e.preventDefault();
    opts.onChangeChord();
  });
  root.appendChild(changeBtn);

  const leftBtn = document.createElement("button");
  leftBtn.type = "button";
  leftBtn.className = "chord-action-move";
  leftBtn.textContent = "◀";
  leftBtn.title = "Move left";
  leftBtn.addEventListener("mousedown", (e) => {
    e.preventDefault();
    opts.onMoveLeft();
  });
  root.appendChild(leftBtn);

  const rightBtn = document.createElement("button");
  rightBtn.type = "button";
  rightBtn.className = "chord-action-move";
  rightBtn.textContent = "▶";
  rightBtn.title = "Move right";
  rightBtn.addEventListener("mousedown", (e) => {
    e.preventDefault();
    opts.onMoveRight();
  });
  root.appendChild(rightBtn);

  const deleteBtn = document.createElement("button");
  deleteBtn.type = "button";
  deleteBtn.className = "chord-action-delete";
  deleteBtn.textContent = "✕";
  deleteBtn.title = "Delete chord";
  deleteBtn.addEventListener("mousedown", (e) => {
    e.preventDefault();
    opts.onDelete();
  });
  root.appendChild(deleteBtn);

  document.body.appendChild(root);
  const positioner: PopupPositioner = positionPopup(
    root,
    () => opts.anchorEl.getBoundingClientRect(),
    { preferAbove: true },
  );

  function close() {
    positioner.cleanup();
    root.remove();
    closeActiveBar = null;
  }

  closeActiveBar = close;
  return { dom: root, close };
}
