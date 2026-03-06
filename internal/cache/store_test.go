package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- WorkspaceKey tests ---

func TestWorkspaceKey_Deterministic(t *testing.T) {
	key1 := WorkspaceKey("https://myworkspace.slack.com")
	key2 := WorkspaceKey("https://myworkspace.slack.com")
	if key1 != key2 {
		t.Fatalf("WorkspaceKey not deterministic: %q != %q", key1, key2)
	}
}

func TestWorkspaceKey_16LowercaseHexChars(t *testing.T) {
	key := WorkspaceKey("https://myworkspace.slack.com")
	if len(key) != 16 {
		t.Fatalf("WorkspaceKey length = %d, want 16", len(key))
	}
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("WorkspaceKey %q contains non-lowercase-hex char %q", key, c)
		}
	}
}

func TestWorkspaceKey_DifferentURLsDifferentKeys(t *testing.T) {
	key1 := WorkspaceKey("https://myworkspace.slack.com")
	key2 := WorkspaceKey("https://otherworkspace.slack.com")
	if key1 == key2 {
		t.Fatal("WorkspaceKey collision for different workspace URLs")
	}
}

// --- Load tests ---

func TestLoad_MissFileAbsent(t *testing.T) {
	s := NewStore(t.TempDir(), time.Hour)
	data, hit, err := s.Load("nokey", "channels")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hit {
		t.Fatal("expected miss, got hit")
	}
	if data != nil {
		t.Fatalf("expected nil data, got %v", data)
	}
}

func TestLoad_HitFreshFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	payload := []byte(`[{"id":"C01"}]`)
	if err := s.Save("key1", "channels", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, hit, err := s.Load("key1", "channels")
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit, got miss")
	}
	if string(data) != string(payload) {
		t.Fatalf("Load: data = %q, want %q", data, payload)
	}
}

func TestLoad_MissStaleFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	payload := []byte(`[{"id":"C01"}]`)
	if err := s.Save("key1", "channels", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, "key1", "channels.json")
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	data, hit, err := s.Load("key1", "channels")
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if hit {
		t.Fatal("expected miss for stale file, got hit")
	}
	if data != nil {
		t.Fatal("expected nil data on stale miss")
	}
}

func TestLoad_MissInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	keyDir := filepath.Join(dir, "key1")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "channels.json"), []byte("not valid json!!!"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, hit, err := s.Load("key1", "channels")
	if err != nil {
		t.Fatalf("Load: unexpected error on invalid JSON: %v", err)
	}
	if hit {
		t.Fatal("expected miss for invalid JSON, got hit")
	}
	if data != nil {
		t.Fatal("expected nil data for invalid JSON miss")
	}
}

// --- Save tests ---

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	payload := []byte(`[{"id":"C01"}]`)
	if err := s.Save("newkey", "channels", payload); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "newkey")); err != nil {
		t.Fatalf("workspace dir not created: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "newkey", "channels.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("file content = %q, want %q", data, payload)
	}
}

func TestSave_NoTempFileLeft(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	if err := s.Save("key1", "channels", []byte(`[]`)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "key1"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temp file not cleaned up: %s", e.Name())
		}
	}
}

func TestSave_UnwritableDir_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	s := NewStore(dir, time.Hour)
	err := s.Save("key1", "channels", []byte(`[]`))
	if err != nil {
		t.Fatalf("Save on unwritable dir: expected nil, got %v", err)
	}
}

// --- Clear tests ---

func TestClear_RemovesWorkspaceSubdir(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	if err := s.Save("key1", "channels", []byte(`[]`)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Clear("key1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "key1")); !os.IsNotExist(err) {
		t.Fatal("workspace dir should be removed after Clear")
	}
}

// --- LoadStable tests ---

// T005: LoadStable bypasses TTL — an expired file is still a hit.
func TestLoadStable_BypassesTTL(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	payload := []byte(`[{"id":"C01"}]`)
	if err := s.Save("key1", "channels", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Backdate the file well past the TTL.
	path := filepath.Join(dir, "key1", "channels.json")
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	// Regular Load should miss.
	_, hit, err := s.Load("key1", "channels")
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if hit {
		t.Fatal("Load: expected miss on stale file, got hit")
	}
	// LoadStable must hit regardless of mod-time.
	data, hit, err := s.LoadStable("key1", "channels")
	if err != nil {
		t.Fatalf("LoadStable: unexpected error: %v", err)
	}
	if !hit {
		t.Fatal("LoadStable: expected hit on stale file, got miss")
	}
	if string(data) != string(payload) {
		t.Fatalf("LoadStable: data = %q, want %q", data, payload)
	}
}

// T006: LoadStable returns miss for absent file.
func TestLoadStable_MissingFile(t *testing.T) {
	s := NewStore(t.TempDir(), time.Hour)
	data, hit, err := s.LoadStable("nokey", "channels")
	if err != nil {
		t.Fatalf("LoadStable: unexpected error: %v", err)
	}
	if hit {
		t.Fatal("LoadStable: expected miss for absent file, got hit")
	}
	if data != nil {
		t.Fatalf("LoadStable: expected nil data, got %v", data)
	}
}

// T006: LoadStable returns miss for invalid JSON.
func TestLoadStable_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	keyDir := filepath.Join(dir, "key1")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "channels.json"), []byte("not valid json!!!"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, hit, err := s.LoadStable("key1", "channels")
	if err != nil {
		t.Fatalf("LoadStable: unexpected error on invalid JSON: %v", err)
	}
	if hit {
		t.Fatal("LoadStable: expected miss for invalid JSON, got hit")
	}
	if data != nil {
		t.Fatal("LoadStable: expected nil data for invalid JSON miss")
	}
}

// T007: SaveStable then LoadStable returns original data.
func TestSaveStable_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	payload := []byte(`[{"ts":"1234.0","user":"U1","text":"hello"}]`)
	if err := s.SaveStable("key1", "history/C01/2026-03-01", payload); err != nil {
		t.Fatalf("SaveStable: %v", err)
	}
	data, hit, err := s.LoadStable("key1", "history/C01/2026-03-01")
	if err != nil {
		t.Fatalf("LoadStable: unexpected error: %v", err)
	}
	if !hit {
		t.Fatal("LoadStable: expected hit after SaveStable, got miss")
	}
	if string(data) != string(payload) {
		t.Fatalf("LoadStable: data = %q, want %q", data, payload)
	}
}

// --- ClearAll tests ---

func TestClearAll_RemovesBaseDir(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, time.Hour)
	if err := s.Save("key1", "channels", []byte(`[]`)); err != nil {
		t.Fatalf("Save key1: %v", err)
	}
	if err := s.Save("key2", "users", []byte(`[]`)); err != nil {
		t.Fatalf("Save key2: %v", err)
	}
	if err := s.ClearAll(); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("base dir should be removed after ClearAll")
	}
}
