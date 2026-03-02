//go:build linux

package tokens

func platformPBKDF2Iterations() int { return LinuxPBKDF2Iterations }

func defaultKeyringReader() KeyringReader  { return &LinuxKeyringReader{} }
func defaultPathProvider() CookiePathProvider { return &LinuxPathProvider{} }
