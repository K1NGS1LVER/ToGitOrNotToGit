package diff

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateDiff_ShortInputUnchanged(t *testing.T) {
	short := "diff --git a/x b/x\n+hello"
	if got := truncateDiff(short); got != short {
		t.Errorf("truncateDiff(%q) = %q, want unchanged", short, got)
	}
}

func TestTruncateDiff_CapsLongInput(t *testing.T) {
	long := strings.Repeat("a", maxDiffBytes+100)
	got := truncateDiff(long)

	if !strings.HasSuffix(got, "\n... (truncated)") {
		t.Errorf("truncated diff is missing the marker: %q", got)
	}
	if want := maxDiffBytes + len("\n... (truncated)"); len(got) > want {
		t.Errorf("truncated diff is %d bytes, want at most %d", len(got), want)
	}
}

func TestTruncateDiff_SplitsRuneSafely(t *testing.T) {
	// Fill to one byte short of the limit, then add 3-byte runes so the byte cut
	// lands in the middle of the first one.
	input := strings.Repeat("a", maxDiffBytes-1) + "€€€"
	got := truncateDiff(input)

	if !utf8.ValidString(got) {
		t.Errorf("truncateDiff produced invalid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "\n... (truncated)") {
		t.Errorf("truncated diff is missing the marker: %q", got)
	}
}
