package slack

import "regexp"

// mentionPattern matches Slack inline user mention tokens: <@USERID> and <@USERID|label>
var mentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)(?:\|([^>]+))?>`)

// userIDPattern matches Slack user ID strings used as DM channel names.
var userIDPattern = regexp.MustCompile(`^U[A-Z0-9]+$`)

// subteamPattern matches <!subteam^ID> and <!subteam^ID|label> tokens.
// Group 1 captures the group ID; group 2 captures the optional label.
var subteamPattern = regexp.MustCompile(`<!subteam\^([A-Z0-9]+)(?:\|([^>]+))?>`)

// broadcastPattern matches <!here>, <!channel>, and <!everyone>.
var broadcastPattern = regexp.MustCompile(`<!(here|channel|everyone)>`)

// urlPattern matches Slack URL tokens: <URL> and <URL|display text>.
var urlPattern = regexp.MustCompile(`<(https?://[^|>]+)(?:\|([^>]+))?>`)

// Resolver holds pre-built lookup maps for resolving Slack IDs to human-readable names.
// It is constructed once per command invocation and discarded after output is written.
type Resolver struct {
	users    map[string]string
	channels map[string]string
	groups   map[string]string
}

// NewResolver constructs a Resolver from slices of users, channels, and user groups.
// It builds O(n) lookup maps at construction time. Nil slices are safe.
// For users, RealName is preferred; DisplayName is the fallback; raw ID
// is used when both are empty.
func NewResolver(users []User, channels []Channel, groups []UserGroup) *Resolver {
	r := &Resolver{
		users:    make(map[string]string, len(users)),
		channels: make(map[string]string, len(channels)),
		groups:   make(map[string]string, len(groups)),
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
	for _, g := range groups {
		if g.Handle != "" {
			r.groups[g.ID] = g.Handle
		}
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

// ResolveChannelDisplay returns the display string for a channel.
// If name matches a Slack user ID pattern (DM channel), it is resolved to
// "@DisplayName" (or "@rawID" if not in the user map). If name is empty,
// it falls through to ChannelName(id). Otherwise name is returned unchanged.
func (r *Resolver) ResolveChannelDisplay(id, name string) string {
	if userIDPattern.MatchString(name) {
		if display, ok := r.users[name]; ok {
			return "@" + display
		}
		return "@" + name
	}
	if name == "" {
		return r.ChannelName(id)
	}
	return name
}

// ResolveMentions replaces Slack markup tokens in text with human-readable forms:
//   - <@USERID>                        → @Real Name (or @USERID if unresolved)
//   - <!subteam^ID|@handle>            → @handle
//   - <!subteam^ID>                    → @[group]
//   - <!here>, <!channel>, <!everyone> → @here, @channel, @everyone
//   - <https://url|display text>       → display text
//   - <https://url>                    → https://url
func (r *Resolver) ResolveMentions(text string) string {
	text = mentionPattern.ReplaceAllStringFunc(text, func(match string) string {
		subs := mentionPattern.FindStringSubmatch(match)
		id := subs[1]
		if name, ok := r.users[id]; ok {
			return "@" + name
		}
		if len(subs) > 2 && subs[2] != "" {
			return "@" + subs[2] // embedded label fallback
		}
		return "@" + id // raw ID last resort
	})
	text = subteamPattern.ReplaceAllStringFunc(text, func(match string) string {
		subs := subteamPattern.FindStringSubmatch(match)
		if len(subs) > 2 && subs[2] != "" {
			return subs[2] // embedded label wins over group lookup
		}
		if len(subs) > 1 && subs[1] != "" {
			if handle, ok := r.groups[subs[1]]; ok {
				return "@" + handle
			}
		}
		return "@[group]"
	})
	text = broadcastPattern.ReplaceAllString(text, "@$1")
	text = urlPattern.ReplaceAllStringFunc(text, func(match string) string {
		subs := urlPattern.FindStringSubmatch(match)
		if len(subs) > 2 && subs[2] != "" {
			return subs[2] // prefer display text when present
		}
		return subs[1] // bare URL otherwise
	})
	return text
}
