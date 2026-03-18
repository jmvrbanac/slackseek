package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/slack"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// mockSlackClient implements slackClient for unit tests.
type mockSlackClient struct {
	searchResults     []slack.SearchResult
	searchErr         error
	historyMsgs       []slack.Message
	historyErr        error
	userMsgs          []slack.Message
	userMsgsErr       error
	threadMsgs        []slack.Message
	threadErr         error
	resolveChannelID  string
	resolveChannelErr error
	resolveUserID     string
	resolveUserErr    error
	fetchUserResult slack.User
	fetchUserErr    error
	// US3 listing fields
	channels                []slack.Channel
	channelsErr             error
	channelsIncludeArchived bool // records last includeArchived arg
	users                   []slack.User
	usersErr                error
}

func (m *mockSlackClient) SearchMessages(_ context.Context, _ string, _ int) ([]slack.SearchResult, error) {
	return m.searchResults, m.searchErr
}
func (m *mockSlackClient) FetchHistory(_ context.Context, _ string, _ slack.DateRange, _ int, _ bool) ([]slack.Message, error) {
	return m.historyMsgs, m.historyErr
}
func (m *mockSlackClient) GetUserMessages(_ context.Context, _, _ string, _ slack.DateRange, _ int) ([]slack.Message, error) {
	return m.userMsgs, m.userMsgsErr
}
func (m *mockSlackClient) FetchThread(_ context.Context, _, _ string) ([]slack.Message, error) {
	return m.threadMsgs, m.threadErr
}
func (m *mockSlackClient) ListChannels(_ context.Context, _ []string, includeArchived bool) ([]slack.Channel, error) {
	m.channelsIncludeArchived = includeArchived
	return m.channels, m.channelsErr
}
func (m *mockSlackClient) ListUsers(_ context.Context) ([]slack.User, error) {
	return m.users, m.usersErr
}
func (m *mockSlackClient) ResolveChannel(_ context.Context, nameOrID string) (string, error) {
	if m.resolveChannelErr != nil {
		return "", m.resolveChannelErr
	}
	if m.resolveChannelID != "" {
		return m.resolveChannelID, nil
	}
	return nameOrID, nil
}
func (m *mockSlackClient) ResolveUser(_ context.Context, nameOrID string) (string, error) {
	if m.resolveUserErr != nil {
		return "", m.resolveUserErr
	}
	if m.resolveUserID != "" {
		return m.resolveUserID, nil
	}
	return nameOrID, nil
}
func (m *mockSlackClient) FetchUser(_ context.Context, _ string) (slack.User, error) {
	return m.fetchUserResult, m.fetchUserErr
}
func (m *mockSlackClient) FetchChannel(_ context.Context, _ string) (slack.Channel, error) {
	return slack.Channel{}, nil
}
func (m *mockSlackClient) ListUserGroups(_ context.Context) ([]slack.UserGroup, error) {
	return nil, nil
}
func (m *mockSlackClient) ForceRefreshUserGroups(_ context.Context) ([]slack.UserGroup, error) {
	return nil, nil
}

// newCallRequest creates a CallToolRequest with the given arguments map.
func newCallRequest(args map[string]any) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Arguments: args,
		},
	}
}

// mockBuilder returns a clientBuilderFn that always returns the given mock.
func mockBuilder(c slackClient) func(tokens.Workspace) slackClient {
	return func(_ tokens.Workspace) slackClient { return c }
}

// successTC returns a tokenCache pre-populated with one workspace.
func successTC() *tokenCache {
	return &tokenCache{extractFn: successExtract("acme")}
}

// errorTC returns a tokenCache whose extractFn always fails.
func errorTC(msg string) *tokenCache {
	return &tokenCache{extractFn: errorExtract(msg)}
}

// extractTextContent returns the text of the first TextContent item in the result.
func extractTextContent(t *testing.T, r *mcplib.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("expected at least one content item")
	}
	tc, ok := r.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", r.Content[0])
	}
	return tc.Text
}

// --- handleSlackSearch tests ---

func TestHandleSlackSearch_Success(t *testing.T) {
	mock := &mockSlackClient{
		searchResults: []slack.SearchResult{
			{
				Message: slack.Message{
					Timestamp:   "1700000000.123456",
					UserID:      "U123",
					Text:        "hello world",
					ChannelID:   "C123",
					ChannelName: "general",
				},
				Permalink: "https://acme.slack.com/archives/C123/p1700000000123456",
			},
		},
	}
	req := newCallRequest(map[string]any{"query": "hello"})
	result, err := handleSlackSearch(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true: %v", result.Content)
	}
	text := extractTextContent(t, result)
	var results []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &results); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["text"] != "hello world" {
		t.Errorf("expected text 'hello world', got %v", results[0]["text"])
	}
}

func TestHandleSlackSearch_MissingQuery(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackSearch(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing query")
	}
}

func TestHandleSlackSearch_SlackAPIError(t *testing.T) {
	mock := &mockSlackClient{searchErr: errors.New("rate limited")}
	req := newCallRequest(map[string]any{"query": "hello"})
	result, err := handleSlackSearch(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for Slack API error")
	}
}

func TestHandleSlackSearch_AuthError(t *testing.T) {
	req := newCallRequest(map[string]any{"query": "hello"})
	result, err := handleSlackSearch(context.Background(), req, errorTC("no creds"), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for auth error")
	}
}

// --- buildSearchQuery tests ---

func TestBuildSearchQuery_ChannelIDNoHash(t *testing.T) {
	q := buildSearchQuery("", []string{"C01234567"}, "", slack.DateRange{})
	if !strings.Contains(q, "in:C01234567") {
		t.Errorf("expected in:C01234567 (no hash), got: %q", q)
	}
	if strings.Contains(q, "in:#C01234567") {
		t.Errorf("unexpected hash prefix for channel ID in: %q", q)
	}
}

func TestBuildSearchQuery_ChannelNameHash(t *testing.T) {
	q := buildSearchQuery("", []string{"general"}, "", slack.DateRange{})
	if !strings.Contains(q, "in:#general") {
		t.Errorf("expected in:#general, got: %q", q)
	}
}

func TestBuildSearchQuery_DeduplicatesInModifier(t *testing.T) {
	q := buildSearchQuery("in:#general", []string{"general"}, "", slack.DateRange{})
	count := strings.Count(q, "in:#general")
	if count != 1 {
		t.Errorf("expected exactly one in:#general, got %d in: %q", count, q)
	}
}

func TestHandleSlackSearch_UserResolvedToDisplayName(t *testing.T) {
	capturedQuery := ""
	mock := &mockSlackClient{
		resolveUserID:   "U123",
		fetchUserResult: slack.User{DisplayName: "alice"},
		searchResults:   []slack.SearchResult{},
	}
	// Override SearchMessages to capture the query
	mock2 := &captureQueryMock{mockSlackClient: mock}
	req := newCallRequest(map[string]any{"query": "", "user": "alice"})
	_, err := handleSlackSearch(context.Background(), req, successTC(), mockBuilder(mock2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	capturedQuery = mock2.lastQuery
	if !strings.Contains(capturedQuery, "from:alice") {
		t.Errorf("expected from:alice in query, got: %q", capturedQuery)
	}
	if strings.Contains(capturedQuery, "from:U123") {
		t.Errorf("expected display name not user ID in query, got: %q", capturedQuery)
	}
}

// captureQueryMock wraps mockSlackClient and records the query passed to SearchMessages.
type captureQueryMock struct {
	*mockSlackClient
	lastQuery string
}

func (c *captureQueryMock) SearchMessages(ctx context.Context, query string, limit int) ([]slack.SearchResult, error) {
	c.lastQuery = query
	return c.mockSlackClient.SearchMessages(ctx, query, limit)
}

// --- handleSlackHistory tests ---

func TestHandleSlackHistory_Success(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "msg1", ChannelID: "C123"},
		},
	}
	req := newCallRequest(map[string]any{"channel": "general"})
	result, err := handleSlackHistory(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var msgs []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &msgs); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestHandleSlackHistory_MissingChannel(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackHistory(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing channel")
	}
}

func TestHandleSlackHistory_ChannelNotFound(t *testing.T) {
	mock := &mockSlackClient{
		resolveChannelErr: errors.New("channel not found"),
	}
	req := newCallRequest(map[string]any{"channel": "missing"})
	result, err := handleSlackHistory(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for channel not found")
	}
	text := extractTextContent(t, result)
	if !strings.Contains(text, "slack_channels") {
		t.Errorf("expected hint to use slack_channels, got: %s", text)
	}
}

// --- handleSlackMessages tests ---

func TestHandleSlackMessages_Success(t *testing.T) {
	mock := &mockSlackClient{
		userMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "my message"},
		},
	}
	req := newCallRequest(map[string]any{"user": "john"})
	result, err := handleSlackMessages(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var msgs []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &msgs); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestHandleSlackMessages_MissingUser(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackMessages(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing user")
	}
}

func TestHandleSlackMessages_UserNotFound(t *testing.T) {
	mock := &mockSlackClient{
		resolveUserErr: errors.New("user not found"),
	}
	req := newCallRequest(map[string]any{"user": "nobody"})
	result, err := handleSlackMessages(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for user not found")
	}
	text := extractTextContent(t, result)
	if !strings.Contains(text, "slack_users") {
		t.Errorf("expected hint to use slack_users, got: %s", text)
	}
}

// --- handleSlackThread tests ---

func TestHandleSlackThread_Success(t *testing.T) {
	mock := &mockSlackClient{
		threadMsgs: []slack.Message{
			{Timestamp: "1700000000.123456", UserID: "U123", Text: "thread msg"},
		},
	}
	req := newCallRequest(map[string]any{
		"url": "https://acme.slack.com/archives/C01234567/p1700000000123456",
	})
	result, err := handleSlackThread(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var msgs []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &msgs); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestHandleSlackThread_MissingURL(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackThread(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing url")
	}
}

func TestHandleSlackThread_InvalidPermalink(t *testing.T) {
	req := newCallRequest(map[string]any{"url": "not-a-valid-url"})
	result, err := handleSlackThread(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for invalid permalink")
	}
}

// --- handleSlackChannels tests ---

func TestHandleSlackChannels_Success(t *testing.T) {
	mock := &mockSlackClient{
		channels: []slack.Channel{
			{ID: "C1", Name: "general", Type: "public_channel"},
			{ID: "C2", Name: "random", Type: "public_channel"},
		},
	}
	req := newCallRequest(map[string]any{})
	result, err := handleSlackChannels(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var channels []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &channels); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
}

func TestHandleSlackChannels_Filter(t *testing.T) {
	mock := &mockSlackClient{
		channels: []slack.Channel{
			{ID: "C1", Name: "general", Type: "public_channel"},
			{ID: "C2", Name: "engineering", Type: "public_channel"},
			{ID: "C3", Name: "random", Type: "public_channel"},
		},
	}
	req := newCallRequest(map[string]any{"filter": "eng"})
	result, err := handleSlackChannels(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var channels []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &channels); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 filtered channel, got %d", len(channels))
	}
	if channels[0]["name"] != "engineering" {
		t.Errorf("expected 'engineering', got %v", channels[0]["name"])
	}
}

func TestHandleSlackChannels_IncludeArchived(t *testing.T) {
	mock := &mockSlackClient{}
	req := newCallRequest(map[string]any{"include_archived": true})
	_, err := handleSlackChannels(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.channelsIncludeArchived {
		t.Error("expected include_archived=true to be forwarded to ListChannels")
	}
}

func TestHandleSlackChannels_AuthError(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackChannels(context.Background(), req, errorTC("no creds"), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for auth error")
	}
}

// --- handleSlackUsers tests ---

func TestHandleSlackUsers_Success(t *testing.T) {
	mock := &mockSlackClient{
		users: []slack.User{
			{ID: "U1", DisplayName: "alice", RealName: "Alice Smith", Email: "alice@example.com"},
			{ID: "U2", DisplayName: "bob", RealName: "Bob Jones", Email: "bob@example.com"},
		},
	}
	req := newCallRequest(map[string]any{})
	result, err := handleSlackUsers(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var users []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &users); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestHandleSlackUsers_FilterByDisplayName(t *testing.T) {
	mock := &mockSlackClient{
		users: []slack.User{
			{ID: "U1", DisplayName: "alice", RealName: "Alice Smith", Email: "alice@example.com"},
			{ID: "U2", DisplayName: "bob", RealName: "Bob Jones", Email: "bob@example.com"},
		},
	}
	req := newCallRequest(map[string]any{"filter": "ali"})
	result, err := handleSlackUsers(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var users []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &users); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 filtered user, got %d", len(users))
	}
	if users[0]["displayName"] != "alice" {
		t.Errorf("expected 'alice', got %v", users[0]["displayName"])
	}
}

func TestHandleSlackUsers_FilterByEmail(t *testing.T) {
	mock := &mockSlackClient{
		users: []slack.User{
			{ID: "U1", DisplayName: "alice", Email: "alice@acme.com"},
			{ID: "U2", DisplayName: "bob", Email: "bob@other.com"},
		},
	}
	req := newCallRequest(map[string]any{"filter": "acme"})
	result, err := handleSlackUsers(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextContent(t, result)
	var users []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &users); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user filtered by email domain, got %d", len(users))
	}
}

func TestHandleSlackUsers_AuthError(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackUsers(context.Background(), req, errorTC("no creds"), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for auth error")
	}
}

// --- handleSlackDigest tests ---

func TestHandleSlackDigest_Success(t *testing.T) {
	mock := &mockSlackClient{
		userMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "msg1", ChannelID: "C1", ChannelName: "general"},
			{Timestamp: "1700000001.000000", UserID: "U123", Text: "msg2", ChannelID: "C2", ChannelName: "engineering"},
		},
	}
	req := newCallRequest(map[string]any{"user": "john"})
	result, err := handleSlackDigest(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true: %v", extractTextContent(t, result))
	}
	text := extractTextContent(t, result)
	var groups []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &groups); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 channel groups, got %d", len(groups))
	}
}

func TestHandleSlackDigest_MissingUser(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackDigest(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing user")
	}
}

func TestHandleSlackDigest_UserNotFound(t *testing.T) {
	mock := &mockSlackClient{resolveUserErr: errors.New("not found")}
	req := newCallRequest(map[string]any{"user": "nobody"})
	result, err := handleSlackDigest(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for user not found")
	}
	if !strings.Contains(extractTextContent(t, result), "slack_users") {
		t.Error("expected hint to use slack_users")
	}
}

// --- handleSlackPostmortem tests ---

func TestHandleSlackPostmortem_Success(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "deploy completed", ChannelID: "C1"},
		},
	}
	req := newCallRequest(map[string]any{"channel": "incidents"})
	result, err := handleSlackPostmortem(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var doc map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &doc); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if _, ok := doc["channel"]; !ok {
		t.Error("expected 'channel' field in postmortem JSON")
	}
	if _, ok := doc["period"]; !ok {
		t.Error("expected 'period' field in postmortem JSON")
	}
}

func TestHandleSlackPostmortem_MissingChannel(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackPostmortem(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing channel")
	}
}

func TestHandleSlackPostmortem_ChannelNotFound(t *testing.T) {
	mock := &mockSlackClient{resolveChannelErr: errors.New("not found")}
	req := newCallRequest(map[string]any{"channel": "missing"})
	result, err := handleSlackPostmortem(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for channel not found")
	}
	if !strings.Contains(extractTextContent(t, result), "slack_channels") {
		t.Error("expected hint to use slack_channels")
	}
}

// --- handleSlackMetrics tests ---

func TestHandleSlackMetrics_Success(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "hello"},
			{Timestamp: "1700000001.000000", UserID: "U456", Text: "world"},
		},
	}
	req := newCallRequest(map[string]any{"channel": "general"})
	result, err := handleSlackMetrics(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var metrics map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &metrics); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if metrics["messageCount"] != float64(2) {
		t.Errorf("expected messageCount=2, got %v", metrics["messageCount"])
	}
	if metrics["activeUsers"] != float64(2) {
		t.Errorf("expected activeUsers=2, got %v", metrics["activeUsers"])
	}
}

func TestHandleSlackMetrics_MissingChannel(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackMetrics(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing channel")
	}
}

// --- handleSlackActions tests ---

func TestHandleSlackActions_Success(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "I'll follow up on this"},
			{Timestamp: "1700000001.000000", UserID: "U456", Text: "no commitment here"},
		},
	}
	req := newCallRequest(map[string]any{"channel": "general"})
	result, err := handleSlackActions(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got IsError=true")
	}
	text := extractTextContent(t, result)
	var items []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &items); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	// The message with "I'll" should be extracted as an action item
	if len(items) != 1 {
		t.Fatalf("expected 1 action item, got %d", len(items))
	}
}

func TestHandleSlackActions_MissingChannel(t *testing.T) {
	req := newCallRequest(map[string]any{})
	result, err := handleSlackActions(context.Background(), req, successTC(), mockBuilder(&mockSlackClient{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing channel")
	}
}

func TestHandleSlackActions_ChannelNotFound(t *testing.T) {
	mock := &mockSlackClient{resolveChannelErr: errors.New("not found")}
	req := newCallRequest(map[string]any{"channel": "missing"})
	result, err := handleSlackActions(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for channel not found")
	}
	if !strings.Contains(extractTextContent(t, result), "slack_channels") {
		t.Error("expected hint to use slack_channels")
	}
}

// --- buildMCPResolver tests ---

func TestBuildMCPResolver_PopulatesUserMap(t *testing.T) {
	mock := &mockSlackClient{
		users: []slack.User{
			{ID: "U123", RealName: "Alice Smith", DisplayName: "alice"},
		},
		channels: []slack.Channel{},
	}
	r := buildMCPResolver(context.Background(), mock)
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	if got := r.UserDisplayName("U123"); got != "Alice Smith" {
		t.Errorf("expected 'Alice Smith', got %q", got)
	}
}

func TestBuildMCPResolver_NilOnListUsersError(t *testing.T) {
	mock := &mockSlackClient{usersErr: errors.New("api down")}
	r := buildMCPResolver(context.Background(), mock)
	if r != nil {
		t.Error("expected nil resolver when ListUsers fails")
	}
}

// --- entity resolution in handler output tests ---

func TestHandleSlackHistory_ResolvesUserName(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "hello", ChannelID: "C1"},
		},
		users:    []slack.User{{ID: "U123", RealName: "Alice Smith"}},
		channels: []slack.Channel{},
	}
	req := newCallRequest(map[string]any{"channel": "general"})
	result, err := handleSlackHistory(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextContent(t, result)
	var msgs []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &msgs); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if msgs[0]["userName"] != "Alice Smith" {
		t.Errorf("expected userName 'Alice Smith', got %v", msgs[0]["userName"])
	}
}

func TestHandleSlackHistory_ResolvesMentions(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U456", Text: "ping <@U123>", ChannelID: "C1"},
		},
		users:    []slack.User{{ID: "U123", RealName: "Alice Smith"}},
		channels: []slack.Channel{},
	}
	req := newCallRequest(map[string]any{"channel": "general"})
	result, err := handleSlackHistory(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextContent(t, result)
	var msgs []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &msgs); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if msgs[0]["text"] != "ping @Alice Smith" {
		t.Errorf("expected resolved mention, got %v", msgs[0]["text"])
	}
}

func TestHandleSlackSearch_ResolvesUserName(t *testing.T) {
	mock := &mockSlackClient{
		searchResults: []slack.SearchResult{
			{
				Message: slack.Message{
					Timestamp: "1700000000.000000",
					UserID:    "U123",
					Text:      "found it",
					ChannelID: "C1",
				},
			},
		},
		users:    []slack.User{{ID: "U123", RealName: "Alice Smith"}},
		channels: []slack.Channel{},
	}
	req := newCallRequest(map[string]any{"query": "found"})
	result, err := handleSlackSearch(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextContent(t, result)
	var results []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &results); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if results[0]["userName"] != "Alice Smith" {
		t.Errorf("expected userName 'Alice Smith', got %v", results[0]["userName"])
	}
}

func TestHandleSlackHistory_FallsBackToRawIDOnListUsersError(t *testing.T) {
	mock := &mockSlackClient{
		historyMsgs: []slack.Message{
			{Timestamp: "1700000000.000000", UserID: "U123", Text: "hello", ChannelID: "C1"},
		},
		usersErr: errors.New("api error"),
	}
	req := newCallRequest(map[string]any{"channel": "general"})
	result, err := handleSlackHistory(context.Background(), req, successTC(), mockBuilder(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractTextContent(t, result)
	var msgs []map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &msgs); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jsonErr, text)
	}
	if msgs[0]["userName"] != "U123" {
		t.Errorf("expected raw userID as fallback, got %v", msgs[0]["userName"])
	}
}
