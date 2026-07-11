import { Schema } from "prosemirror-model";

// Mirrors internal/transcription/blocks.go's Block/Token model: each Block
// becomes a top-level node, and a chord_line's Tokens become a flat run of
// atomic chordWord leaves (chord stacked above its one lyric word), matching
// the flex-based chord-word rendering internal/web/transcription_render.go
// added this session (no column math — chords just sit above their word).
export const schema = new Schema({
  nodes: {
    doc: { content: "block+" },

    section_header: {
      group: "block",
      content: "text*",
      toDOM: () => ["div", { class: "section-header" }, 0],
      parseDOM: [{ tag: "div.section-header" }],
    },

    text_line: {
      group: "block",
      content: "text*",
      toDOM: () => ["div", { class: "text-line" }, 0],
      parseDOM: [{ tag: "div.text-line" }],
    },

    chord_line: {
      group: "block",
      content: "chordWord*",
      attrs: { annotation: { default: "" } },
      toDOM: (node) => [
        "div",
        { class: "chord-line", "data-annotation": node.attrs.annotation as string },
        0,
      ],
      parseDOM: [
        {
          tag: "div.chord-line",
          getAttrs: (dom) => ({
            annotation: (dom as HTMLElement).getAttribute("data-annotation") || "",
          }),
        },
      ],
    },

    chordWord: {
      group: "inline",
      inline: true,
      atom: true,
      attrs: { chord: { default: "" }, word: { default: "" } },
      toDOM: (node) => [
        "span",
        { class: "chord-word" },
        ["span", { class: "chord" }, node.attrs.chord as string],
        ["span", { class: "lyric" }, (node.attrs.word as string) + " "],
      ],
      parseDOM: [
        {
          tag: "span.chord-word",
          getAttrs: (dom) => {
            const el = dom as HTMLElement;
            return {
              chord: el.querySelector(".chord")?.textContent?.trim() || "",
              word: el.querySelector(".lyric")?.textContent?.trim() || "",
            };
          },
        },
      ],
    },

    text: { group: "inline" },
  },
});
