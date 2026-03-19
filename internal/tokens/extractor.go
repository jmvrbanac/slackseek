package tokens

import (
	"fmt"
	"io"
	"net/url"
)

// diagf writes a formatted diagnostic line to w. No-op if w is nil.
func diagf(w io.Writer, format string, args ...interface{}) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}

// parseWorkspaceHost extracts the hostname from a workspace URL.
// Returns an empty string if rawURL cannot be parsed or has no host.
func parseWorkspaceHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}

// Extract orchestrates workspace token extraction and cookie decryption.
// Diagnostic output is written to diag (pass io.Discard for silence).
// Cookie failures are non-fatal: they are appended to Warnings.
// Returns an error only if zero workspaces are found.
func Extract(kr KeyringReader, pp CookiePathProvider, diag io.Writer) (TokenExtractionResult, error) {
	if diag == nil {
		diag = io.Discard
	}
	var result TokenExtractionResult

	rawTokens, err := ExtractWorkspaceTokens(pp.LevelDBPath())
	if err != nil {
		return result, fmt.Errorf("extract workspace tokens: %w", err)
	}

	iterations := platformPBKDF2Iterations()
	cookiePath := pp.CookiePath()

	result.Workspaces = make([]Workspace, len(rawTokens))
	for i, tok := range rawTokens {
		host := parseWorkspaceHost(tok.URL)
		diagf(diag, "[workspace] %s (%s)", tok.Name, host)
		cookie, cookieErr := DecryptCookie(cookiePath, kr, iterations, host, diag)
		if cookieErr != nil {
			wsLabel := tok.Name
			if host != "" {
				wsLabel = fmt.Sprintf("%s (%s)", tok.Name, host)
			}
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("%s: %v", wsLabel, cookieErr),
			)
		}
		result.Workspaces[i] = Workspace{
			Name:   tok.Name,
			URL:    tok.URL,
			Token:  tok.Token,
			Cookie: cookie,
		}
	}
	return result, nil
}

// DefaultExtract constructs the platform-appropriate KeyringReader and
// CookiePathProvider at runtime and calls Extract with no diagnostic output.
func DefaultExtract() (TokenExtractionResult, error) {
	return Extract(defaultKeyringReader(), defaultPathProvider(), io.Discard)
}

// DefaultExtractDiag is like DefaultExtract but writes diagnostic output to diag.
func DefaultExtractDiag(diag io.Writer) (TokenExtractionResult, error) {
	return Extract(defaultKeyringReader(), defaultPathProvider(), diag)
}

// DefaultPaths returns the platform-appropriate CookiePathProvider.
// Used by auth diagnose to include paths in its report.
func DefaultPaths() CookiePathProvider {
	return defaultPathProvider()
}

// NewDefaultKeyringReader returns the platform-appropriate KeyringReader.
// Exported for use by debug/diagnostic commands.
func NewDefaultKeyringReader() KeyringReader {
	return defaultKeyringReader()
}
