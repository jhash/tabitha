package transcription

import (
	"encoding/json"
	"testing"
)

func TestBlockKindMarshalsToReadableString(t *testing.T) {
	tests := []struct {
		kind BlockKind
		want string
	}{
		{SectionHeader, `"section_header"`},
		{ChordLyricPair, `"chord_lyric_pair"`},
		{ChordOnlyLine, `"chord_only_line"`},
		{TextLine, `"text_line"`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.kind)
		if err != nil {
			t.Fatalf("Marshal(%v) error = %v", tt.kind, err)
		}
		if string(got) != tt.want {
			t.Errorf("Marshal(%v) = %s, want %s", tt.kind, got, tt.want)
		}
	}
}

func TestBlockKindUnmarshalsFromString(t *testing.T) {
	var k BlockKind
	if err := json.Unmarshal([]byte(`"chord_lyric_pair"`), &k); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if k != ChordLyricPair {
		t.Errorf("got %v, want ChordLyricPair", k)
	}
}

func TestBlockKindUnmarshalRejectsUnknownString(t *testing.T) {
	var k BlockKind
	if err := json.Unmarshal([]byte(`"not_a_real_kind"`), &k); err == nil {
		t.Error("expected an error for an unrecognized block kind, got nil")
	}
}

func TestMarshalDocumentRoundTripsThroughJSON(t *testing.T) {
	original := []Block{
		{Kind: SectionHeader, Text: "CHORUS:"},
		{
			Kind: ChordLyricPair,
			Tokens: []Token{
				{Chord: "E"},
				{Text: "I can't get no satisfaction"},
			},
			Annotation: "repeat 3x",
		},
		{Kind: TextLine, Text: "(CHORUS)"},
	}

	data, err := MarshalDocument(original)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}

	got, err := UnmarshalDocument(data)
	if err != nil {
		t.Fatalf("UnmarshalDocument() error = %v", err)
	}

	if len(got) != len(original) {
		t.Fatalf("got %d blocks, want %d", len(got), len(original))
	}
	for i := range original {
		if got[i].Kind != original[i].Kind {
			t.Errorf("block %d: Kind = %v, want %v", i, got[i].Kind, original[i].Kind)
		}
		if got[i].Text != original[i].Text {
			t.Errorf("block %d: Text = %q, want %q", i, got[i].Text, original[i].Text)
		}
		if got[i].Annotation != original[i].Annotation {
			t.Errorf("block %d: Annotation = %q, want %q", i, got[i].Annotation, original[i].Annotation)
		}
		if len(got[i].Tokens) != len(original[i].Tokens) {
			t.Errorf("block %d: got %d tokens, want %d", i, len(got[i].Tokens), len(original[i].Tokens))
		}
	}
}

func TestMarshalDocumentRoundTripsTokenMarks(t *testing.T) {
	original := []Block{
		{
			Kind: ChordLyricPair,
			Tokens: []Token{
				{Chord: "E"},
				{Text: "I can't get ", Bold: true},
				{Text: "no", Italic: true, Underline: true},
				{Text: " satisfaction"},
			},
		},
	}

	data, err := MarshalDocument(original)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	got, err := UnmarshalDocument(data)
	if err != nil {
		t.Fatalf("UnmarshalDocument() error = %v", err)
	}

	tokens := got[0].Tokens
	if len(tokens) != 4 {
		t.Fatalf("got %d tokens, want 4: %+v", len(tokens), tokens)
	}
	if !tokens[1].Bold || tokens[1].Italic || tokens[1].Underline {
		t.Errorf("token[1] = %+v, want only Bold set", tokens[1])
	}
	if !tokens[2].Italic || !tokens[2].Underline || tokens[2].Bold {
		t.Errorf("token[2] = %+v, want Italic and Underline set, not Bold", tokens[2])
	}
	if tokens[3].Bold || tokens[3].Italic || tokens[3].Underline {
		t.Errorf("token[3] = %+v, want no marks set", tokens[3])
	}
}

func TestMarshalDocumentMatchesSchemaDefaultShape(t *testing.T) {
	// The songs migration defaults transcription_versions.content to
	// '{"blocks": []}' — confirm our Go-side shape actually matches that,
	// not some other envelope key.
	data, err := MarshalDocument(nil)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	if string(data) != `{"blocks":[]}` {
		t.Errorf("MarshalDocument(nil) = %s, want {\"blocks\":[]}", data)
	}
}

func TestUnmarshalDocumentHandlesSchemaDefaultShape(t *testing.T) {
	blocks, err := UnmarshalDocument([]byte(`{"blocks": []}`))
	if err != nil {
		t.Fatalf("UnmarshalDocument() error = %v", err)
	}
	if len(blocks) != 0 {
		t.Errorf("got %d blocks, want 0", len(blocks))
	}
}
