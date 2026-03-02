package tokens

// CookiePathProvider resolves the platform-specific paths to the Slack
// LevelDB Local Storage directory and the Chromium Cookies SQLite file.
type CookiePathProvider interface {
	LevelDBPath() string
	CookiePath() string
}
