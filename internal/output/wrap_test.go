package output

import (
	"strings"
	"testing"
)

// T018: WordWrap unit tests

func TestWordWrap_WidthZeroNoOp(t *testing.T) {
	s := "this is a long string that should not be wrapped at all"
	got := WordWrap(s, 0, 0)
	if got != s {
		t.Errorf("expected no wrapping with width=0, got %q", got)
	}
}

func TestWordWrap_ShortStringUnchanged(t *testing.T) {
	s := "hello world"
	got := WordWrap(s, 80, 0)
	if got != s {
		t.Errorf("expected unchanged short string, got %q", got)
	}
}

func TestWordWrap_WrapsAtWordBoundary(t *testing.T) {
	s := "hello world foo bar"
	got := WordWrap(s, 12, 0)
	lines := strings.Split(got, "\n")
	for _, line := range lines {
		if len([]rune(line)) > 12 {
			t.Errorf("line exceeds width 12: %q (len=%d)", line, len([]rune(line)))
		}
	}
}

func TestWordWrap_ContinuationIndent(t *testing.T) {
	s := "hello world this is a long message that should wrap"
	got := WordWrap(s, 20, 4)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatal("expected multiple lines")
	}
	// continuation lines should start with 4 spaces
	for _, line := range lines[1:] {
		if line != "" && !strings.HasPrefix(line, "    ") {
			t.Errorf("expected continuation line to start with 4 spaces, got %q", line)
		}
	}
}

func TestWordWrap_EmptyString(t *testing.T) {
	got := WordWrap("", 80, 0)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestWordWrap_SingleLongWordNotBroken(t *testing.T) {
	s := "superlongwordthatcannotbesplit"
	got := WordWrap(s, 10, 0)
	// single long word should appear on its own line, not broken mid-word
	lines := strings.Split(got, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, s) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected long word to appear intact in output, got %q", got)
	}
}

func TestWordWrap_ExactBoundary(t *testing.T) {
	// exactly 10 chars - should not wrap
	s := "1234567890"
	got := WordWrap(s, 10, 0)
	if got != s {
		t.Errorf("expected no wrap at exact boundary, got %q", got)
	}
}
