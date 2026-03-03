package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/olekukonko/tablewriter"
)

// Format controls the output representation of all Print functions.
type Format string

const (
	FormatText  Format = "text"
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// ValidFormats lists all accepted --format values.
var ValidFormats = []Format{FormatText, FormatTable, FormatJSON}

// truncate returns the first n runes of s followed by "…" if s is longer than n.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// formatReactions formats a reaction slice as "name×count name×count".
func formatReactions(rs []slack.Reaction) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, fmt.Sprintf("%s×%d", r.Name, r.Count))
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
func resolveMessageFields(m slack.Message, resolver *slack.Resolver) (userDisplay, channelDisplay, text string) {
	userDisplay = m.UserID
	channelDisplay = m.ChannelName
	text = m.Text
	if resolver != nil {
		userDisplay = resolver.UserDisplayName(m.UserID)
		if channelDisplay == "" {
			channelDisplay = resolver.ChannelName(m.ChannelID)
		}
		text = resolver.ResolveMentions(text)
	}
	return
}

// --- PrintMessages ---

// PrintMessages writes message data to w in the requested format.
// resolver is optional; when nil, raw user IDs are used as-is.
func PrintMessages(w io.Writer, format Format, messages []slack.Message, resolver *slack.Resolver) error {
	switch format {
	case FormatJSON:
		out := make([]messageJSON, len(messages))
		for i, m := range messages {
			out[i] = toMessageJSON(m, resolver)
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"Timestamp", "User", "Channel", "Text", "Depth", "Reactions"})
		rows := make([][]string, len(messages))
		for i, m := range messages {
			user, ch, text := resolveMessageFields(m, resolver)
			rows[i] = []string{
				m.Time.Format(time.RFC3339),
				user, ch,
				truncate(text, 80),
				strconv.Itoa(m.ThreadDepth),
				formatReactions(m.Reactions),
			}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building messages table: %w", err)
		}
		return tbl.Render()
	default: // FormatText
		for _, m := range messages {
			user, ch, text := resolveMessageFields(m, resolver)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Time.Format(time.RFC3339), user, ch, text)
		}
		return nil
	}
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
				truncate(text, 80),
				sr.Permalink,
			}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building search results table: %w", err)
		}
		return tbl.Render()
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
		if channelName == "" {
			channelName = resolver.ChannelName(m.ChannelID)
		}
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
