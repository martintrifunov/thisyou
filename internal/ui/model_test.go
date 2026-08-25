package ui

import (
	"strings"
	"testing"
)

func TestWrapTextDoesNotExceedWidth(t *testing.T) {
	t.Parallel()
	got := wrapText("a short sentence followed-by-a-very-long-unbroken-value", 12)
	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 12 {
			t.Fatalf("line %q exceeds width: %q", line, got)
		}
	}
}

func TestWrapTextPreservesParagraphs(t *testing.T) {
	t.Parallel()
	got := wrapText("first paragraph\n\nsecond paragraph", 80)
	if got != "first paragraph\n\nsecond paragraph" {
		t.Fatalf("got %q", got)
	}
}
