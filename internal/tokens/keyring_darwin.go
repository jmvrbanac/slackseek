//go:build darwin

package tokens

import (
	"fmt"

	"github.com/keybase/go-keychain"
)

// DarwinPBKDF2Iterations is the number of PBKDF2 iterations used on macOS.
// Chromium on macOS uses 1003 iterations.
const DarwinPBKDF2Iterations = 1003

// DarwinKeyringReader reads the Slack encryption password from the macOS
// login Keychain using the keybase/go-keychain library.
type DarwinKeyringReader struct{}

// ReadPassword retrieves the password stored by Slack in the macOS login
// Keychain under service "Slack Safe Storage", account "Slack".
func (d *DarwinKeyringReader) ReadPassword(service, account string) ([]byte, error) {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetAccount(account)
	item.SetMatchLimit(keychain.MatchLimitOne)
	item.SetReturnData(true)

	results, err := keychain.QueryItem(item)
	if err != nil {
		return nil, fmt.Errorf(
			"query macOS Keychain for %q/%q: %w: "+
				"grant Terminal (or slackseek) Keychain access when prompted",
			service, account, err,
		)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf(
			"no Keychain entry found for service %q account %q: "+
				"open Slack once to create the Keychain entry, then retry",
			service, account,
		)
	}
	return results[0].Data, nil
}
