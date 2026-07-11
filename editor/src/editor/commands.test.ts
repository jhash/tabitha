import { EditorState, TextSelection } from "prosemirror-state";
import { describe, expect, it } from "vitest";
import { schema } from "./schema";
import {
  buildMoveChordMarkerTransaction,
  buildToggleSectionHeaderTransaction,
  findEnclosingLineDepth,
} from "./commands";

describe("findEnclosingLineDepth", () => {
  it("finds the line depth when the cursor is inside a line's text", () => {
    const doc = schema.node("doc", null, [
      schema.node("line", null, [schema.text("hello")]),
    ]);
    const $pos = doc.resolve(2);
    expect(findEnclosingLineDepth($pos)).toBe(1);
  });

  it("returns null when the cursor is inside a section_header", () => {
    const doc = schema.node("doc", null, [
      schema.node("section_header", null, [schema.text("CHORUS:")]),
    ]);
    const $pos = doc.resolve(2);
    expect(findEnclosingLineDepth($pos)).toBeNull();
  });
});

describe("buildMoveChordMarkerTransaction", () => {
  function stateWithLine(children: Parameters<typeof schema.node>[2]) {
    const doc = schema.node("doc", null, [schema.node("line", null, children)]);
    return EditorState.create({ schema, doc });
  }

  it("moving right into an adjacent text node hops exactly one character", () => {
    const state = stateWithLine([
      schema.nodes.chordMarker.create({ chord: "C" }),
      schema.text("hello world"),
    ]);
    // doc(0) line-open(pos0) -> content starts at pos1: marker at 1, text at 2..13
    const tr = buildMoveChordMarkerTransaction(state, 1, "right");
    expect(tr).not.toBeNull();
    const line = tr!.doc.firstChild!;
    expect(line.textContent).toBe("hello world");
    const kinds = childKinds(line);
    expect(kinds).toEqual(["text:h", "chordMarker", "text:ello world"]);
  });

  it("moving left into an adjacent text node hops exactly one character", () => {
    const state = stateWithLine([
      schema.text("hello world"),
      schema.nodes.chordMarker.create({ chord: "C" }),
    ]);
    // text at 1..12 (11 chars), marker at 12
    const tr = buildMoveChordMarkerTransaction(state, 12, "left");
    expect(tr).not.toBeNull();
    const line = tr!.doc.firstChild!;
    const kinds = childKinds(line);
    expect(kinds).toEqual(["text:hello worl", "chordMarker", "text:d"]);
  });

  it("blocks moving into an adjacent chordMarker instead of swapping", () => {
    const state = stateWithLine([
      schema.nodes.chordMarker.create({ chord: "A" }),
      schema.nodes.chordMarker.create({ chord: "B" }),
    ]);
    expect(buildMoveChordMarkerTransaction(state, 2, "left")).toBeNull();
  });

  it("returns null when there's nothing to move past (start of line)", () => {
    const state = stateWithLine([
      schema.nodes.chordMarker.create({ chord: "C" }),
      schema.text("hello"),
    ]);
    expect(buildMoveChordMarkerTransaction(state, 1, "left")).toBeNull();
  });

  it("returns null when there's nothing to move past (end of line)", () => {
    const state = stateWithLine([
      schema.text("hello"),
      schema.nodes.chordMarker.create({ chord: "C" }),
    ]);
    expect(buildMoveChordMarkerTransaction(state, 6, "right")).toBeNull();
  });

  it("moves left one character at a time, reaching the very front of the next word", () => {
    // Regression: character-level movement (rather than word-level) means
    // there's no whitespace/word-boundary special-casing at all — a chord
    // sitting right after "Don't " can walk all the way past it one
    // character per move, ending up flush against "tell" with nothing
    // stuck or skipped.
    const state = stateWithLine([
      schema.text("Don't "),
      schema.nodes.chordMarker.create({ chord: "Ebm" }),
      schema.text("tell me"),
    ]);
    // text "Don't " at pos 1..7, marker at 7
    let tr = buildMoveChordMarkerTransaction(state, 7, "left")!;
    expect(tr).not.toBeNull();
    expect(childKinds(tr.doc.firstChild!)).toEqual(["text:Don't", "chordMarker", "text: tell me"]);

    // Keep walking left through the remaining 5 characters of "Don't ".
    let curState = EditorState.create({ schema, doc: tr.doc });
    let markerPos = tr.selection.from;
    for (let i = 0; i < 5; i++) {
      tr = buildMoveChordMarkerTransaction(curState, markerPos, "left")!;
      expect(tr).not.toBeNull();
      curState = EditorState.create({ schema, doc: tr.doc });
      markerPos = tr.selection.from;
    }

    expect(curState.doc.firstChild!.textContent).toBe("Don't tell me");
    expect(childKinds(curState.doc.firstChild!)[0]).toBe("chordMarker");
    // Nothing left to move past.
    expect(buildMoveChordMarkerTransaction(curState, markerPos, "left")).toBeNull();
  });

  it("moves right one character at a time, reaching the very back of the previous word", () => {
    const state = stateWithLine([
      schema.text("tell me"),
      schema.nodes.chordMarker.create({ chord: "Db" }),
      schema.text(" it's"),
    ]);
    // text "tell me" at 1..8, marker at 8, " it's" at 9..14
    const tr = buildMoveChordMarkerTransaction(state, 8, "right")!;
    expect(tr).not.toBeNull();
    expect(childKinds(tr.doc.firstChild!)).toEqual(["text:tell me ", "chordMarker", "text:it's"]);
  });

  it("re-selects the moved marker as a node selection", () => {
    const state = stateWithLine([
      schema.nodes.chordMarker.create({ chord: "C" }),
      schema.text("hello world"),
    ]);
    const tr = buildMoveChordMarkerTransaction(state, 1, "right");
    expect(tr!.selection.$from.nodeAfter?.type.name).toBe("chordMarker");
  });
});

describe("buildToggleSectionHeaderTransaction", () => {
  it("converts a chordless line into a section header", () => {
    const doc = schema.node("doc", null, [schema.node("line", null, [schema.text("VERSE 1:")])]);
    const state = EditorState.create({ schema, doc, selection: TextSelection.create(doc, 2) });
    const tr = buildToggleSectionHeaderTransaction(state)!;
    expect(tr).not.toBeNull();
    expect(tr.doc.firstChild!.type.name).toBe("section_header");
    expect(tr.doc.firstChild!.textContent).toBe("VERSE 1:");
  });

  it("converts a section header back into a line", () => {
    const doc = schema.node("doc", null, [schema.node("section_header", null, [schema.text("VERSE 1:")])]);
    const state = EditorState.create({ schema, doc, selection: TextSelection.create(doc, 2) });
    const tr = buildToggleSectionHeaderTransaction(state)!;
    expect(tr).not.toBeNull();
    expect(tr.doc.firstChild!.type.name).toBe("line");
    expect(tr.doc.firstChild!.textContent).toBe("VERSE 1:");
  });

  it("refuses to convert a line containing a chordMarker into a header", () => {
    const doc = schema.node("doc", null, [
      schema.node("line", null, [schema.nodes.chordMarker.create({ chord: "G" }), schema.text("hi")]),
    ]);
    const state = EditorState.create({ schema, doc, selection: TextSelection.create(doc, 2) });
    expect(buildToggleSectionHeaderTransaction(state)).toBeNull();
  });

});

function childKinds(node: ReturnType<typeof schema.node>): string[] {
  const out: string[] = [];
  node.forEach((child) => {
    out.push(child.isText ? `text:${child.text}` : child.type.name);
  });
  return out;
}
