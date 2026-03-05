package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jmvrbanac/slackseek/internal/emoji"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/olekukonko/tablewriter"
)

// EmojiEnabled controls whether :name: tokens are rendered as Unicode in output.
// Set by the CLI layer before calling any Print functions.
var EmojiEnabled = false

// WrapWidth controls word-wrap width for text output. 0 = disabled.
// Set by the CLI layer before calling any Print functions.
var WrapWidth = 0

// Format controls the output representation of all Print functions.
type Format string

const (
	FormatText     Format = "text"
	FormatTable    Format = "table"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "markdown"
)

// ValidFormats lists all accepted --format values.
var ValidFormats = []Format{FormatText, FormatTable, FormatJSON, FormatMarkdown}

// truncate returns the first n runes of s followed by "…" if s is longer than n.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// TableSafe collapses all whitespace (newlines, tabs, multiple spaces) in s
// to single spaces using strings.Fields, then truncates to n runes.
// This prevents multi-line messages from breaking table cell alignment.
func TableSafe(s string, n int) string {
	return truncate(strings.Join(strings.Fields(s), " "), n)
}

// formatReactions formats a reaction slice as "name×count name×count".
// When EmojiEnabled is true, reaction names are rendered as Unicode.
func formatReactions(rs []slack.Reaction) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		name := r.Name
		if EmojiEnabled {
			name = emoji.RenderName(r.Name)
		}
		parts = append(parts, fmt.Sprintf("%s×%d", name, r.Count))
	}
	return strings.Join(parts, " ")
}

// --- JSON intermediate types ---

type workspaceJSON struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Token  string `json:"token"`
	Cookie string `json:"cookie"`
}

type channelJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	MemberCount int    `json:"member_count"`
	Topic       string `json:"topic"`
	IsArchived  bool   `json:"is_archived"`
}

type reactionJSON struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type messageJSON struct {
	Timestamp       string         `json:"timestamp"`
	SlackTS         string         `json:"slack_ts"`
	UserID          string         `json:"user_id"`
	UserDisplayName string         `json:"user_display_name"`
	Text            string         `json:"text"`
	ChannelID       string         `json:"channel_id"`
	ChannelName     string         `json:"channel_name,omitempty"`
	ThreadTS        string         `json:"thread_ts"`
	ThreadDepth     int            `json:"thread_depth"`
	Reactions       []reactionJSON `json:"reactions"`
	Replies         []messageJSON  `json:"replies,omitempty"`
}

type searchResultJSON struct {
	Timestamp       string         `json:"timestamp"`
	SlackTS         string         `json:"slack_ts"`
	UserID          string         `json:"user_id"`
	UserDisplayName string         `json:"user_display_name"`
	Text            string         `json:"text"`
	ChannelID       string         `json:"channel_id"`
	ChannelName     string         `json:"channel_name,omitempty"`
	ThreadTS        string         `json:"thread_ts"`
	ThreadDepth     int            `json:"thread_depth"`
	Reactions       []reactionJSON `json:"reactions"`
	Permalink       string         `json:"permalink"`
}

type userJSON struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name"`
	Email       string `json:"email"`
	IsBot       bool   `json:"is_bot"`
	IsDeleted   bool   `json:"is_deleted"`
}

// --- Helper: encode to JSON array ---

func writeJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --- PrintWorkspaces ---

// PrintWorkspaces writes workspace data to w in the requested format.
func PrintWorkspaces(w io.Writer, format Format, workspaces []tokens.Workspace) error {
	switch format {
	case FormatJSON:
		out := make([]workspaceJSON, len(workspaces))
		for i, ws := range workspaces {
			out[i] = workspaceJSON{
				Name:   ws.Name,
				URL:    ws.URL,
				Token:  truncate(ws.Token, 12),
				Cookie: truncate(ws.Cookie, 8),
			}
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"Name", "URL", "Token", "Cookie"})
		rows := make([][]string, len(workspaces))
		for i, ws := range workspaces {
			rows[i] = []string{ws.Name, ws.URL, truncate(ws.Token, 12), truncate(ws.Cookie, 8)}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building workspace table: %w", err)
		}
		return tbl.Render()
	default: // FormatText
		for _, ws := range workspaces {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ws.Name, ws.URL, truncate(ws.Token, 12), truncate(ws.Cookie, 8))
		}
		return nil
	}
}

// --- PrintChannels ---

// PrintChannels writes channel data to w in the requested format.
func PrintChannels(w io.Writer, format Format, channels []slack.Channel) error {
	switch format {
	case FormatJSON:
		out := make([]channelJSON, len(channels))
		for i, ch := range channels {
			out[i] = channelJSON{
				ID:          ch.ID,
				Name:        ch.Name,
				Type:        ch.Type,
				MemberCount: ch.MemberCount,
				Topic:       ch.Topic,
				IsArchived:  ch.IsArchived,
			}
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"ID", "Name", "Type", "Members", "Topic"})
		rows := make([][]string, len(channels))
		for i, ch := range channels {
			rows[i] = []string{ch.ID, ch.Name, ch.Type, strconv.Itoa(ch.MemberCount), truncate(ch.Topic, 60)}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building channels table: %w", err)
		}
		return tbl.Render()
	default: // FormatText
		for _, ch := range channels {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", ch.ID, ch.Name, ch.Type, ch.MemberCount, ch.Topic)
		}
		return nil
	}
}

// resolveMessageFields returns resolved user name, channel name, and message
// text (with inline @mentions replaced), falling back to raw values when the
// resolver is nil or an ID is unknown.
// When EmojiEnabled is true, :name: tokens in text are rendered as Unicode.
func resolveMessageFields(m slack.Message, resolver *slack.Resolver) (userDisplay, channelDisplay, text string) {
	userDisplay = m.UserID
	channelDisplay = m.ChannelName
	text = m.Text
	if resolver != nil {
		userDisplay = resolver.UserDisplayName(m.UserID)
		channelDisplay = resolver.ResolveChannelDisplay(m.ChannelID, m.ChannelName)
		text = resolver.ResolveMentions(text)
	}
	if EmojiEnabled {
		text = emoji.Render(text)
	}
	return
}

// GroupByThread separates messages into root messages and a replies map.
// A message is a root if its ThreadTS is empty or equals its own Timestamp.
// The replies map is keyed by parent Timestamp. O(n) single pass.
func GroupByThread(msgs []slack.Message) (roots []slack.Message, replies map[string][]slack.Message) {
	replies = make(map[string][]slack.Message)
	for _, m := range msgs {
		if m.ThreadTS == "" || m.ThreadTS == m.Timestamp {
			roots = append(roots, m)
		} else {
			replies[m.ThreadTS] = append(replies[m.ThreadTS], m)
		}
	}
	return
}

// --- PrintMessages ---

// PrintMessages writes message data to w in the requested format.
// resolver is optional; when nil, raw user IDs are used as-is.
func PrintMessages(w io.Writer, format Format, messages []slack.Message, resolver *slack.Resolver) error {
	switch format {
	case FormatJSON:
		return printMessagesJSON(w, messages, resolver)
	case FormatTable:
		return printMessagesTable(w, messages, resolver)
	case FormatMarkdown:
		return printMessagesMarkdown(w, messages, resolver)
	default: // FormatText
		return printMessagesText(w, messages, resolver)
	}
}

func printMessagesJSON(w io.Writer, messages []slack.Message, resolver *slack.Resolver) error {
	roots, replies := GroupByThread(messages)
	out := make([]messageJSON, len(roots))
	for i, m := range roots {
		mj := toMessageJSON(m, resolver)
		if reps, ok := replies[m.Timestamp]; ok {
			mj.Replies = make([]messageJSON, len(reps))
			for j, r := range reps {
				mj.Replies[j] = toMessageJSON(r, resolver)
			}
		}
		out[i] = mj
	}
	return writeJSON(w, out)
}

func printMessagesTable(w io.Writer, messages []slack.Message, resolver *slack.Resolver) error {
	tbl := tablewriter.NewWriter(w)
	tbl.Header([]string{"Timestamp", "User", "Channel", "Text", "Depth", "Reactions"})
	roots, replies := GroupByThread(messages)
	var rows [][]string
	for _, m := range roots {
		user, ch, text := resolveMessageFields(m, resolver)
		rows = append(rows, []string{
			m.Time.Format(time.RFC3339), user, ch,
			TableSafe(text, 80), strconv.Itoa(m.ThreadDepth), formatReactions(m.Reactions),
		})
		for _, reply := range replies[m.Timestamp] {
			rUser, rCh, rText := resolveMessageFields(reply, resolver)
			rows = append(rows, []string{
				reply.Time.Format(time.RFC3339), rUser, rCh,
				"  └─ " + TableSafe(rText, 75), strconv.Itoa(reply.ThreadDepth), formatReactions(reply.Reactions),
			})
		}
	}
	if err := tbl.Bulk(rows); err != nil {
		return fmt.Errorf("building messages table: %w", err)
	}
	return tbl.Render()
}

func printMessagesText(w io.Writer, messages []slack.Message, resolver *slack.Resolver) error {
	roots, replies := GroupByThread(messages)
	for i, m := range roots {
		user, ch, text := resolveMessageFields(m, resolver)
		ts := m.Time.Format(time.RFC3339)
		// prefix width = timestamp + tab + user + tab + channel + tab
		prefixWidth := len(ts) + 1 + len(user) + 1 + len(ch) + 1
		if WrapWidth > 0 && prefixWidth < WrapWidth {
			text = WordWrap(text, WrapWidth-prefixWidth, prefixWidth)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ts, user, ch, text)
		for _, reply := range replies[m.Timestamp] {
			rUser, _, rText := resolveMessageFields(reply, resolver)
			rts := reply.Time.Format(time.RFC3339)
			replyPrefixWidth := 5 + len(rts) + 1 + len(rUser) + 1 // "  └─ " prefix
			if WrapWidth > 0 && replyPrefixWidth < WrapWidth {
				rText = WordWrap(rText, WrapWidth-replyPrefixWidth, replyPrefixWidth)
			}
			fmt.Fprintf(w, "  └─ %s\t%s\t%s\n", rts, rUser, rText)
		}
		if i < len(roots)-1 {
			fmt.Fprintln(w)
		}
	}
	return nil
}

// toSearchResultJSON converts a SearchResult to its JSON representation.
// resolver is optional; when nil, raw user IDs are used as-is.
func toSearchResultJSON(sr slack.SearchResult, resolver *slack.Resolver) searchResultJSON {
	mj := toMessageJSON(sr.Message, resolver)
	return searchResultJSON{
		Timestamp:       mj.Timestamp,
		SlackTS:         mj.SlackTS,
		UserID:          mj.UserID,
		UserDisplayName: mj.UserDisplayName,
		Text:            mj.Text,
		ChannelID:       mj.ChannelID,
		ChannelName:     mj.ChannelName,
		ThreadTS:        mj.ThreadTS,
		ThreadDepth:     mj.ThreadDepth,
		Reactions:       mj.Reactions,
		Permalink:       sr.Permalink,
	}
}

// --- PrintSearchResults ---

// PrintSearchResults writes search result data to w in the requested format.
// resolver is optional; when nil, raw user IDs are used as-is.
func PrintSearchResults(w io.Writer, format Format, results []slack.SearchResult, resolver *slack.Resolver) error {
	switch format {
	case FormatJSON:
		out := make([]searchResultJSON, len(results))
		for i, sr := range results {
			out[i] = toSearchResultJSON(sr, resolver)
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"Timestamp", "Channel", "User", "Text", "Permalink"})
		rows := make([][]string, len(results))
		for i, sr := range results {
			user, _, text := resolveMessageFields(sr.Message, resolver)
			rows[i] = []string{
				sr.Time.Format(time.RFC3339),
				sr.ChannelName,
				user,
				TableSafe(text, 80),
				sr.Permalink,
			}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building search results table: %w", err)
		}
		return tbl.Render()
	case FormatMarkdown:
		return printSearchResultsMarkdown(w, results, resolver)
	default: // FormatText
		for _, sr := range results {
			user, _, text := resolveMessageFields(sr.Message, resolver)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sr.Time.Format(time.RFC3339), sr.ChannelName, user, text)
		}
		return nil
	}
}

// --- PrintUsers ---

// PrintUsers writes user data to w in the requested format.
func PrintUsers(w io.Writer, format Format, users []slack.User) error {
	switch format {
	case FormatJSON:
		out := make([]userJSON, len(users))
		for i, u := range users {
			out[i] = userJSON{
				ID:          u.ID,
				DisplayName: u.DisplayName,
				RealName:    u.RealName,
				Email:       u.Email,
				IsBot:       u.IsBot,
				IsDeleted:   u.IsDeleted,
			}
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"ID", "Display Name", "Real Name", "Email", "Bot", "Deleted"})
		rows := make([][]string, len(users))
		for i, u := range users {
			rows[i] = []string{
				u.ID,
				u.DisplayName,
				u.RealName,
				u.Email,
				strconv.FormatBool(u.IsBot),
				strconv.FormatBool(u.IsDeleted),
			}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building users table: %w", err)
		}
		return tbl.Render()
	default: // FormatText
		for _, u := range users {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.ID, u.DisplayName, u.RealName, u.Email)
		}
		return nil
	}
}

// --- helpers ---

// printMessagesMarkdown writes messages as a Markdown document.
// Root messages become ## headings; replies become > block quotes.
func printMessagesMarkdown(w io.Writer, messages []slack.Message, resolver *slack.Resolver) error {
	if len(messages) == 0 {
		return nil
	}
	ch := messages[0].ChannelName
	if ch == "" && resolver != nil {
		ch = resolver.ChannelName(messages[0].ChannelID)
	}
	date := messages[0].Time.Format("2006-01-02")
	allSameDate := true
	for _, m := range messages[1:] {
		if m.Time.Format("2006-01-02") != date {
			allSameDate = false
			break
		}
	}
	if allSameDate {
		fmt.Fprintf(w, "# #%s — %s\n\n", ch, date)
	} else {
		fmt.Fprintf(w, "# #%s\n\n", ch)
	}
	roots, replies := GroupByThread(messages)
	for _, m := range roots {
		user, _, text := resolveMessageFields(m, resolver)
		fmt.Fprintf(w, "## %s · %s\n\n%s\n", m.Time.Format(time.RFC3339), user, text)
		if len(m.Reactions) > 0 {
			fmt.Fprintf(w, "\n_%s_\n", formatReactions(m.Reactions))
		}
		for _, reply := range replies[m.Timestamp] {
			rUser, _, rText := resolveMessageFields(reply, resolver)
			fmt.Fprintf(w, "\n> **%s** · %s: %s\n", rUser, reply.Time.Format(time.RFC3339), rText)
			if len(reply.Reactions) > 0 {
				fmt.Fprintf(w, ">\n> _%s_\n", formatReactions(reply.Reactions))
			}
		}
		fmt.Fprintf(w, "\n---\n")
	}
	return nil
}

// printSearchResultsMarkdown writes search results as a Markdown document.
func printSearchResultsMarkdown(w io.Writer, results []slack.SearchResult, resolver *slack.Resolver) error {
	fmt.Fprintf(w, "# Search results\n\n")
	for _, sr := range results {
		user, _, text := resolveMessageFields(sr.Message, resolver)
		date := sr.Time.Format("2006-01-02")
		ch := sr.ChannelName
		if ch == "" && resolver != nil {
			ch = resolver.ChannelName(sr.ChannelID)
		}
		fmt.Fprintf(w, "## %s · %s · %s\n\n%s\n", date, ch, user, text)
		if sr.Permalink != "" {
			fmt.Fprintf(w, "\n[View in Slack](%s)\n", sr.Permalink)
		}
		fmt.Fprintf(w, "\n---\n")
	}
	return nil
}

func toMessageJSON(m slack.Message, resolver *slack.Resolver) messageJSON {
	reactions := make([]reactionJSON, len(m.Reactions))
	for i, r := range m.Reactions {
		reactions[i] = reactionJSON{Name: r.Name, Count: r.Count}
	}
	userDisplay := ""
	if resolver != nil {
		userDisplay = resolver.UserDisplayName(m.UserID)
	}
	channelName := m.ChannelName
	text := m.Text
	if resolver != nil {
		channelName = resolver.ResolveChannelDisplay(m.ChannelID, m.ChannelName)
		text = resolver.ResolveMentions(text)
	}
	return messageJSON{
		Timestamp:       m.Time.Format(time.RFC3339),
		SlackTS:         m.Timestamp,
		UserID:          m.UserID,
		UserDisplayName: userDisplay,
		Text:            text,
		ChannelID:       m.ChannelID,
		ChannelName:     channelName,
		ThreadTS:        m.ThreadTS,
		ThreadDepth:     m.ThreadDepth,
		Reactions:       reactions,
	}
}
