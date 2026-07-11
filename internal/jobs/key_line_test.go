package jobs

import "testing"

func TestParseKeyLineExtractsPerformanceAndOriginalKey(t *testing.T) {
	rawText := "Great Balls of Fire\nAs performed by: Jerry Lee Lewis\nKey:  C, Original C\nINTRO:  E   A   B\n"
	performance, original, ok := parseKeyLine(rawText)
	if !ok {
		t.Fatal("parseKeyLine() ok = false, want true")
	}
	if performance != "C" || original != "C" {
		t.Errorf("performance = %q, original = %q, want C, C", performance, original)
	}
}

func TestParseKeyLineHandlesDifferingKeys(t *testing.T) {
	rawText := "Some Song\nAs performed by: Someone\nKey:  Bb, Original A\nVERSE 1:\n"
	performance, original, ok := parseKeyLine(rawText)
	if !ok {
		t.Fatal("parseKeyLine() ok = false, want true")
	}
	if performance != "Bb" || original != "A" {
		t.Errorf("performance = %q, original = %q, want Bb, A", performance, original)
	}
}

func TestParseKeyLineToleratesExtraInternalWhitespace(t *testing.T) {
	rawText := "Song\nAs performed by:  Someone\nKey:   Gm,   Original   Cm\nVERSE 1:\n"
	performance, original, ok := parseKeyLine(rawText)
	if !ok {
		t.Fatal("parseKeyLine() ok = false, want true")
	}
	if performance != "Gm" || original != "Cm" {
		t.Errorf("performance = %q, original = %q, want Gm, Cm", performance, original)
	}
}

func TestParseKeyLineReturnsFalseWhenNoKeyLine(t *testing.T) {
	rawText := "Song\nAs performed by: Someone\nVERSE 1:\n"
	_, _, ok := parseKeyLine(rawText)
	if ok {
		t.Error("parseKeyLine() ok = true, want false (no Key line present)")
	}
}
