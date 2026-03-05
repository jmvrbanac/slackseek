package output

import (
	"strings"
	"unicode/utf8"
)

// WordWrap wraps s at word boundaries so no line exceeds width runes.
// Continuation lines are indented by indent spaces.
// A width of 0 disables wrapping and returns s unchanged.
func WordWrap(s string, width, indent int) string {
	if width <= 0 || utf8.RuneCountInString(s) <= width {
		return s
	}

	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}

	indentStr := strings.Repeat(" ", indent)
	var b strings.Builder
	lineLen := 0
	firstLine := true

	for _, word := range words {
		wordLen := utf8.RuneCountInString(word)
		if !firstLine && lineLen == 0 {
			// Start of a continuation line
			b.WriteString(indentStr)
			lineLen = indent
		}
		if lineLen == 0 {
			b.WriteString(word)
			lineLen = wordLen
		} else if lineLen+1+wordLen <= width {
			b.WriteByte(' ')
			b.WriteString(word)
			lineLen += 1 + wordLen
		} else {
			b.WriteByte('\n')
			b.WriteString(indentStr)
			b.WriteString(word)
			lineLen = indent + wordLen
			firstLine = false
		}
	}

	return b.String()
}
