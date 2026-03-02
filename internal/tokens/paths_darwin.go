//go:build darwin

package tokens

import (
	"os"
	"path/filepath"
)

// DarwinPathProvider implements CookiePathProvider for macOS, pointing at the
// Slack desktop application's data directories under ~/Library/Application Support/Slack/.
type DarwinPathProvider struct{}

// LevelDBPath returns the path to Slack's LevelDB Local Storage directory.
func (d *DarwinPathProvider) LevelDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Slack", "Local Storage", "leveldb")
}

// CookiePath returns the path to Slack's Chromium Cookies SQLite file.
func (d *DarwinPathProvider) CookiePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Slack", "Cookies")
}
