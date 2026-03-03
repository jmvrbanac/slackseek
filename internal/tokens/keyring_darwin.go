//go:build darwin

package tokens

import (
	"fmt"
	"os/exec"
	"strings"
)

// DarwinPBKDF2Iterations is the number of PBKDF2 iterations used on macOS.
// Chromium on macOS uses 1003 iterations.
const DarwinPBKDF2Iterations = 1003

// DarwinKeyringReader reads the Slack encryption password from the macOS
// login Keychain using the /usr/bin/security command-line tool.
// This avoids a CGo dependency and allows cross-compilation from Linux.
type DarwinKeyringReader struct{}

// ReadPassword retrieves the password stored by Slack in the macOS login
// Keychain under the given service and account names.
func (d *DarwinKeyringReader) ReadPassword(service, account string) ([]byte, error) {
	out, err := exec.Command(
		"/usr/bin/security",
		"find-generic-password",
		"-w", // print password only
		"-s", service,
		"-a", account,
	).Output()
	if err != nil {
		return nil, fmt.Errorf(
			"query macOS Keychain for %q/%q via /usr/bin/security: %w: "+
				"grant Terminal (or slackseek) Keychain access when prompted",
			service, account, err,
		)
	}
	password := strings.TrimRight(string(out), "\n")
	if password == "" {
		return nil, fmt.Errorf(
			"no Keychain entry found for service %q account %q: "+
				"open Slack once to create the Keychain entry, then retry",
			service, account,
		)
	}
	return []byte(password), nil
}
