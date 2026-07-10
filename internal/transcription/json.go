package transcription

import (
	"encoding/json"
	"fmt"
)

func (k BlockKind) String() string {
	switch k {
	case SectionHeader:
		return "section_header"
	case ChordLyricPair:
		return "chord_lyric_pair"
	case ChordOnlyLine:
		return "chord_only_line"
	case TextLine:
		return "text_line"
	default:
		return "unknown"
	}
}

// MarshalJSON renders BlockKind as its readable string form rather than a
// bare integer — this is what ends up stored in Postgres JSONB and read by
// the future ProseMirror editor, so it needs to be self-describing.
func (k BlockKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

func (k *BlockKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "section_header":
		*k = SectionHeader
	case "chord_lyric_pair":
		*k = ChordLyricPair
	case "chord_only_line":
		*k = ChordOnlyLine
	case "text_line":
		*k = TextLine
	default:
		return fmt.Errorf("transcription: unknown block kind %q", s)
	}
	return nil
}

// document is the JSON envelope stored in transcription_versions.content —
// matches the migration's default of '{"blocks": []}'.
type document struct {
	Blocks []Block `json:"blocks"`
}

// MarshalDocument renders blocks into the JSON shape stored in Postgres.
func MarshalDocument(blocks []Block) ([]byte, error) {
	if blocks == nil {
		blocks = []Block{}
	}
	return json.Marshal(document{Blocks: blocks})
}

// UnmarshalDocument reverses MarshalDocument.
func UnmarshalDocument(data []byte) ([]Block, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("transcription: unmarshaling document: %w", err)
	}
	return doc.Blocks, nil
}
