package slack

import "time"

// Channel represents a Slack conversation space returned by the API.
type Channel struct {
	ID          string // Slack channel ID, e.g. "C01234567"
	Name        string // display name without '#', e.g. "general"
	Type        string // "public_channel" | "private_channel" | "mpim" | "im"
	MemberCount int    // number of members (0 for IMs)
	Topic       string // current topic text (may be empty)
	IsArchived  bool
}

// Message represents a single Slack message, including thread metadata.
type Message struct {
	Timestamp   string    // Slack ts format, e.g. "1700000000.123456"
	Time        time.Time // parsed from Timestamp (UTC)
	UserID      string    // Slack user ID, e.g. "U01234567"
	Text        string    // message body (may contain mrkdwn)
	ChannelID   string    // channel this message belongs to
	ChannelName string    // resolved channel name; empty in history context
	ThreadTS    string    // parent message ts; empty if root message
	ThreadDepth int       // 0 = root, 1 = direct reply
	Reactions   []Reaction
}

// Reaction represents a single emoji reaction on a message.
type Reaction struct {
	Name  string // emoji name without colons, e.g. "thumbsup"
	Count int
}

// User represents a Slack workspace member.
type User struct {
	ID          string // Slack user ID, e.g. "U01234567"
	DisplayName string // @-mentionable name
	RealName    string // full legal or preferred name
	Email       string // may be empty if not accessible with current token
	IsBot       bool
	IsDeleted   bool // true for deactivated accounts
}

// SearchResult extends Message with search-specific metadata returned by
// the search.messages API.
type SearchResult struct {
	Message           // embedded — all Message fields apply
	Permalink string  // full URL to the message in Slack web UI
}
