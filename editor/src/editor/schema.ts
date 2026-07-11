import { Schema } from "prosemirror-model";

// Mirrors internal/transcription/blocks.go's Block/Token model directly: a
// "line" node's content is the literal interleaved text/chord token stream
// (chordMarker is an independent inline atom, not fused to the word it sits
// above) so lyrics are natively editable and chords can be moved, added, or
// removed without touching the surrounding text. The chord-over-word visual
// is produced by CSS positioning the marker's label above itself — see
// internal/web/transcription_render.go, which regroups this same token
// stream into wrappable word units for the public page independently of how
// it was authored here.
export const schema = new Schema({
  marks: {
    // Bold/italic/underline apply to lyric text only (see convert.ts) —
    // the underlying transcription.Token model only carries these marks
    // for ChordLyricPair/ChordOnlyLine tokens, not section headers (which
    // are already permanently bold via CSS) or title/artist/note text
    // lines, so the floating format toolbar (FormatToolbar.ts) only shows
    // up for selections inside a "line" node.
    strong: {
      toDOM: () => ["strong", 0],
      parseDOM: [{ tag: "strong" }, { tag: "b" }],
    },
    em: {
      toDOM: () => ["em", 0],
      parseDOM: [{ tag: "em" }, { tag: "i" }],
    },
    underline: {
      toDOM: () => ["u", 0],
      parseDOM: [{ tag: "u" }],
    },
  },
  nodes: {
    doc: { content: "block+" },

    section_header: {
      group: "block",
      content: "text*",
      // No marks: internal/transcription's SectionHeader Block only has a
      // plain Text string, nowhere to persist bold/italic/underline even
      // if the schema allowed applying them here.
      marks: "",
      toDOM: () => ["div", { class: "section-header" }, 0],
      parseDOM: [{ tag: "div.section-header" }],
    },

    line: {
      group: "block",
      content: "inline*",
      attrs: { annotation: { default: "" } },
      toDOM: (node) => [
        "div",
        { class: "line", "data-annotation": node.attrs.annotation as string },
        0,
      ],
      parseDOM: [
        {
          tag: "div.line",
          getAttrs: (dom) => ({
            annotation: (dom as HTMLElement).getAttribute("data-annotation") || "",
          }),
        },
      ],
    },

    chordMarker: {
      group: "inline",
      inline: true,
      atom: true,
      draggable: true,
      attrs: { chord: { default: "" } },
      toDOM: (node) => [
        "span",
        { class: "chord-marker" },
        ["span", { class: "chord-marker-label" }, node.attrs.chord as string],
      ],
      parseDOM: [
        {
          tag: "span.chord-marker",
          getAttrs: (dom) => ({
            chord: (dom as HTMLElement).querySelector(".chord-marker-label")?.textContent || "",
          }),
        },
      ],
    },

    text: { group: "inline" },
  },
});
