package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jmvrbanac/slackseek/internal/cache"
	"github.com/jmvrbanac/slackseek/internal/output"
	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// slackClient is a narrow interface over *slack.Client covering only the
// methods used by MCP tool handlers. *slack.Client satisfies this interface
// without modification.
type slackClient interface {
	SearchMessages(ctx context.Context, query string, limit int) ([]slack.SearchResult, error)
	FetchHistory(ctx context.Context, channelID string, dr slack.DateRange, limit int, threads bool) ([]slack.Message, error)
	GetUserMessages(ctx context.Context, userID, channelID string, dr slack.DateRange, limit int) ([]slack.Message, error)
	FetchThread(ctx context.Context, channelID, threadTS string) ([]slack.Message, error)
	ListChannels(ctx context.Context, types []string, includeArchived bool) ([]slack.Channel, error)
	ListUsers(ctx context.Context) ([]slack.User, error)
	ResolveChannel(ctx context.Context, nameOrID string) (string, error)
	ResolveUser(ctx context.Context, nameOrID string) (string, error)
	FetchUser(ctx context.Context, id string) (slack.User, error)
	FetchChannel(ctx context.Context, id string) (slack.Channel, error)
	ListUserGroups(ctx context.Context) ([]slack.UserGroup, error)
	ForceRefreshUserGroups(ctx context.Context) ([]slack.UserGroup, error)
}

// mcpCacheTTL is the cache TTL used when building a Slack client for MCP tool
// calls. 24 hours matches the default CLI cache TTL.
const mcpCacheTTL = 24 * time.Hour

// mcpChannelIDPattern matches Slack channel/group/DM IDs (uppercase letter + alphanumerics).
var mcpChannelIDPattern = regexp.MustCompile(`^[CDGW][A-Z0-9]{5,}$`)

// parseDateRange resolves since/until strings into a slack.DateRange.
// It delegates to ParseRelativeDateRange when either argument is non-empty
// (supporting ISO date, RFC 3339, and relative durations like "7d"), and to
// ParseDateRange when both are absolute or empty.
func parseDateRange(since, until string) (slack.DateRange, error) {
	if since != "" || until != "" {
		dr, err := slack.ParseRelativeDateRange(since, until)
		if err != nil {
			return dr, fmt.Errorf("invalid date range: %w", err)
		}
		return dr, nil
	}
	return slack.ParseDateRange("", "")
}

// selectWorkspace picks a workspace by name or URL from the slice.
// When selector is empty the first workspace is returned. Unlike the CLI
// version this function does not write to stderr (stdout/stderr are the
// MCP transport on stdio).
func selectWorkspace(workspaces []tokens.Workspace, selector string) (tokens.Workspace, error) {
	if len(workspaces) == 0 {
		return tokens.Workspace{}, fmt.Errorf(
			"no Slack workspaces found — ensure the Slack desktop app is running and you are logged in",
		)
	}
	if selector == "" {
		return workspaces[0], nil
	}
	lower := strings.ToLower(selector)
	for _, ws := range workspaces {
		if strings.ToLower(ws.Name) == lower || ws.URL == selector {
			return ws, nil
		}
	}
	names := make([]string, len(workspaces))
	for i, w := range workspaces {
		names[i] = w.Name
	}
	return tokens.Workspace{}, fmt.Errorf(
		"workspace %q not found — available workspaces: %s",
		selector, strings.Join(names, ", "),
	)
}

// buildMCPClient constructs a *slack.Client backed by a file cache for the
// given workspace. The cache TTL is fixed at mcpCacheTTL (24 h).
func buildMCPClient(ws tokens.Workspace) slackClient {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return slack.NewClient(ws.Token, ws.Cookie, nil)
	}
	store := cache.NewStore(filepath.Join(userCacheDir, "slackseek"), mcpCacheTTL)
	return slack.NewClientWithCache(ws.Token, ws.Cookie, nil, store, cache.WorkspaceKey(ws.URL))
}

// --- JSON output types for MCP tool results ---

type mcpMessageJSON struct {
	Timestamp   string `json:"timestamp"`
	Time        string `json:"time"`
	UserID      string `json:"userID"`
	UserName    string `json:"userName"`
	Text        string `json:"text"`
	ChannelID   string `json:"channelID"`
	ChannelName string `json:"channelName,omitempty"`
	ThreadTS    string `json:"threadTS"`
	ThreadDepth int    `json:"threadDepth"`
}

type mcpSearchResultJSON struct {
	Timestamp   string `json:"timestamp"`
	Time        string `json:"time"`
	UserID      string `json:"userID"`
	UserName    string `json:"userName"`
	Text        string `json:"text"`
	ChannelID   string `json:"channelID"`
	ChannelName string `json:"channelName,omitempty"`
	ThreadTS    string `json:"threadTS"`
	Permalink   string `json:"permalink"`
}

// buildMCPResolver constructs a *slack.Resolver for entity resolution in MCP output.
// Returns nil on error — callers degrade gracefully to raw IDs.
func buildMCPResolver(ctx context.Context, c slackClient) *slack.Resolver {
	users, err := c.ListUsers(ctx)
	if err != nil {
		return nil
	}
	channels, err := c.ListChannels(ctx, nil, false)
	if err != nil {
		return nil
	}
	groups, _ := c.ListUserGroups(ctx)
	fetchUser := func(id string) (string, error) {
		u, fErr := c.FetchUser(ctx, id)
		if fErr != nil {
			return "", fErr
		}
		if u.RealName != "" {
			return u.RealName, nil
		}
		return u.DisplayName, nil
	}
	fetchChannel := func(id string) (string, error) {
		ch, fErr := c.FetchChannel(ctx, id)
		if fErr != nil {
			return "", fErr
		}
		return ch.Name, nil
	}
	fetchGroups := func() ([]slack.UserGroup, error) {
		return c.ForceRefreshUserGroups(ctx)
	}
	return slack.NewResolverWithFetch(users, channels, groups, fetchUser, fetchChannel, fetchGroups)
}

func toMCPMessageJSON(m slack.Message, r *slack.Resolver) mcpMessageJSON {
	userName := m.UserID
	text := m.Text
	channelName := m.ChannelName
	if r != nil {
		userName = r.UserDisplayName(m.UserID)
		text = r.ResolveMentions(text)
		channelName = r.ResolveChannelDisplay(m.ChannelID, m.ChannelName)
	}
	return mcpMessageJSON{
		Timestamp:   m.Timestamp,
		Time:        m.Time.UTC().Format(time.RFC3339),
		UserID:      m.UserID,
		UserName:    userName,
		Text:        text,
		ChannelID:   m.ChannelID,
		ChannelName: channelName,
		ThreadTS:    m.ThreadTS,
		ThreadDepth: m.ThreadDepth,
	}
}

func toMCPSearchResultJSON(sr slack.SearchResult, r *slack.Resolver) mcpSearchResultJSON {
	userName := sr.UserID
	text := sr.Text
	if r != nil {
		userName = r.UserDisplayName(sr.UserID)
		text = r.ResolveMentions(text)
	}
	return mcpSearchResultJSON{
		Timestamp:   sr.Timestamp,
		Time:        sr.Time.UTC().Format(time.RFC3339),
		UserID:      sr.UserID,
		UserName:    userName,
		Text:        text,
		ChannelID:   sr.ChannelID,
		ChannelName: sr.ChannelName,
		ThreadTS:    sr.ThreadTS,
		Permalink:   sr.Permalink,
	}
}

// --- Shared handler helpers ---

// authError returns a CallToolResult with IsError=true for credential failures.
func authError(err error) *mcplib.CallToolResult {
	return mcplib.NewToolResultError(fmt.Sprintf(
		"slack credentials unavailable: %s — ensure the Slack desktop app is running and you are logged in",
		err,
	))
}

// marshalResult marshals v to JSON and wraps it in a text CallToolResult.
// On marshal failure it returns an error result.
func marshalResult(toolName string, v any) *mcplib.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("%s failed: %s", toolName, err))
	}
	return mcplib.NewToolResultText(string(data))
}

// parseStringSlice extracts a []string from an array argument; returns nil when absent.
func parseStringSlice(req mcplib.CallToolRequest, key string) []string {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// resolveOptionalUserName resolves a display name or Slack ID to the user's
// display name for use in search from: modifiers. Returns ("", nil) when empty.
func resolveOptionalUserName(ctx context.Context, nameOrID string, c slackClient) (string, error) {
	if nameOrID == "" {
		return "", nil
	}
	id, err := c.ResolveUser(ctx, nameOrID)
	if err != nil {
		return "", fmt.Errorf("user %q not found — use slack_users to list available users", nameOrID)
	}
	u, err := c.FetchUser(ctx, id)
	if err != nil {
		return "", fmt.Errorf("user %q not found — use slack_users to list available users", nameOrID)
	}
	if u.DisplayName != "" {
		return u.DisplayName, nil
	}
	return u.RealName, nil
}

// buildSearchQuery constructs a Slack full-text query with optional channel and
// date-range filters. Multiple channels are appended as separate in: terms.
// Channel IDs (C…/D…/G…/W…) are passed without '#'; names get in:#name.
// in: terms already present in query are not duplicated.
func buildSearchQuery(query string, channels []string, userName string, dr slack.DateRange) string {
	parts := []string{query}
	for _, ch := range channels {
		ch = strings.TrimLeft(ch, "#@")
		var inTerm string
		if mcpChannelIDPattern.MatchString(ch) {
			inTerm = "in:" + ch
		} else {
			inTerm = "in:#" + ch
		}
		if !strings.Contains(query, inTerm) {
			parts = append(parts, inTerm)
		}
	}
	if userName != "" {
		parts = append(parts, "from:"+userName)
	}
	if dr.From != nil {
		parts = append(parts, "after:"+dr.From.Format("2006-01-02"))
	}
	if dr.To != nil {
		parts = append(parts, "before:"+dr.To.Format("2006-01-02"))
	}
	return strings.Join(parts, " ")
}

// getWorkspaceAndClient resolves workspace selection and builds a Slack client.
func getWorkspaceAndClient(tc *tokenCache, wsSel string, buildClient func(tokens.Workspace) slackClient) (slackClient, error) {
	workspaces, err := tc.get()
	if err != nil {
		return nil, err
	}
	ws, err := selectWorkspace(workspaces, wsSel)
	if err != nil {
		return nil, err
	}
	return buildClient(ws), nil
}

// --- US2: Core Retrieval Tool Handlers ---

// handleSlackSearch handles the slack_search MCP tool call.
func handleSlackSearch(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	workspaces, err := tc.get()
	if err != nil {
		return authError(err), nil
	}
	query, reqErr := req.RequireString("query")
	if reqErr != nil {
		return mcplib.NewToolResultError("query parameter is required"), nil
	}
	ws, wsErr := selectWorkspace(workspaces, req.GetString("workspace", ""))
	if wsErr != nil {
		return mcplib.NewToolResultError(wsErr.Error()), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	c := buildClient(ws)
	userName, uErr := resolveOptionalUserName(ctx, req.GetString("user", ""), c)
	if uErr != nil {
		return mcplib.NewToolResultError(uErr.Error()), nil
	}
	q := buildSearchQuery(query, parseStringSlice(req, "channels"), userName, dr)
	results, sErr := c.SearchMessages(ctx, q, req.GetInt("limit", 100))
	if sErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("search %q failed: %s", query, sErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	out := make([]mcpSearchResultJSON, len(results))
	for i, sr := range results {
		out[i] = toMCPSearchResultJSON(sr, r)
	}
	return marshalResult("slack_search", out), nil
}

// handleSlackHistory handles the slack_history MCP tool call.
func handleSlackHistory(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	channel, chErr := req.RequireString("channel")
	if chErr != nil {
		return mcplib.NewToolResultError("channel parameter is required"), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	channelID, resolveErr := c.ResolveChannel(ctx, channel)
	if resolveErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"channel %q not found — use slack_channels to list available channels", channel)), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	threads := mcplib.ParseBoolean(req, "threads", false)
	msgs, hErr := c.FetchHistory(ctx, channelID, dr, req.GetInt("limit", 100), threads)
	if hErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_history failed: %s", hErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	out := make([]mcpMessageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = toMCPMessageJSON(m, r)
	}
	return marshalResult("slack_history", out), nil
}

// handleSlackMessages handles the slack_messages MCP tool call.
func handleSlackMessages(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	user, uReqErr := req.RequireString("user")
	if uReqErr != nil {
		return mcplib.NewToolResultError("user parameter is required"), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	userID, resolveErr := c.ResolveUser(ctx, user)
	if resolveErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"user %q not found — use slack_users to list available users", user)), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	msgs, mErr := c.GetUserMessages(ctx, userID, "", dr, req.GetInt("limit", 100))
	if mErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_messages failed: %s", mErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	out := make([]mcpMessageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = toMCPMessageJSON(m, r)
	}
	return marshalResult("slack_messages", out), nil
}

// --- US3: Entity Listing Tool Handlers ---

// handleSlackChannels handles the slack_channels MCP tool call.
func handleSlackChannels(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	includeArchived := mcplib.ParseBoolean(req, "include_archived", false)
	channels, err := c.ListChannels(ctx, nil, includeArchived)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_channels failed: %s", err)), nil
	}
	filter := strings.ToLower(req.GetString("filter", ""))
	if filter != "" {
		var filtered []slack.Channel
		for _, ch := range channels {
			if strings.Contains(strings.ToLower(ch.Name), filter) {
				filtered = append(filtered, ch)
			}
		}
		channels = filtered
	}
	return marshalResult("slack_channels", channels), nil
}

// handleSlackUsers handles the slack_users MCP tool call.
func handleSlackUsers(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	users, err := c.ListUsers(ctx)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_users failed: %s", err)), nil
	}
	filter := strings.ToLower(req.GetString("filter", ""))
	if filter != "" {
		var filtered []slack.User
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.DisplayName), filter) ||
				strings.Contains(strings.ToLower(u.RealName), filter) ||
				strings.Contains(strings.ToLower(u.Email), filter) {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}
	return marshalResult("slack_users", users), nil
}

// --- US4: Analysis Tool JSON types and helpers ---

type mcpChannelDigestJSON struct {
	ChannelID    string           `json:"channelID"`
	ChannelName  string           `json:"channelName"`
	MessageCount int              `json:"messageCount"`
	Messages     []mcpMessageJSON `json:"messages"`
}

type mcpIncidentPeriodJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type mcpTimelineRowJSON struct {
	Time    string `json:"time"`
	Who     string `json:"who"`
	Event   string `json:"event"`
	Replies int    `json:"replies"`
}

type mcpIncidentDocJSON struct {
	Channel      string               `json:"channel"`
	Period       mcpIncidentPeriodJSON `json:"period"`
	Participants []string             `json:"participants"`
	Timeline     []mcpTimelineRowJSON `json:"timeline"`
}

type mcpUserCountJSON struct {
	User  string `json:"user"`
	Count int    `json:"count"`
}

type mcpChannelMetricsJSON struct {
	Channel      string             `json:"channel"`
	MessageCount int                `json:"messageCount"`
	ActiveUsers  int                `json:"activeUsers"`
	TopPosters   []mcpUserCountJSON `json:"topPosters"`
	PeakHour     int                `json:"peakHour"`
}

type mcpActionItemJSON struct {
	UserID    string `json:"userID"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

func toMCPChannelDigestJSON(g output.ChannelDigest, r *slack.Resolver) mcpChannelDigestJSON {
	channelID := ""
	if len(g.Messages) > 0 {
		channelID = g.Messages[0].ChannelID
	}
	msgs := make([]mcpMessageJSON, len(g.Messages))
	for i, m := range g.Messages {
		msgs[i] = toMCPMessageJSON(m, r)
	}
	return mcpChannelDigestJSON{
		ChannelID: channelID, ChannelName: g.ChannelName,
		MessageCount: len(g.Messages), Messages: msgs,
	}
}

func toMCPIncidentDocJSON(doc output.IncidentDoc) mcpIncidentDocJSON {
	rows := make([]mcpTimelineRowJSON, len(doc.Timeline))
	for i, r := range doc.Timeline {
		rows[i] = mcpTimelineRowJSON{
			Time: r.Time.UTC().Format(time.RFC3339), Who: r.Who, Event: r.Event, Replies: r.Replies,
		}
	}
	return mcpIncidentDocJSON{
		Channel: doc.Channel,
		Period: mcpIncidentPeriodJSON{
			From: doc.PeriodFrom.UTC().Format(time.RFC3339),
			To:   doc.PeriodTo.UTC().Format(time.RFC3339),
		},
		Participants: doc.Participants,
		Timeline:     rows,
	}
}

func toMCPChannelMetricsJSON(channelName string, m output.ChannelMetrics) mcpChannelMetricsJSON {
	msgCount := 0
	for _, uc := range m.UserCounts {
		msgCount += uc.Count
	}
	peakHour := 0
	for h, cnt := range m.HourlyDist {
		if cnt > m.HourlyDist[peakHour] {
			peakHour = h
		}
	}
	posters := make([]mcpUserCountJSON, len(m.UserCounts))
	for i, uc := range m.UserCounts {
		posters[i] = mcpUserCountJSON{User: uc.DisplayName, Count: uc.Count}
	}
	return mcpChannelMetricsJSON{
		Channel: channelName, MessageCount: msgCount,
		ActiveUsers: len(m.UserCounts), TopPosters: posters, PeakHour: peakHour,
	}
}

// --- US4: Analysis Tool Handlers ---

// handleSlackDigest handles the slack_digest MCP tool call.
func handleSlackDigest(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	user, uReqErr := req.RequireString("user")
	if uReqErr != nil {
		return mcplib.NewToolResultError("user parameter is required"), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	userID, resolveErr := c.ResolveUser(ctx, user)
	if resolveErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"user %q not found — use slack_users to list available users", user)), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	msgs, mErr := c.GetUserMessages(ctx, userID, "", dr, 0)
	if mErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_digest failed: %s", mErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	groups := output.GroupByChannel(msgs)
	out := make([]mcpChannelDigestJSON, len(groups))
	for i, g := range groups {
		out[i] = toMCPChannelDigestJSON(g, r)
	}
	return marshalResult("slack_digest", out), nil
}

// handleSlackPostmortem handles the slack_postmortem MCP tool call.
func handleSlackPostmortem(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	channel, chErr := req.RequireString("channel")
	if chErr != nil {
		return mcplib.NewToolResultError("channel parameter is required"), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	channelID, resolveErr := c.ResolveChannel(ctx, channel)
	if resolveErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"channel %q not found — use slack_channels to list available channels", channel)), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	msgs, hErr := c.FetchHistory(ctx, channelID, dr, 0, true)
	if hErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_postmortem failed: %s", hErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	return marshalResult("slack_postmortem", toMCPIncidentDocJSON(output.BuildIncidentDoc(msgs, r))), nil
}

// handleSlackMetrics handles the slack_metrics MCP tool call.
func handleSlackMetrics(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	channel, chErr := req.RequireString("channel")
	if chErr != nil {
		return mcplib.NewToolResultError("channel parameter is required"), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	channelID, resolveErr := c.ResolveChannel(ctx, channel)
	if resolveErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"channel %q not found — use slack_channels to list available channels", channel)), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	msgs, hErr := c.FetchHistory(ctx, channelID, dr, 0, false)
	if hErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_metrics failed: %s", hErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	return marshalResult("slack_metrics", toMCPChannelMetricsJSON(channel, output.ComputeMetrics(msgs, r))), nil
}

// handleSlackActions handles the slack_actions MCP tool call.
func handleSlackActions(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	channel, chErr := req.RequireString("channel")
	if chErr != nil {
		return mcplib.NewToolResultError("channel parameter is required"), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	channelID, resolveErr := c.ResolveChannel(ctx, channel)
	if resolveErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"channel %q not found — use slack_channels to list available channels", channel)), nil
	}
	dr, drErr := parseDateRange(req.GetString("since", ""), req.GetString("until", ""))
	if drErr != nil {
		return mcplib.NewToolResultError(drErr.Error()), nil
	}
	msgs, hErr := c.FetchHistory(ctx, channelID, dr, 0, false)
	if hErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_actions failed: %s", hErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	items := output.ExtractActions(msgs, r)
	out := make([]mcpActionItemJSON, len(items))
	for i, item := range items {
		out[i] = mcpActionItemJSON{UserID: item.Who, Text: item.Text, Timestamp: item.Timestamp.UTC().Format(time.RFC3339)}
	}
	return marshalResult("slack_actions", out), nil
}

// handleSlackThread handles the slack_thread MCP tool call.
func handleSlackThread(ctx context.Context, req mcplib.CallToolRequest, tc *tokenCache, buildClient func(tokens.Workspace) slackClient) (*mcplib.CallToolResult, error) {
	rawURL, urlErr := req.RequireString("url")
	if urlErr != nil {
		return mcplib.NewToolResultError("url parameter is required"), nil
	}
	pl, parseErr := slack.ParsePermalink(rawURL)
	if parseErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf(
			"thread %q failed: invalid permalink format", rawURL)), nil
	}
	c, cErr := getWorkspaceAndClient(tc, req.GetString("workspace", ""), buildClient)
	if cErr != nil {
		return authError(cErr), nil
	}
	msgs, tErr := c.FetchThread(ctx, pl.ChannelID, pl.ThreadTS)
	if tErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("slack_thread failed: %s", tErr)), nil
	}
	r := buildMCPResolver(ctx, c)
	out := make([]mcpMessageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = toMCPMessageJSON(m, r)
	}
	return marshalResult("slack_thread", out), nil
}
