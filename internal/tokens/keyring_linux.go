//go:build linux

package tokens

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// LinuxPBKDF2Iterations is the number of PBKDF2 iterations used on Linux.
// Chromium on Linux uses 1 iteration for its Gnome/KDE keyring key derivation.
const LinuxPBKDF2Iterations = 1

// LinuxKeyringReader reads the Slack encryption password from the D-Bus
// SecretService (used by GNOME Keyring / KDE Wallet on Linux).
type LinuxKeyringReader struct{}

// ReadPassword retrieves the password stored by Slack in the D-Bus
// SecretService under the "Slack Safe Storage" collection.
func (l *LinuxKeyringReader) ReadPassword(_, _ string) ([]byte, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf(
			"connect to D-Bus session: %w: "+
				"ensure a D-Bus session is running (usually available in a desktop session)",
			err,
		)
	}

	// Use the org.freedesktop.secrets interface to search for the Slack item.
	ss := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")

	// Search all collections for items with the "application" attribute.
	attributes := map[string]string{"application": "Slack"}
	var unlocked, locked []dbus.ObjectPath
	call := ss.Call("org.freedesktop.secrets.Service.SearchItems", 0, attributes)
	if err := call.Store(&unlocked, &locked); err != nil {
		return nil, fmt.Errorf(
			"search D-Bus SecretService for Slack items: %w: "+
				"install and unlock GNOME Keyring or KDE Wallet, then log into Slack",
			err,
		)
	}

	paths := append(unlocked, locked...)
	if len(paths) == 0 {
		return nil, fmt.Errorf(
			"no Slack secret found in D-Bus SecretService: " +
				"open Slack once to create the keyring entry, then retry",
		)
	}

	// Open a session for GetSecret.
	var sessionPath dbus.ObjectPath
	var sessionOutput dbus.Variant
	openCall := ss.Call(
		"org.freedesktop.secrets.Service.OpenSession", 0,
		"plain", dbus.MakeVariant(""),
	)
	if err := openCall.Store(&sessionOutput, &sessionPath); err != nil {
		return nil, fmt.Errorf("open D-Bus secrets session: %w", err)
	}

	// Retrieve the secret from the first matching item.
	item := conn.Object("org.freedesktop.secrets", paths[0])
	var secret struct {
		Session     dbus.ObjectPath
		Parameters  []byte
		Value       []byte
		ContentType string
	}
	getCall := item.Call("org.freedesktop.secrets.Item.GetSecret", 0, sessionPath)
	if err := getCall.Store(&secret); err != nil {
		return nil, fmt.Errorf("get secret from D-Bus item: %w", err)
	}
	return secret.Value, nil
}
