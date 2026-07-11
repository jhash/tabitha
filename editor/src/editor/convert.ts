import type { Node as PMNode } from "prosemirror-model";
import type { Block, Token } from "./blocks";

interface ChordWord {
  chord: string;
  word: string;
}

// Mirrors internal/web/transcription_render.go's splitIntoChordWords: pairs
// each chord token with the next non-synthetic lyric word, splitting text
// tokens on whitespace so a token like "Hello world" becomes two words.
function tokensToChordWords(tokens: Token[]): ChordWord[] {
  const words: ChordWord[] = [];
  let pendingChord = "";
  let sawChord = false;

  for (const t of tokens) {
    if (t.chord) {
      if (sawChord) words.push({ chord: pendingChord, word: "" });
      pendingChord = t.chord;
      sawChord = true;
      continue;
    }
    if (t.synthetic || !t.text) continue;
    for (const w of t.text.split(/\s+/).filter(Boolean)) {
      words.push({ chord: pendingChord, word: w });
      pendingChord = "";
      sawChord = false;
    }
  }
  if (sawChord) words.push({ chord: pendingChord, word: "" });
  return words;
}

function chordWordsToTokens(words: ChordWord[]): Token[] {
  const tokens: Token[] = [];
  for (const w of words) {
    if (w.chord) tokens.push({ chord: w.chord });
    if (w.word) tokens.push({ text: w.word });
  }
  return tokens;
}

export function blocksToDocJSON(blocks: Block[]) {
  return {
    type: "doc",
    content: blocks.map(blockToNodeJSON),
  };
}

function blockToNodeJSON(block: Block) {
  switch (block.kind) {
    case "section_header":
      return {
        type: "section_header",
        content: block.text ? [{ type: "text", text: block.text }] : [],
      };
    case "text_line":
      return {
        type: "text_line",
        content: block.text ? [{ type: "text", text: block.text }] : [],
      };
    case "chord_lyric_pair":
    case "chord_only_line":
      return {
        type: "chord_line",
        attrs: { annotation: block.annotation ?? "" },
        content: tokensToChordWords(block.tokens ?? []).map((w) => ({
          type: "chordWord",
          attrs: w,
        })),
      };
  }
}

export function docNodeToBlocks(doc: PMNode): Block[] {
  const blocks: Block[] = [];
  doc.content.forEach((node) => {
    blocks.push(nodeToBlock(node));
  });
  return blocks;
}

function nodeToBlock(node: PMNode): Block {
  switch (node.type.name) {
    case "section_header":
      return { kind: "section_header", text: node.textContent };
    case "text_line":
      return { kind: "text_line", text: node.textContent };
    case "chord_line": {
      const words: ChordWord[] = [];
      node.content.forEach((child) => {
        words.push({ chord: child.attrs.chord as string, word: child.attrs.word as string });
      });
      const kind = words.some((w) => w.word) ? "chord_lyric_pair" : "chord_only_line";
      return {
        kind,
        tokens: chordWordsToTokens(words),
        annotation: (node.attrs.annotation as string) ?? "",
      };
    }
    default:
      throw new Error(`docNodeToBlocks: unknown node type ${node.type.name}`);
  }
}
