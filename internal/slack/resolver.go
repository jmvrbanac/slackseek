package slack

import "regexp"

// mentionPattern matches Slack inline user mention tokens: <@USERID>
var mentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)>`)

// subteamPattern matches <!subteam^ID> and <!subteam^ID|label> tokens.
var subteamPattern = regexp.MustCompile(`<!subteam\^[A-Z0-9]+(?:\|([^>]+))?>`)

// broadcastPattern matches <!here>, <!channel>, and <!everyone>.
var broadcastPattern = regexp.MustCompile(`<!(here|channel|everyone)>`)

// Resolver holds pre-built lookup maps for resolving Slack IDs to human-readable names.
// It is constructed once per command invocation and discarded after output is written.
type Resolver struct {
	users    map[string]string
	channels map[string]string
}

// NewResolver constructs a Resolver from slices of users and channels.
// It builds O(n) lookup maps at construction time. Nil slices are safe.
// For users, RealName is preferred; DisplayName is the fallback; raw ID
// is used when both are empty.
func NewResolver(users []User, channels []Channel) *Resolver {
	r := &Resolver{
		users:    make(map[string]string, len(users)),
		channels: make(map[string]string, len(channels)),
	}
	for _, u := range users {
		name := u.RealName
		if name == "" {
			name = u.DisplayName
		}
		if name != "" {
			r.users[u.ID] = name
		}
	}
	for _, ch := range channels {
		r.channels[ch.ID] = ch.Name
	}
	return r
}

// UserDisplayName returns the resolved display name for a user ID.
// Falls back to the raw ID string if the user is not found.
func (r *Resolver) UserDisplayName(id string) string {
	if name, ok := r.users[id]; ok {
		return name
	}
	return id
}

// ChannelName returns the resolved name for a channel ID.
// Falls back to the raw ID string if the channel is not found.
func (r *Resolver) ChannelName(id string) string {
	if name, ok := r.channels[id]; ok {
		return name
	}
	return id
}

// ResolveMentions replaces Slack mention tokens in text with human-readable forms:
//   - <@USERID>                      → @Real Name (or @USERID if unresolved)
//   - <!subteam^ID|@handle>          → @handle
//   - <!subteam^ID>                  → @[group]
//   - <!here>, <!channel>, <!everyone> → @here, @channel, @everyone
func (r *Resolver) ResolveMentions(text string) string {
	text = mentionPattern.ReplaceAllStringFunc(text, func(match string) string {
		id := match[2 : len(match)-1] // strip "<@" and ">"
		return "@" + r.UserDisplayName(id)
	})
	text = subteamPattern.ReplaceAllStringFunc(text, func(match string) string {
		if subs := subteamPattern.FindStringSubmatch(match); len(subs) > 1 && subs[1] != "" {
			return subs[1]
		}
		return "@[group]"
	})
	text = broadcastPattern.ReplaceAllString(text, "@$1")
	return text
}
