package slack

import (
	"fmt"
	"net/url"
	"strings"
)

// ThreadPermalink holds the parsed components of a Slack permalink URL.
type ThreadPermalink struct {
	WorkspaceURL string // e.g. "https://acme.slack.com"
	ChannelID    string // e.g. "C01234567"
	ThreadTS     string // Slack ts, e.g. "1700000000.123456"
}

// ParsePermalink parses a Slack permalink URL and returns a ThreadPermalink.
//
// Accepted format: https://<workspace>.slack.com/archives/<channelID>/p<ts>[?thread_ts=<ts>]
// When ?thread_ts is present in the query string it is used as the root thread
// timestamp; otherwise the path timestamp is used.
func ParsePermalink(rawURL string) (ThreadPermalink, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ThreadPermalink{}, fmt.Errorf("parse permalink %q: %w", rawURL, err)
	}
	if u.Scheme != "https" {
		return ThreadPermalink{}, fmt.Errorf("parse permalink %q: scheme must be https", rawURL)
	}
	channelID, tsSegment, parseErr := parseArchivePath(rawURL, u.Path)
	if parseErr != nil {
		return ThreadPermalink{}, parseErr
	}
	threadTS := convertTSToSlackFormat(tsSegment[1:]) // strip leading 'p'
	if qt := u.Query().Get("thread_ts"); qt != "" {
		threadTS = qt
	}
	return ThreadPermalink{
		WorkspaceURL: u.Scheme + "://" + u.Host,
		ChannelID:    channelID,
		ThreadTS:     threadTS,
	}, nil
}

// parseArchivePath validates and extracts channelID and ts segment from path.
func parseArchivePath(rawURL, path string) (channelID, tsSegment string, err error) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 3 || segments[0] != "archives" {
		return "", "", fmt.Errorf("parse permalink %q: expected path /archives/<channelID>/p<ts>", rawURL)
	}
	channelID = segments[1]
	if !channelIDPattern.MatchString(channelID) {
		return "", "", fmt.Errorf("parse permalink %q: channel ID %q does not match expected pattern", rawURL, channelID)
	}
	tsSegment = segments[2]
	if !strings.HasPrefix(tsSegment, "p") {
		return "", "", fmt.Errorf("parse permalink %q: timestamp segment %q must start with 'p'", rawURL, tsSegment)
	}
	return channelID, tsSegment, nil
}

// convertTSToSlackFormat converts a p-prefix timestamp (digits only) to
// Slack ts format with a dot separator before the last 6 digits.
// e.g. "1700000000123456" → "1700000000.123456"
func convertTSToSlackFormat(raw string) string {
	if len(raw) <= 6 {
		return raw
	}
	return raw[:len(raw)-6] + "." + raw[len(raw)-6:]
}
