package tokens

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

// localConfigV2 mirrors the minimal JSON structure of the Slack LevelDB
// `localConfig_v2` value — only the fields we need to extract.
type localConfigV2 struct {
	Teams map[string]struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"teams"`
}

// writeSyntheticLevelDB creates a LevelDB at dir and inserts a `localConfig_v2`
// key with the given teams, plus a spurious unrelated key that must be ignored.
func writeSyntheticLevelDB(t *testing.T, dir string, cfg localConfigV2) {
	t.Helper()
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatalf("open synthetic LevelDB: %v", err)
	}
	defer db.Close()

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal localConfig_v2: %v", err)
	}
	// Chromium Local Storage encoding: prefix 0x01 = UTF-8 string.
	val := append([]byte{0x01}, raw...)
	if err := db.Put([]byte("_slack_leveldb_localConfig_v2"), val, nil); err != nil {
		t.Fatalf("put localConfig_v2: %v", err)
	}
	// Unrelated key — must be ignored.
	if err := db.Put([]byte("unrelated_key"), []byte(`{"ignored":true}`), nil); err != nil {
		t.Fatalf("put unrelated key: %v", err)
	}
}

func TestExtractWorkspaceTokens_HappyPath(t *testing.T) {
	dir := t.TempDir()
	cfg := localConfigV2{}
	cfg.Teams = map[string]struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}{
		"T01234567": {Name: "Acme Corp", URL: "https://acme.slack.com", Token: "xoxs-111-222-333"},
		"T09876543": {Name: "Beta Inc", URL: "https://beta.slack.com", Token: "xoxc-aaa-bbb-ccc"},
	}
	writeSyntheticLevelDB(t, dir, cfg)

	tokens, err := ExtractWorkspaceTokens(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	byName := make(map[string]WorkspaceToken, len(tokens))
	for _, tok := range tokens {
		byName[tok.Name] = tok
	}

	acme, ok := byName["Acme Corp"]
	if !ok {
		t.Fatal("expected token for 'Acme Corp'")
	}
	if acme.Token != "xoxs-111-222-333" {
		t.Errorf("wrong Token: %q", acme.Token)
	}
	if acme.URL != "https://acme.slack.com" {
		t.Errorf("wrong URL: %q", acme.URL)
	}

	beta, ok := byName["Beta Inc"]
	if !ok {
		t.Fatal("expected token for 'Beta Inc'")
	}
	if beta.Token != "xoxc-aaa-bbb-ccc" {
		t.Errorf("wrong Token: %q", beta.Token)
	}
}

func TestExtractWorkspaceTokens_UnrelatedKeysIgnored(t *testing.T) {
	dir := t.TempDir()
	// Write only the unrelated key (no localConfig_v2 key at all via writeSyntheticLevelDB).
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatalf("open LevelDB: %v", err)
	}
	if err := db.Put([]byte("some_other_key"), []byte(`{}`), nil); err != nil {
		t.Fatalf("put key: %v", err)
	}
	db.Close()

	tokens, err := ExtractWorkspaceTokens(dir)
	if err == nil {
		t.Fatal("expected error for zero workspaces, got nil")
	}
	if len(tokens) != 0 {
		t.Errorf("expected no tokens, got %d", len(tokens))
	}
}

func TestExtractWorkspaceTokens_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	// Create an empty LevelDB (no keys).
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatalf("open LevelDB: %v", err)
	}
	db.Close()

	_, err = ExtractWorkspaceTokens(dir)
	if err == nil {
		t.Fatal("expected error for empty LevelDB, got nil")
	}
}

func TestExtractWorkspaceTokens_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	nonexistent := dir + "/does_not_exist"
	if err := os.MkdirAll(nonexistent, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Empty dir — LevelDB won't find a MANIFEST so opening is fine but empty.
	_, err := ExtractWorkspaceTokens(nonexistent)
	if err == nil {
		t.Fatal("expected error for empty/missing LevelDB")
	}
}
