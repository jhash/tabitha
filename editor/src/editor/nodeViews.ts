import type { Node as PMNode } from "prosemirror-model";
import type { EditorView, NodeView } from "prosemirror-view";
import { openChordActionBar, type ChordActionBarHandle } from "./ChordActionBar";
import { openChordPicker } from "./ChordPicker";
import { openContextMenu } from "./ContextMenu";
import { moveChordMarker } from "./commands";

// chordMarker is an independent inline atom (see schema.ts) — this is what
// makes it draggable/selectable/deletable on its own, separate from the
// lyric text around it. Click or right-click to change/remove the chord;
// native HTML5 drag (draggable=true) lets ProseMirror move it in the doc.
export function chordMarkerNodeView(
  initialNode: PMNode,
  view: EditorView,
  getPos: () => number | undefined,
): NodeView {
  let node = initialNode;
  let bar: ChordActionBarHandle | null = null;

  const dom = document.createElement("span");
  dom.className = "chord-marker";
  dom.draggable = true;

  const label = document.createElement("span");
  label.className = "chord-marker-label";
  dom.appendChild(label);

  function render() {
    label.textContent = node.attrs.chord || "+";
    dom.classList.toggle("chord-marker-empty", !node.attrs.chord);
  }
  render();

  function closeBar() {
    bar?.close();
    bar = null;
  }

  // anchorEl/keepBarOpen let the action bar's "change chord" button open
  // the picker anchored to the BAR itself (see openBar below) instead of
  // the marker, and without closing the bar first — that's what keeps the
  // bar visible while a new chord is being picked. The context menu (no
  // bar involved) just uses the defaults.
  function openPicker(opts?: { anchorEl?: HTMLElement; keepBarOpen?: boolean }) {
    const pos = getPos();
    if (pos === undefined) return;
    if (!opts?.keepBarOpen) closeBar();
    openChordPicker({
      anchor: () => (opts?.anchorEl ?? dom).getBoundingClientRect(),
      initialValue: node.attrs.chord,
      onSelect: (chord) => {
        view.dispatch(view.state.tr.setNodeAttribute(pos, "chord", chord));
        view.focus();
        openBar();
      },
      onCancel: () => {
        view.focus();
        openBar();
      },
    });
  }

  function remove() {
    const pos = getPos();
    if (pos === undefined) return;
    closeBar();
    view.dispatch(view.state.tr.delete(pos, pos + node.nodeSize));
    view.focus();
  }

  function move(dir: "left" | "right") {
    const pos = getPos();
    if (pos === undefined) return;
    moveChordMarker(view, pos, dir);
    // ProseMirror reuses this same NodeView instance across the move
    // (the chordMarker's markup — its attrs — didn't change), and its
    // selectNode/deselectNode hooks only fire on a selection *transition*.
    // Since the marker stays continuously selected before and after the
    // move, they never fire again here, so the bar has to be reopened
    // explicitly rather than left to selectNode.
    openBar();
  }

  // The floating action bar (see ChordActionBar.ts) is how an *existing*
  // chord gets edited/moved/deleted — it opens whenever this marker becomes
  // the NodeSelection (selectNode below), which ProseMirror does natively
  // when you click an atomic node, no click handler needed here. A
  // newly-inserted chord instead goes straight to the picker (see
  // commands.ts's insertChordAtPos) so the first pick still feels immediate.
  // Anchored to the label (not the zero-width marker span, whose own box
  // is inflated by the chorded line's larger line-height) so the bar sits
  // fully above the chord instead of overlapping it or the text below.
  function openBar() {
    closeBar();
    bar = openChordActionBar({
      anchorEl: label,
      chord: node.attrs.chord,
      onChangeChord: () => openPicker({ anchorEl: bar!.dom, keepBarOpen: true }),
      onMoveLeft: () => move("left"),
      onMoveRight: () => move("right"),
      onDelete: remove,
    });
  }

  dom.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    e.stopPropagation();
    closeBar();
    openContextMenu(e.clientX, e.clientY, [
      { label: "Change chord", onSelect: () => openPicker() },
      { label: "Delete chord", onSelect: remove },
    ]);
  });

  return {
    dom,
    update(updated) {
      if (updated.type.name !== "chordMarker") return false;
      node = updated;
      render();
      return true;
    },
    selectNode() {
      dom.classList.add("chord-marker-selected");
      openBar();
    },
    deselectNode() {
      dom.classList.remove("chord-marker-selected");
      closeBar();
    },
  };
}

// line's editable inline content (text + chordMarker) lives in contentDOM;
// the annotation field is a plain input living outside it, so typing in it
// never touches ProseMirror's own selection/content handling.
export function lineNodeView(
  initialNode: PMNode,
  view: EditorView,
  getPos: () => number | undefined,
): NodeView {
  let node = initialNode;

  const dom = document.createElement("div");
  dom.className = "line";

  const content = document.createElement("span");
  content.className = "line-content";
  dom.appendChild(content);

  function hasChordMarker(n: PMNode): boolean {
    let found = false;
    n.content.forEach((child) => {
      if (child.type.name === "chordMarker") found = true;
    });
    return found;
  }

  function syncHasChordsClass(n: PMNode) {
    dom.classList.toggle("has-chords", hasChordMarker(n));
  }
  syncHasChordsClass(node);

  const annotationInput = document.createElement("input");
  annotationInput.type = "text";
  annotationInput.className = "line-annotation";
  annotationInput.placeholder = "note";
  annotationInput.value = node.attrs.annotation || "";
  dom.appendChild(annotationInput);

  annotationInput.addEventListener("mousedown", (e) => e.stopPropagation());
  annotationInput.addEventListener("keydown", (e) => e.stopPropagation());
  annotationInput.addEventListener("input", () => {
    const pos = getPos();
    if (pos === undefined) return;
    view.dispatch(view.state.tr.setNodeAttribute(pos, "annotation", annotationInput.value));
  });

  return {
    dom,
    contentDOM: content,
    update(updated) {
      if (updated.type.name !== "line") return false;
      node = updated;
      if (document.activeElement !== annotationInput) {
        annotationInput.value = node.attrs.annotation || "";
      }
      syncHasChordsClass(node);
      return true;
    },
  };
}
