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

  it("splits a chord_lyric_pair's tokens into chord-word pairs", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [
          { chord: "G" },
          { text: "Hello " },
          { chord: "D" },
          { text: "world" },
        ],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    const chordLine = doc.content.child(0);
    expect(chordLine.type.name).toBe("chord_line");
    expect(chordLine.childCount).toBe(2);
    expect(chordLine.child(0).attrs).toEqual({ chord: "G", word: "Hello" });
    expect(chordLine.child(1).attrs).toEqual({ chord: "D", word: "world" });
  });

  it("attaches a chord with no following lyric word to an empty word", () => {
    const blocks: Block[] = [
      { kind: "chord_only_line", tokens: [{ chord: "Em" }, { chord: "G" }] },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    const chordLine = doc.content.child(0);
    expect(chordLine.childCount).toBe(2);
    expect(chordLine.child(0).attrs).toEqual({ chord: "Em", word: "" });
    expect(chordLine.child(1).attrs).toEqual({ chord: "G", word: "" });
  });

  it("preserves a chord line's trailing annotation", () => {
    const blocks: Block[] = [
      {
        kind: "chord_only_line",
        tokens: [{ chord: "G" }],
        annotation: "  3rd x: Girl reaction",
      },
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

  it("round-trips a chord_lyric_pair back through docNodeToBlocks", () => {
    const blocks: Block[] = [
      {
        kind: "chord_lyric_pair",
        tokens: [{ chord: "G" }, { text: "Hello" }, { text: "world" }],
      },
    ];
    const doc = schema.nodeFromJSON(blocksToDocJSON(blocks));
    expect(docNodeToBlocks(doc)).toEqual([
      {
        kind: "chord_lyric_pair",
        tokens: [{ chord: "G" }, { text: "Hello" }, { text: "world" }],
        annotation: "",
      },
    ]);
  });
});
