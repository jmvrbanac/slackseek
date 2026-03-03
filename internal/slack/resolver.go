package slack

// Resolver holds pre-built lookup maps for resolving Slack IDs to human-readable names.
// It is constructed once per command invocation and discarded after output is written.
type Resolver struct {
	users    map[string]string
	channels map[string]string
}

// NewResolver constructs a Resolver from slices of users and channels.
// It builds O(n) lookup maps at construction time. Nil slices are safe.
// For users, DisplayName is preferred; RealName is the fallback; raw ID
// is used when both are empty.
func NewResolver(users []User, channels []Channel) *Resolver {
	r := &Resolver{
		users:    make(map[string]string, len(users)),
		channels: make(map[string]string, len(channels)),
	}
	for _, u := range users {
		name := u.DisplayName
		if name == "" {
			name = u.RealName
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
