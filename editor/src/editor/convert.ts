import type { Node as PMNode } from "prosemirror-model";
import type { Block, Token } from "./blocks";

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
        type: "line",
        // Tokens (marks-aware) take priority when present; older/plain
        // text lines fall back to the flat string (see blocks.go).
        content: block.tokens?.length
          ? tokensToInlineJSON(block.tokens)
          : block.text
            ? [{ type: "text", text: block.text }]
            : [],
      };
    case "chord_lyric_pair":
    case "chord_only_line":
      return {
        type: "line",
        attrs: { annotation: block.annotation ?? "" },
        content: tokensToInlineJSON(block.tokens ?? []),
      };
  }
}

// Direct 1:1 mapping onto Go's Token stream — no word-boundary grouping, so
// a chord landing mid-word (e.g. "kno" + chord + "w") round-trips exactly.
// Synthetic alignment padding is dropped: it's meaningless once nothing
// does column math over the token stream.
function tokensToInlineJSON(tokens: Token[]) {
  const content: unknown[] = [];
  for (const t of tokens) {
    if (t.chord) {
      content.push({ type: "chordMarker", attrs: { chord: t.chord } });
    } else if (t.text && !t.synthetic) {
      const marks: { type: string }[] = [];
      if (t.bold) marks.push({ type: "strong" });
      if (t.italic) marks.push({ type: "em" });
      if (t.underline) marks.push({ type: "underline" });
      content.push(marks.length ? { type: "text", text: t.text, marks } : { type: "text", text: t.text });
    }
  }
  return content;
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
    case "line": {
      const tokens: Token[] = [];
      let hasChord = false;
      let hasMarks = false;
      node.content.forEach((child) => {
        if (child.type.name === "chordMarker") {
          tokens.push({ chord: child.attrs.chord as string });
          hasChord = true;
        } else {
          const bold = child.marks.some((m) => m.type.name === "strong");
          const italic = child.marks.some((m) => m.type.name === "em");
          const underline = child.marks.some((m) => m.type.name === "underline");
          if (bold || italic || underline) hasMarks = true;
          tokens.push({
            text: child.text ?? "",
            ...(bold ? { bold: true } : {}),
            ...(italic ? { italic: true } : {}),
            ...(underline ? { underline: true } : {}),
          });
        }
      });
      if (!hasChord) {
        // tokens only included when there's actually formatting to
        // preserve — plain text lines keep the simpler flat-string shape.
        return {
          kind: "text_line",
          text: node.textContent,
          ...(hasMarks ? { tokens } : {}),
        };
      }
      const hasWord = tokens.some((t) => t.text && t.text.trim() !== "");
      return {
        kind: hasWord ? "chord_lyric_pair" : "chord_only_line",
        tokens,
        annotation: (node.attrs.annotation as string) ?? "",
      };
    }
    default:
      throw new Error(`docNodeToBlocks: unknown node type ${node.type.name}`);
  }
}
