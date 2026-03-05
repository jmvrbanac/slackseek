package emoji

import (
	_ "embed"
	"encoding/json"
	"regexp"
)

//go:embed emoji-map.json
var emojiMapJSON []byte

var emojiMap map[string]string

// emojiTokenPattern matches :name: tokens (Slack emoji syntax).
var emojiTokenPattern = regexp.MustCompile(`:([a-z0-9_+\-]+):`)

func init() {
	emojiMap = mustLoad()
}

func mustLoad() map[string]string {
	m := make(map[string]string)
	if err := json.Unmarshal(emojiMapJSON, &m); err != nil {
		panic("emoji: failed to load emoji-map.json: " + err.Error())
	}
	return m
}

// Render replaces all :name: tokens in s with their Unicode equivalents.
// Tokens with no mapping are left unchanged.
func Render(s string) string {
	return emojiTokenPattern.ReplaceAllStringFunc(s, func(match string) string {
		subs := emojiTokenPattern.FindStringSubmatch(match)
		if len(subs) < 2 {
			return match
		}
		if unicode, ok := emojiMap[subs[1]]; ok {
			return unicode
		}
		return match
	})
}

// RenderName returns the Unicode equivalent of a single emoji name
// (without colons). Returns the name unchanged if not found.
func RenderName(name string) string {
	if unicode, ok := emojiMap[name]; ok {
		return unicode
	}
	return name
}
