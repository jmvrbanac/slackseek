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
	Timestamp   string         `json:"timestamp"`
	SlackTS     string         `json:"slack_ts"`
	UserID      string         `json:"user_id"`
	Text        string         `json:"text"`
	ChannelID   string         `json:"channel_id"`
	ChannelName string         `json:"channel_name,omitempty"`
	ThreadTS    string         `json:"thread_ts"`
	ThreadDepth int            `json:"thread_depth"`
	Reactions   []reactionJSON `json:"reactions"`
}

type searchResultJSON struct {
	Timestamp   string         `json:"timestamp"`
	SlackTS     string         `json:"slack_ts"`
	UserID      string         `json:"user_id"`
	Text        string         `json:"text"`
	ChannelID   string         `json:"channel_id"`
	ChannelName string         `json:"channel_name,omitempty"`
	ThreadTS    string         `json:"thread_ts"`
	ThreadDepth int            `json:"thread_depth"`
	Reactions   []reactionJSON `json:"reactions"`
	Permalink   string         `json:"permalink"`
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

// --- PrintMessages ---

// PrintMessages writes message data to w in the requested format.
func PrintMessages(w io.Writer, format Format, messages []slack.Message) error {
	switch format {
	case FormatJSON:
		out := make([]messageJSON, len(messages))
		for i, m := range messages {
			out[i] = toMessageJSON(m)
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"Timestamp", "User", "Text", "Depth", "Reactions"})
		rows := make([][]string, len(messages))
		for i, m := range messages {
			rows[i] = []string{
				m.Time.Format(time.RFC3339),
				m.UserID,
				truncate(m.Text, 80),
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
			fmt.Fprintf(w, "%s\t%s\t%s\n", m.Time.Format(time.RFC3339), m.UserID, m.Text)
		}
		return nil
	}
}

// toSearchResultJSON converts a SearchResult to its JSON representation.
func toSearchResultJSON(sr slack.SearchResult) searchResultJSON {
	mj := toMessageJSON(sr.Message)
	return searchResultJSON{
		Timestamp:   mj.Timestamp,
		SlackTS:     mj.SlackTS,
		UserID:      mj.UserID,
		Text:        mj.Text,
		ChannelID:   mj.ChannelID,
		ChannelName: mj.ChannelName,
		ThreadTS:    mj.ThreadTS,
		ThreadDepth: mj.ThreadDepth,
		Reactions:   mj.Reactions,
		Permalink:   sr.Permalink,
	}
}

// --- PrintSearchResults ---

// PrintSearchResults writes search result data to w in the requested format.
func PrintSearchResults(w io.Writer, format Format, results []slack.SearchResult) error {
	switch format {
	case FormatJSON:
		out := make([]searchResultJSON, len(results))
		for i, sr := range results {
			out[i] = toSearchResultJSON(sr)
		}
		return writeJSON(w, out)
	case FormatTable:
		tbl := tablewriter.NewWriter(w)
		tbl.Header([]string{"Timestamp", "Channel", "User", "Text", "Permalink"})
		rows := make([][]string, len(results))
		for i, sr := range results {
			rows[i] = []string{
				sr.Time.Format(time.RFC3339),
				sr.ChannelName,
				sr.UserID,
				truncate(sr.Text, 80),
				sr.Permalink,
			}
		}
		if err := tbl.Bulk(rows); err != nil {
			return fmt.Errorf("building search results table: %w", err)
		}
		return tbl.Render()
	default: // FormatText
		for _, sr := range results {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sr.Time.Format(time.RFC3339), sr.ChannelName, sr.UserID, sr.Text)
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

func toMessageJSON(m slack.Message) messageJSON {
	reactions := make([]reactionJSON, len(m.Reactions))
	for i, r := range m.Reactions {
		reactions[i] = reactionJSON{Name: r.Name, Count: r.Count}
	}
	return messageJSON{
		Timestamp:   m.Time.Format(time.RFC3339),
		SlackTS:     m.Timestamp,
		UserID:      m.UserID,
		Text:        m.Text,
		ChannelID:   m.ChannelID,
		ChannelName: m.ChannelName,
		ThreadTS:    m.ThreadTS,
		ThreadDepth: m.ThreadDepth,
		Reactions:   reactions,
	}
}
