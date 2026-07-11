// Mirrors internal/transcription/blocks.go and json.go's wire format.
// BlockKind is serialized as its readable string form by Go's MarshalJSON.

export type BlockKind =
  | "section_header"
  | "chord_lyric_pair"
  | "chord_only_line"
  | "text_line";

export interface Token {
  chord?: string;
  text?: string;
  synthetic?: boolean;
}

export interface Block {
  kind: BlockKind;
  text?: string;
  tokens?: Token[];
  annotation?: string;
}

export interface Document {
  blocks: Block[];
}
