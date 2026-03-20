package tokens

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// mockPathProvider implements CookiePathProvider using fixed paths.
type mockPathProvider struct {
	leveldbPath string
	cookiePath  string
}

func (m *mockPathProvider) LevelDBPath() string { return m.leveldbPath }
func (m *mockPathProvider) CookiePath() string  { return m.cookiePath }

// buildTestLevelDB writes a synthetic LevelDB with one team and returns its dir.
func buildTestLevelDB(t *testing.T, name, url, token string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := localConfigV2{}
	cfg.Teams = map[string]struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}{
		"T00000001": {Name: name, URL: url, Token: token},
	}
	writeSyntheticLevelDB(t, dir, cfg)
	return dir
}

// buildTestCookieDB writes a synthetic Cookies SQLite file with a host_key
// derived from workspaceURL (e.g. "https://test.slack.com" → ".test.slack.com").
func buildTestCookieDB(t *testing.T, dir, workspaceURL, cookiePlaintext, password string) string {
	t.Helper()
	dbPath := filepath.Join(dir, "Cookies")
	encrypted := encryptCookie(cookiePlaintext, []byte(password), 1)
	host := parseWorkspaceHost(workspaceURL)
	hostKey := "." + host
	if host == "" {
		hostKey = ".slack.com"
	}
	createSyntheticCookieDB(t, dbPath, hostKey, encrypted, 20)
	return dbPath
}

func TestExtract_SingleWorkspaceHappyPath(t *testing.T) {
	const (
		wsName  = "Test Corp"
		wsURL   = "https://test.slack.com"
		wsToken = "xoxs-test-token"
		cookie  = "session-cookie"
		pw      = "keyring-pw"
	)

	ldbDir := buildTestLevelDB(t, wsName, wsURL, wsToken)
	cookieDir := t.TempDir()
	cookiePath := buildTestCookieDB(t, cookieDir, wsURL, cookie, pw)

	kr := &mockKR{password: []byte(pw)}
	pp := &mockPathProvider{leveldbPath: ldbDir, cookiePath: cookiePath}

	result, err := Extract(kr, pp, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
	ws := result.Workspaces[0]
	if ws.Name != wsName {
		t.Errorf("Name: got %q, want %q", ws.Name, wsName)
	}
	if ws.URL != wsURL {
		t.Errorf("URL: got %q, want %q", ws.URL, wsURL)
	}
	if ws.Token != wsToken {
		t.Errorf("Token: got %q, want %q", ws.Token, wsToken)
	}
	if ws.Cookie != cookie {
		t.Errorf("Cookie: got %q, want %q", ws.Cookie, cookie)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", result.Warnings)
	}
}

func TestExtract_MissingCookieIsNonFatal(t *testing.T) {
	ldbDir := buildTestLevelDB(t, "Corp", "https://corp.slack.com", "xoxs-tok")
	pp := &mockPathProvider{
		leveldbPath: ldbDir,
		cookiePath:  "/nonexistent/path/Cookies",
	}
	kr := &mockKR{password: []byte("pw")}

	result, err := Extract(kr, pp, io.Discard)
	if err != nil {
		t.Fatalf("cookie failure should be non-fatal, got error: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace even without cookie, got %d", len(result.Workspaces))
	}
	if len(result.Warnings) == 0 {
		t.Error("expected at least one warning for cookie failure")
	}
}

func TestExtract_KeyringFailureIsNonFatal(t *testing.T) {
	ldbDir := buildTestLevelDB(t, "Corp", "https://corp.slack.com", "xoxs-tok")
	cookieDir := t.TempDir()
	cookiePath := buildTestCookieDB(t, cookieDir, "https://corp.slack.com", "cookie", "pw")

	kr := &mockKR{password: []byte("wrong-password")}
	pp := &mockPathProvider{leveldbPath: ldbDir, cookiePath: cookiePath}

	// Providing wrong password → decryption will produce garbage text but no
	// error (AES doesn't authenticate). The result depends on implementation
	// details; what matters is that Extract itself doesn't fail.
	result, err := Extract(kr, pp, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
}

func TestExtract_ZeroWorkspacesReturnsError(t *testing.T) {
	// Use a nonexistent LevelDB path so ExtractWorkspaceTokens fails →
	// Extract propagates the error.
	pp := &mockPathProvider{
		leveldbPath: t.TempDir() + "/no_ldb_here",
		cookiePath:  "/nonexistent",
	}
	kr := &mockKR{password: []byte("pw")}

	_, err := Extract(kr, pp, io.Discard)
	if err == nil {
		t.Fatal("expected error when no workspaces found")
	}
}

// buildTwoWorkspaceFixture creates a LevelDB with two workspaces (Acme and Beta)
// and a cookie DB with per-workspace encrypted cookies. Returns ldbDir, dbPath, and pw.
func buildTwoWorkspaceFixture(t *testing.T) (ldbDir, dbPath, pw string) {
	t.Helper()
	pw = "keyring-pw"
	ldbDir = t.TempDir()
	cfg := localConfigV2{}
	cfg.Teams = map[string]struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}{
		"T00000001": {Name: "Acme Corp", URL: "https://acme.slack.com", Token: "xoxs-acme"},
		"T00000002": {Name: "Beta Inc", URL: "https://beta.slack.com", Token: "xoxs-beta"},
	}
	writeSyntheticLevelDB(t, ldbDir, cfg)

	cookieDir := t.TempDir()
	dbPath = filepath.Join(cookieDir, "Cookies")
	createSyntheticCookieDB(t, dbPath, ".acme.slack.com",
		encryptCookie("acme-session", []byte(pw), 1), 20)
	addCookieRow(t, dbPath, ".beta.slack.com",
		encryptCookie("beta-session", []byte(pw), 1))
	return
}

func TestExtract_MultiWorkspacePerWorkspaceCookie(t *testing.T) {
	ldbDir, dbPath, pw := buildTwoWorkspaceFixture(t)
	kr := &mockKR{password: []byte(pw)}
	pp := &mockPathProvider{leveldbPath: ldbDir, cookiePath: dbPath}

	result, err := Extract(kr, pp, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(result.Workspaces))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", result.Warnings)
	}

	byName := make(map[string]Workspace)
	for _, ws := range result.Workspaces {
		byName[ws.Name] = ws
	}

	if byName["Acme Corp"].Cookie != "acme-session" {
		t.Errorf("acme cookie: got %q, want %q", byName["Acme Corp"].Cookie, "acme-session")
	}
	if byName["Beta Inc"].Cookie != "beta-session" {
		t.Errorf("beta cookie: got %q, want %q", byName["Beta Inc"].Cookie, "beta-session")
	}
	if strings.Contains(byName["Acme Corp"].Cookie, "beta-session") {
		t.Error("acme workspace received beta's cookie")
	}
}
