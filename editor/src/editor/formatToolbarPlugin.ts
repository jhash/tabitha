import { toggleMark } from "prosemirror-commands";
import { Plugin, TextSelection } from "prosemirror-state";
import type { MarkType } from "prosemirror-model";
import type { EditorView } from "prosemirror-view";
import { openFormatToolbar, type ToggleableMark } from "./FormatToolbar";
import { findEnclosingBlockDepth, toggleSectionHeader } from "./commands";

function markActive(view: EditorView, type: MarkType): boolean {
  const { from, to, empty, $from } = view.state.selection;
  if (empty) return !!type.isInSet(view.state.storedMarks ?? $from.marks());
  return view.state.doc.rangeHasMark(from, to, type);
}

// Shows FormatToolbar.ts above a non-empty text selection inside a lyric
// line — bold/italic/underline plus the section-header toggle. Closed for
// empty selections and for selections inside a section_header (marks
// aren't supported there — see schema.ts).
export function formatToolbarPlugin(): Plugin {
  return new Plugin({
    view(editorView) {
      let close: (() => void) | null = null;

      function update(view: EditorView) {
        const { state } = view;
        const { selection } = state;
        const { from, to, empty, $from } = selection;

        // Only a real (non-collapsed) text selection — a NodeSelection
        // (e.g. a clicked chordMarker, which the action bar already
        // handles) also reports empty === false, and would otherwise pop
        // this toolbar open on top of the chord action bar, stealing its
        // clicks. Shown inside a section_header too (not just "line") so
        // the H button can convert it back — bold/italic/underline just
        // silently no-op there since the schema disallows those marks on
        // section_header content.
        if (!(selection instanceof TextSelection) || empty || findEnclosingBlockDepth($from) === null) {
          close?.();
          close = null;
          return;
        }

        const startCoords = view.coordsAtPos(from);
        const endCoords = view.coordsAtPos(to);
        const rect = new DOMRect(
          Math.min(startCoords.left, endCoords.left),
          Math.min(startCoords.top, endCoords.top),
          Math.abs(endCoords.left - startCoords.left) || 1,
          Math.max(startCoords.bottom, endCoords.bottom) - Math.min(startCoords.top, endCoords.top),
        );

        close?.();
        close = openFormatToolbar({
          anchor: () => rect,
          isActive: (mark: ToggleableMark) => markActive(view, view.state.schema.marks[mark]),
          onToggle: (mark: ToggleableMark) => {
            toggleMark(view.state.schema.marks[mark])(view.state, view.dispatch);
            view.focus();
          },
          onToggleHeader: () => toggleSectionHeader(view),
        });
      }

      update(editorView);
      return {
        update,
        destroy: () => close?.(),
      };
    },
  });
}
