package tokens

// Workspace represents a single authenticated Slack workspace discovered
// from local storage.
type Workspace struct {
	Name   string // human-readable workspace name, e.g. "Acme Corp"
	URL    string // workspace base URL, e.g. "https://acme.slack.com"
	Token  string // user token (xoxs-* or xoxc-*); never written to disk
	Cookie string // decrypted value of the 'd' session cookie
}

// TokenExtractionResult aggregates all workspaces found in local storage
// along with any non-fatal warnings from the extraction process.
type TokenExtractionResult struct {
	Workspaces []Workspace
	Warnings   []string // non-fatal issues encountered during extraction
}
