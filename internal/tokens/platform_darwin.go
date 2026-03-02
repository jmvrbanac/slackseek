//go:build darwin

package tokens

func platformPBKDF2Iterations() int { return DarwinPBKDF2Iterations }

func defaultKeyringReader() KeyringReader      { return &DarwinKeyringReader{} }
func defaultPathProvider() CookiePathProvider { return &DarwinPathProvider{} }
