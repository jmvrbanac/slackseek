package tokens

import (
	"fmt"
)

// Extract orchestrates workspace token extraction and cookie decryption.
// It calls ExtractWorkspaceTokens(pp.LevelDBPath()) to enumerate workspaces,
// then calls DecryptCookie(pp.CookiePath(), kr, platformIterations) for each.
// Cookie failures are non-fatal: they are appended to Warnings and extraction
// continues.  Returns an error if zero workspaces are found.
func Extract(kr KeyringReader, pp CookiePathProvider) (TokenExtractionResult, error) {
	var result TokenExtractionResult

	rawTokens, err := ExtractWorkspaceTokens(pp.LevelDBPath())
	if err != nil {
		return result, fmt.Errorf("extract workspace tokens: %w", err)
	}

	iterations := platformPBKDF2Iterations()
	cookie, cookieErr := DecryptCookie(pp.CookiePath(), kr, iterations)
	if cookieErr != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("cookie decryption failed (non-fatal): %v", cookieErr),
		)
	}

	result.Workspaces = make([]Workspace, len(rawTokens))
	for i, tok := range rawTokens {
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
// CookiePathProvider at runtime and calls Extract.
func DefaultExtract() (TokenExtractionResult, error) {
	kr := defaultKeyringReader()
	pp := defaultPathProvider()
	return Extract(kr, pp)
}
