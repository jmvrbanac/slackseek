package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store is a file-backed, TTL-based cache. Entries are stored as JSON files
// under dir/{key}/{kind}.json. The file's modification time serves as the
// cached-at timestamp; no metadata is embedded in the payload.
type Store struct {
	dir string
	ttl time.Duration
}

// NewStore returns a Store that caches files in dir with the given TTL.
func NewStore(dir string, ttl time.Duration) *Store {
	return &Store{dir: dir, ttl: ttl}
}

// WorkspaceKey returns a 16-char lowercase hex string derived from the
// SHA-256 hash of workspaceURL, safe for use as a cache subdirectory name.
func WorkspaceKey(workspaceURL string) string {
	sum := sha256.Sum256([]byte(workspaceURL))
	return fmt.Sprintf("%x", sum)[:16]
}

// Load reads the cached bytes for key/kind. Returns (data, true, nil) on a
// fresh hit, (nil, false, nil) on any miss, and (nil, false, err) only on
// unexpected I/O errors.
func (s *Store) Load(key, kind string) ([]byte, bool, error) {
	path := filepath.Join(s.dir, key, kind+".json")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cache stat %s: %w", path, err)
	}
	if time.Since(info.ModTime()) > s.ttl {
		return nil, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("cache read %s: %w", path, err)
	}
	if !json.Valid(data) {
		return nil, false, nil
	}
	return data, true, nil
}

// Save atomically writes data to {dir}/{key}/{kind}.json, creating the
// workspace subdirectory as needed. Any write error is printed to stderr
// and nil is returned so the caller is not blocked by a cache failure.
func (s *Store) Save(key, kind string, data []byte) error {
	dir := filepath.Join(s.dir, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write cache: %v\n", err)
		return nil
	}
	target := filepath.Join(dir, kind+".json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write cache: %v\n", err)
		return nil
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintf(os.Stderr, "Warning: could not write cache: %v\n", err)
		return nil
	}
	return nil
}

// Clear removes the workspace subdirectory for the given key and all its
// contents.
func (s *Store) Clear(key string) error {
	return os.RemoveAll(filepath.Join(s.dir, key))
}

// ClearAll removes the entire base cache directory and all its contents.
func (s *Store) ClearAll() error {
	return os.RemoveAll(s.dir)
}
