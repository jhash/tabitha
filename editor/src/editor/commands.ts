import { Fragment, type Node as PMNode, type ResolvedPos } from "prosemirror-model";
import { NodeSelection, type EditorState, type Transaction } from "prosemirror-state";
import type { EditorView } from "prosemirror-view";
import { openChordPicker } from "./ChordPicker";

// A chord can only be inserted inside a "line" block (section headers are
// plain text). Returns the depth of the enclosing line node, or null if
// $pos isn't inside one.
export function findEnclosingLineDepth($pos: ResolvedPos): number | null {
  for (let d = $pos.depth; d >= 0; d--) {
    if ($pos.node(d).type.name === "line") return d;
  }
  return null;
}

// Inserts an empty chordMarker at the given position (must be inside a
// line node) and immediately opens the picker so the user chooses its
// chord; cancelling removes the placeholder rather than leaving an empty
// chord behind. Shared by the toolbar button, the Mod-k keymap, and the
// "Insert chord here" context menu item.
export function insertChordAtPos(view: EditorView, pos: number): void {
  const $pos = view.state.doc.resolve(pos);
  if (findEnclosingLineDepth($pos) === null) return;

  const marker = view.state.schema.nodes.chordMarker.create({ chord: "" });
  view.dispatch(view.state.tr.insert(pos, marker));
  view.focus();

  requestAnimationFrame(() => {
    const coords = view.coordsAtPos(pos);
    const anchor = new DOMRect(coords.left, coords.top, 0, coords.bottom - coords.top);
    openChordPicker({
      anchor,
      onSelect: (chord) => {
        view.dispatch(view.state.tr.setNodeAttribute(pos, "chord", chord));
        view.focus();
      },
      onCancel: () => {
        view.dispatch(view.state.tr.delete(pos, pos + marker.nodeSize));
        view.focus();
      },
    });
  });
}

export function insertChordAtSelection(view: EditorView): void {
  insertChordAtPos(view, view.state.selection.from);
}

// Deletes the chordMarker at pos.
export function deleteChordMarker(view: EditorView, pos: number): void {
  const marker = view.state.doc.nodeAt(pos);
  if (!marker || marker.type.name !== "chordMarker") return;
  view.dispatch(view.state.tr.delete(pos, pos + marker.nodeSize));
  view.focus();
}

interface LineChild {
  node: PMNode;
  pos: number;
}

function lineChildrenWithPos($pos: ResolvedPos, lineDepth: number): LineChild[] {
  const line = $pos.node(lineDepth);
  const lineStart = $pos.before(lineDepth) + 1;
  const children: LineChild[] = [];
  let pos = lineStart;
  line.forEach((child) => {
    children.push({ node: child, pos });
    pos += child.nodeSize;
  });
  return children;
}

// Pure: builds the transaction that moves the chordMarker at markerPos one
// character left/right, or null if the move should be blocked — only at
// the start/end of the line, or when the adjacent sibling is another
// chord (two chords swapping places over the same word isn't meaningful,
// so that's a block rather than a swap). Moving one character at a time
// (rather than one word) means there's no word-boundary logic to get
// wrong — any position, including the front or back of a word, is always
// reachable. Re-selects the moved marker so the floating action bar (see
// nodeViews.ts) follows it.
export function buildMoveChordMarkerTransaction(
  state: EditorState,
  markerPos: number,
  dir: "left" | "right",
): Transaction | null {
  const $pos = state.doc.resolve(markerPos);
  const lineDepth = findEnclosingLineDepth($pos);
  if (lineDepth === null) return null;

  const children = lineChildrenWithPos($pos, lineDepth);
  const idx = children.findIndex((c) => c.pos === markerPos);
  if (idx === -1) return null;
  const marker = children[idx].node;

  const sibIdx = dir === "left" ? idx - 1 : idx + 1;
  if (sibIdx < 0 || sibIdx >= children.length) return null;
  const sib = children[sibIdx];
  if (sib.node.type.name === "chordMarker") return null;

  const text = sib.node.text ?? "";

  let from: number;
  let to: number;
  let nodes: PMNode[];
  let newMarkerPos: number;

  if (dir === "left") {
    from = sib.pos;
    to = markerPos + marker.nodeSize;

    const prefix = text.slice(0, -1);
    const lastChar = text.slice(-1);
    if (!prefix) {
      nodes = [marker, sib.node];
      newMarkerPos = from;
    } else {
      nodes = [state.schema.text(prefix), marker, state.schema.text(lastChar)];
      newMarkerPos = from + prefix.length;
    }
  } else {
    from = markerPos;
    to = sib.pos + sib.node.nodeSize;

    const firstChar = text.slice(0, 1);
    const rest = text.slice(1);
    if (!rest) {
      nodes = [sib.node, marker];
      newMarkerPos = from + sib.node.nodeSize;
    } else {
      nodes = [state.schema.text(firstChar), marker, state.schema.text(rest)];
      newMarkerPos = from + firstChar.length;
    }
  }

  const tr = state.tr.replaceWith(from, to, Fragment.fromArray(nodes));
  tr.setSelection(NodeSelection.create(tr.doc, newMarkerPos));
  return tr;
}

export function moveChordMarker(view: EditorView, markerPos: number, dir: "left" | "right"): void {
  const tr = buildMoveChordMarkerTransaction(view.state, markerPos, dir);
  if (!tr) return;
  view.dispatch(tr);
  view.focus();
}

// Both "line" and "section_header" are direct children of "doc" — this
// finds whichever one $pos is inside, regardless of which.
export function findEnclosingBlockDepth($pos: ResolvedPos): number | null {
  for (let d = $pos.depth; d >= 0; d--) {
    const name = $pos.node(d).type.name;
    if (name === "line" || name === "section_header") return d;
  }
  return null;
}

// Pure: builds the transaction that converts the block containing $pos
// between "line" and "section_header", or null if there's nothing to
// convert (not inside either block type) or the conversion isn't valid —
// specifically, a line containing a chordMarker can't become a section
// header, since section_header's content ("text*") has nowhere to put a
// chord and internal/transcription's SectionHeader Block is plain text
// with no token stream at all.
export function buildToggleSectionHeaderTransaction(state: EditorState): Transaction | null {
  const $pos = state.selection.$from;
  const blockDepth = findEnclosingBlockDepth($pos);
  if (blockDepth === null) return null;

  const pos = $pos.before(blockDepth);
  const node = $pos.node(blockDepth);

  if (node.type.name === "section_header") {
    return state.tr.setNodeMarkup(pos, state.schema.nodes.line, { annotation: "" });
  }

  let hasChord = false;
  node.forEach((child) => {
    if (child.type.name === "chordMarker") hasChord = true;
  });
  if (hasChord) return null;

  return state.tr.setNodeMarkup(pos, state.schema.nodes.section_header);
}

export function toggleSectionHeader(view: EditorView): void {
  const tr = buildToggleSectionHeaderTransaction(view.state);
  if (!tr) return;
  view.dispatch(tr);
  view.focus();
}
