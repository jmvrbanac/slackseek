//go:build linux

package tokens

import (
	"os"
	"path/filepath"
)

// LinuxPathProvider implements CookiePathProvider for Linux, pointing at
// the Slack desktop application's data directories under ~/.config/Slack/.
type LinuxPathProvider struct{}

// LevelDBPath returns the path to Slack's LevelDB Local Storage directory.
func (l *LinuxPathProvider) LevelDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "Slack", "Local Storage", "leveldb")
}

// CookiePath returns the path to Slack's Chromium Cookies SQLite file.
func (l *LinuxPathProvider) CookiePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "Slack", "Cookies")
}
