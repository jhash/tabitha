import { describe, expect, it } from "vitest";
import { schema } from "./schema";
import { blocksToDocJSON, docNodeToBlocks } from "./convert";
import type { Block } from "./blocks";

describe("blocksToDocJSON / docNodeToBlocks", () => {
  it("round-trips a section header", () => {
    const blocks: Block[] = [{ kind: "section_header", text: "CHORUS:" }];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual(blocks);
  });

  it("round-trips a plain text line", () => {
    const blocks: Block[] = [{ kind: "text_line", text: "(repeat chorus)" }];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual(blocks);
  });

  it("maps a chord_lyric_pair's tokens directly to chordMarker/text inline nodes, in order", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [{ chord: "G" }, { text: "Hello " }, { chord: "D" }, { text: "world" }],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    const line = doc.content.child(0);
    expect(line.type.name).toBe("line");
    expect(line.childCount).toBe(4);
    expect(line.child(0).type.name).toBe("chordMarker");
    expect(line.child(0).attrs).toEqual({ chord: "G" });
    expect(line.child(1).type.name).toBe("text");
    expect(line.child(1).text).toBe("Hello ");
    expect(line.child(2).attrs).toEqual({ chord: "D" });
    expect(line.child(3).text).toBe("world");
  });

  it("preserves a mid-word chord placement exactly (no word-boundary snapping)", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [{ text: "kno" }, { chord: "Eb" }, { text: "w" }],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual([
      {
        kind: "chord_lyric_pair",
        tokens: [{ text: "kno" }, { chord: "Eb" }, { text: "w" }],
        annotation: "",
      },
    ]);
  });

  it("drops synthetic alignment-padding text tokens on load", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [{ chord: "G" }, { text: "   ", synthetic: true }, { chord: "D" }, { text: "go" }],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    const line = doc.content.child(0);
    expect(line.childCount).toBe(3);
    expect(line.child(0).attrs).toEqual({ chord: "G" });
    expect(line.child(1).attrs).toEqual({ chord: "D" });
    expect(line.child(2).text).toBe("go");
  });

  it("preserves a chord line's trailing annotation", () => {
    const blocks: Block[] = [
      { kind: "chord_only_line", tokens: [{ chord: "G" }], annotation: "  3rd x: Girl reaction" },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)[0].annotation).toBe("  3rd x: Girl reaction");
  });

  it("round-trips a chord_only_line with no lyric words back through docNodeToBlocks", () => {
    const blocks: Block[] = [
      { kind: "chord_only_line", tokens: [{ chord: "Em" }, { chord: "G" }] },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual([
      { kind: "chord_only_line", tokens: [{ chord: "Em" }, { chord: "G" }], annotation: "" },
    ]);
  });

  it("round-trips a chord_lyric_pair back through docNodeToBlocks, merging adjacent text tokens", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [{ chord: "G" }, { text: "Hello " }, { text: "world" }],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual([
      {
        kind: "chord_lyric_pair",
        tokens: [{ chord: "G" }, { text: "Hello world" }],
        annotation: "",
      },
    ]);
  });

  it("round-trips bold/italic/underline marks on a chord line's tokens", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [
          { chord: "G" },
          { text: "bold", bold: true },
          { text: " italic", italic: true },
          { text: " under", underline: true },
          { text: " plain" },
        ],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual([
      {
        kind: "chord_lyric_pair",
        tokens: [
          { chord: "G" },
          { text: "bold", bold: true },
          { text: " italic", italic: true },
          { text: " under", underline: true },
          { text: " plain" },
        ],
        annotation: "",
      },
    ]);
  });

  it("round-trips marks on a text_line via its optional tokens, without disturbing plain text_line's simpler shape", () => {
    const marked: Block[] = [
      { kind: "text_line", text: "Bryan Adams", tokens: [{ text: "Bryan Adams", bold: true }] },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(marked));
    expect(docNodeToBlocks(doc)).toEqual(marked);

    const plain: Block[] = [{ kind: "text_line", text: "(repeat chorus)" }];
    const plainDoc = schema.nodeFromJSON(blocksToDocJSON(plain));
    expect(docNodeToBlocks(plainDoc)).toEqual(plain);
  });

  it("derives text_line (no chords) from a line node with only text after edits strip all chords", () => {
    const blocks: Block[] = [{ kind: "chord_lyric_pair", tokens: [{ text: "just words" }] }];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual([{ kind: "text_line", text: "just words" }]);
  });
});
