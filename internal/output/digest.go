package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// ChannelDigest groups messages by channel for digest output.
type ChannelDigest struct {
	ChannelName string
	Messages    []slack.Message
}

// GroupByChannel groups messages by ChannelName, sorted descending by count.
func GroupByChannel(messages []slack.Message) []ChannelDigest {
	byChannel := make(map[string][]slack.Message)
	for _, m := range messages {
		key := m.ChannelName
		if key == "" {
			key = m.ChannelID
		}
		byChannel[key] = append(byChannel[key], m)
	}

	groups := make([]ChannelDigest, 0, len(byChannel))
	for name, msgs := range byChannel {
		groups = append(groups, ChannelDigest{ChannelName: name, Messages: msgs})
	}

	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].Messages) != len(groups[j].Messages) {
			return len(groups[i].Messages) > len(groups[j].Messages)
		}
		return groups[i].ChannelName < groups[j].ChannelName
	})
	return groups
}

// PrintDigest formats ChannelDigest slices to w.
func PrintDigest(w io.Writer, fmt Format, groups []ChannelDigest, resolver *slack.Resolver) error {
	switch fmt {
	case FormatJSON:
		return printDigestJSON(w, groups, resolver)
	default:
		return printDigestText(w, groups, resolver)
	}
}

func printDigestText(w io.Writer, groups []ChannelDigest, resolver *slack.Resolver) error {
	for i, g := range groups {
		ch := g.ChannelName
		if resolver != nil && ch != "" {
			ch = resolver.ChannelName(ch)
		}
		fmt.Fprintf(w, "## #%s (%d messages)\n", ch, len(g.Messages))
		for _, m := range g.Messages {
			user := m.UserID
			if resolver != nil {
				user = resolver.UserDisplayName(m.UserID)
			}
			text := m.Text
			if resolver != nil {
				text = resolver.ResolveMentions(text)
			}
			preview := truncate(text, 80)
			fmt.Fprintf(w, "%s  %s  %s\n",
				m.Time.UTC().Format(time.RFC3339),
				user,
				preview,
			)
		}
		if i < len(groups)-1 {
			fmt.Fprintln(w)
		}
	}
	return nil
}

type digestChannelJSON struct {
	Channel  string        `json:"channel"`
	Count    int           `json:"count"`
	Messages []messageJSON `json:"messages"`
}

func printDigestJSON(w io.Writer, groups []ChannelDigest, resolver *slack.Resolver) error {
	out := make([]digestChannelJSON, len(groups))
	for i, g := range groups {
		ch := g.ChannelName
		msgs := make([]messageJSON, len(g.Messages))
		for j, m := range g.Messages {
			msgs[j] = toMessageJSON(m, resolver)
		}
		out[i] = digestChannelJSON{
			Channel:  ch,
			Count:    len(g.Messages),
			Messages: msgs,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
