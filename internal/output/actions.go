package output

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// ActionItem is a message that matched a commitment pattern.
type ActionItem struct {
	Who       string
	Text      string
	Timestamp time.Time
}

// commitmentPatterns are the regular expressions used to detect action items.
var commitmentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bi'?ll\b`),
	regexp.MustCompile(`(?i)\bi will\b`),
	regexp.MustCompile(`(?i)\bwill do\b`),
	regexp.MustCompile(`(?i)\bon it\b`),
	regexp.MustCompile(`(?i)\baction item\b`),
	regexp.MustCompile(`(?i)\bTODO\b`),
	regexp.MustCompile(`(?i)\bfollow[\s-]?up\b`),
	regexp.MustCompile(`(?i)<@[A-Z0-9]+>[^.]*can you\b`),
	regexp.MustCompile(`(?i)<@[A-Z0-9]+>[^.]*please\b`),
}

// ExtractActions scans messages for commitment patterns and returns matches.
func ExtractActions(messages []slack.Message, resolver *slack.Resolver) []ActionItem {
	var items []ActionItem
	for _, m := range messages {
		text := m.Text
		if resolver != nil {
			text = resolver.ResolveMentions(text)
		}
		if matchesCommitment(m.Text) || matchesCommitment(text) {
			who := m.UserID
			if resolver != nil {
				who = resolver.UserDisplayName(m.UserID)
			}
			items = append(items, ActionItem{
				Who:       who,
				Text:      text,
				Timestamp: m.Time,
			})
		}
	}
	return items
}

func matchesCommitment(text string) bool {
	for _, p := range commitmentPatterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// PrintActions formats ActionItems to w.
func PrintActions(w io.Writer, fmt Format, items []ActionItem) error {
	switch fmt {
	case FormatJSON:
		return printActionsJSON(w, items)
	default:
		return printActionsText(w, items)
	}
}

func printActionsText(w io.Writer, items []ActionItem) error {
	if len(items) == 0 {
		fmt.Fprintln(w, "No action items found.")
		return nil
	}
	for _, item := range items {
		text := truncate(item.Text, 60)
		fmt.Fprintf(w, "[ ] @%-15s  %-60s  %s\n",
			item.Who,
			text,
			item.Timestamp.UTC().Format("2006-01-02 15:04"),
		)
	}
	fmt.Fprintf(w, "\n%d action item(s) found.\n", len(items))
	return nil
}

type actionItemJSON struct {
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

func printActionsJSON(w io.Writer, items []ActionItem) error {
	out := make([]actionItemJSON, len(items))
	for i, item := range items {
		out[i] = actionItemJSON{
			User:      item.Who,
			Text:      item.Text,
			Timestamp: item.Timestamp.UTC().Format(time.RFC3339),
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
