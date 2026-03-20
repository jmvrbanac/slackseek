package cmd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/godbus/dbus/v5"
	"github.com/jmvrbanac/slackseek/internal/tokens"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/pbkdf2"

	_ "modernc.org/sqlite" // registers the sqlite3 driver
)

func newAuthDebugCookieCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "debug-cookie",
		Hidden: true,
		Short:  "Dump raw keyring and cookie bytes for diagnosing AES key derivation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDebugCookie(cmd.OutOrStdout())
		},
	}
}

// runDebugCookie dumps raw keyring bytes and cookie blob hex, then attempts
// decryption with several key derivation strategies. Intended only for
// diagnosing AES key mismatch issues.
func runDebugCookie(out io.Writer) error {
	pp := tokens.DefaultPaths()
	fmt.Fprintf(out, "=== Cookie DB ===\n")
	fmt.Fprintf(out, "Path: %s\n\n", pp.CookiePath())
	db, dbErr := openAndListCookieDB(out, pp.CookiePath())
	if db != nil {
		defer db.Close()
	}

	fmt.Fprintf(out, "=== Keyring Items (D-Bus) ===\n")
	allItems := listAndPrintKeyringItems(out)

	fmt.Fprintf(out, "=== Keyring (via ReadPassword) ===\n")
	kr := tokens.NewDefaultKeyringReader()
	rawPw, krErr := kr.ReadPassword("Slack Safe Storage", "Slack")
	if krErr != nil {
		fmt.Fprintf(out, "ERROR reading keyring: %v\n", krErr)
		return nil
	}
	printKeyringInfo(out, rawPw)

	if dbErr != nil {
		fmt.Fprintf(out, "Cannot attempt decryption: cookie DB not opened.\n")
		return nil
	}

	encBlob := readFirstSlackCookie(db)
	if len(encBlob) < 4 {
		fmt.Fprintf(out, "\nNo usable cookie blob for decryption attempts.\n")
		return nil
	}

	tryAllDecryptions(out, rawPw, encBlob, allItems)
	printEnvVars(out)
	return nil
}

func openAndListCookieDB(out io.Writer, cookiePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cookiePath)
	if err != nil {
		fmt.Fprintf(out, "ERROR opening cookie DB: %v\n\n", err)
		return nil, err
	}
	listCookieRows(out, db)
	return db, nil
}

func listAndPrintKeyringItems(out io.Writer) []keyringItem {
	items, err := listAllKeyringItems()
	if err != nil {
		fmt.Fprintf(out, "ERROR listing keyring items: %v\n\n", err)
		return nil
	}
	printAllKeyringItems(out, items)
	return items
}

func printAllKeyringItems(out io.Writer, items []keyringItem) {
	for i, item := range items {
		fmt.Fprintf(out, "Item %d: %s\n", i, item.path)
		for k, v := range item.attrs {
			fmt.Fprintf(out, "  attr %-35s = %q\n", k, v)
		}
		fmt.Fprintf(out, "  secret len=%d  hex=%s\n", len(item.secret), hex.EncodeToString(item.secret))
		fmt.Fprintf(out, "  secret ASCII=%q\n\n", string(item.secret))
	}
}

func printEnvVars(out io.Writer) {
	fmt.Fprintf(out, "=== Environment ===\n")
	for _, envVar := range []string{"XDG_SESSION_TYPE", "DBUS_SESSION_BUS_ADDRESS", "GNOME_KEYRING_CONTROL"} {
		if v := os.Getenv(envVar); v != "" {
			fmt.Fprintf(out, "%s=%s\n", envVar, v)
		}
	}
}

func tryAllDecryptions(out io.Writer, rawPw, encBlob []byte, allItems []keyringItem) {
	fmt.Fprintf(out, "\n=== Decryption Attempts ===\n")
	fmt.Fprintf(out, "Blob len=%d  prefix=%q\n\n", len(encBlob), string(encBlob[:3]))

	trimPw := trimNullBytes(rawPw)
	var decodedKey []byte
	if dec, err := base64.StdEncoding.DecodeString(string(trimPw)); err == nil && len(dec) == 16 {
		decodedKey = dec
	}

	data := encBlob[3:] // strip v10/v11 prefix
	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = 0x20
	}
	ivZero := make([]byte, aes.BlockSize) // alternative: all-zero IV

	pbkdf2Key := pbkdf2.Key(rawPw, []byte("saltysalt"), 1, 16, sha1.New)
	tryDecrypt(out, "PBKDF2(raw_pw, saltysalt, 1, IV=0x20)", pbkdf2Key, data, iv)
	tryDecrypt(out, "PBKDF2(raw_pw, saltysalt, 1, IV=0x00)", pbkdf2Key, data, ivZero)
	if len(data) >= 32 {
		tryDecrypt(out, "PBKDF2(raw_pw, saltysalt, 1, IV=data[0:16])", pbkdf2Key, data[16:], data[:16])
	}
	if decodedKey != nil {
		tryDecrypt(out, "direct_decoded_key (IV=0x20)", decodedKey, data, iv)
		tryDecrypt(out, "direct_decoded_key (IV=0x00)", decodedKey, data, ivZero)
		if len(data) >= 32 {
			tryDecrypt(out, "direct_decoded_key (IV=data[0:16])", decodedKey, data[16:], data[:16])
		}
		tryDecrypt(out, "PBKDF2(decoded_key, saltysalt, 1, IV=0x20)", pbkdf2.Key(decodedKey, []byte("saltysalt"), 1, 16, sha1.New), data, iv)
	}
	tryDecryptKeyringItems(out, allItems, data, iv)
}

func tryDecryptKeyringItems(out io.Writer, allItems []keyringItem, data, iv []byte) {
	for i, item := range allItems {
		if len(item.secret) == 0 {
			continue
		}
		pw := item.secret
		label := ""
		for k, v := range item.attrs {
			label += fmt.Sprintf("%s=%s ", k, v)
		}
		tryDecrypt(out, fmt.Sprintf("item%d(%s) PBKDF2(IV=0x20)", i, label), pbkdf2.Key(pw, []byte("saltysalt"), 1, 16, sha1.New), data, iv)
		if dec, err2 := base64.StdEncoding.DecodeString(string(pw)); err2 == nil && len(dec) == 16 {
			tryDecrypt(out, fmt.Sprintf("item%d(%s) direct_decoded (IV=0x20)", i, label), dec, data, iv)
		}
	}
}

type keyringItem struct {
	path   dbus.ObjectPath
	attrs  map[string]string
	secret []byte
}

func listAllKeyringItems() ([]keyringItem, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to D-Bus: %w", err)
	}
	ss := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")

	sessionPath, err := openKeyringSession(ss)
	if err != nil {
		return nil, err
	}

	allPaths, err := resolveAllKeyringPaths(conn, ss)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[debug] total keyring items: %d\n", len(allPaths))

	var items []keyringItem
	for _, p := range allPaths {
		items = append(items, fetchKeyringItem(conn, sessionPath, p))
	}
	return items, nil
}

func openKeyringSession(ss dbus.BusObject) (dbus.ObjectPath, error) {
	var sessionOutput dbus.Variant
	var sessionPath dbus.ObjectPath
	if err := ss.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&sessionOutput, &sessionPath); err != nil {
		return "", fmt.Errorf("open session: %w", err)
	}
	return sessionPath, nil
}

func resolveAllKeyringPaths(conn *dbus.Conn, ss dbus.BusObject) ([]dbus.ObjectPath, error) {
	loginCollection := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets/collection/login")
	var itemsVariant dbus.Variant
	if err := loginCollection.Call("org.freedesktop.DBus.Properties.Get", 0,
		"org.freedesktop.Secret.Collection", "Items").Store(&itemsVariant); err != nil {
		var unlocked, locked []dbus.ObjectPath
		if err2 := ss.Call("org.freedesktop.Secret.Service.SearchItems", 0, map[string]string{"application": "Slack"}).Store(&unlocked, &locked); err2 != nil {
			return nil, fmt.Errorf("GetItems and SearchItems both failed: %w / %v", err, err2)
		}
		fmt.Printf("[debug] fallback search: found %d items\n", len(unlocked)+len(locked))
		itemsVariant = dbus.MakeVariant(append(unlocked, locked...))
	}
	allPaths, _ := itemsVariant.Value().([]dbus.ObjectPath)
	return allPaths, nil
}

func fetchKeyringItem(conn *dbus.Conn, sessionPath dbus.ObjectPath, p dbus.ObjectPath) keyringItem {
	item := conn.Object("org.freedesktop.secrets", p)
	var attrs map[string]string
	if err := item.Call("org.freedesktop.Secret.Item.GetAttributes", 0).Store(&attrs); err != nil {
		attrs = map[string]string{"error": err.Error()}
	}

	var secret struct {
		Session     dbus.ObjectPath
		Parameters  []byte
		Value       []byte
		ContentType string
	}
	secretErr := item.Call("org.freedesktop.Secret.Item.GetSecret", 0, sessionPath).Store(&secret)
	var secretVal []byte
	if secretErr == nil {
		secretVal = secret.Value
	}

	return keyringItem{path: p, attrs: attrs, secret: secretVal}
}

func listCookieRows(out io.Writer, db *sql.DB) {
	rows, err := db.Query(
		`SELECT host_key, name, length(encrypted_value), substr(encrypted_value,1,50)
		 FROM cookies WHERE host_key LIKE '%slack%' AND name='d'`)
	if err != nil {
		fmt.Fprintf(out, "ERROR querying cookies: %v\n\n", err)
		return
	}
	defer rows.Close()
	fmt.Fprintf(out, "Rows (host LIKE '%%slack%%', name='d'):\n")
	for rows.Next() {
		var hostKey, name string
		var blobLen int
		var blobHead []byte
		if scanErr := rows.Scan(&hostKey, &name, &blobLen, &blobHead); scanErr != nil {
			fmt.Fprintf(out, "  scan error: %v\n", scanErr)
			continue
		}
		prefix := ""
		if len(blobHead) >= 3 {
			prefix = string(blobHead[:3])
		}
		fmt.Fprintf(out, "  host_key=%-30q  len=%4d  prefix=%q\n", hostKey, blobLen, prefix)
		fmt.Fprintf(out, "    first50: %s\n", hex.EncodeToString(blobHead))
	}
	fmt.Fprintln(out)
}

func readFirstSlackCookie(db *sql.DB) []byte {
	var blob []byte
	_ = db.QueryRow(
		`SELECT encrypted_value FROM cookies WHERE host_key LIKE '%slack%' AND name='d' LIMIT 1`,
	).Scan(&blob)
	return blob
}

func printKeyringInfo(out io.Writer, pw []byte) {
	fmt.Fprintf(out, "Raw len:      %d\n", len(pw))
	fmt.Fprintf(out, "Raw hex:      %s\n", hex.EncodeToString(pw))
	fmt.Fprintf(out, "Raw ASCII:    %q\n", string(pw))
	if len(pw) > 0 {
		fmt.Fprintf(out, "Last byte:    0x%02x\n", pw[len(pw)-1])
	}
	trimmed := trimNullBytes(pw)
	if len(trimmed) != len(pw) {
		fmt.Fprintf(out, "NOTE: null terminator stripped → trimmed len=%d  hex=%s\n",
			len(trimmed), hex.EncodeToString(trimmed))
	}
	if dec, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil {
		fmt.Fprintf(out, "Base64-decode: ok → %d bytes: %s\n", len(dec), hex.EncodeToString(dec))
	} else {
		fmt.Fprintf(out, "Base64-decode: FAILED: %v\n", err)
	}
	fmt.Fprintln(out)
}

func tryDecrypt(out io.Writer, name string, key, data, iv []byte) {
	fmt.Fprintf(out, "--- %s ---\n", name)
	fmt.Fprintf(out, "key hex: %s\n", hex.EncodeToString(key))
	if len(key) != 16 {
		fmt.Fprintf(out, "SKIP: key length %d != 16\n\n", len(key))
		return
	}
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		fmt.Fprintf(out, "SKIP: ciphertext len %d not usable\n\n", len(data))
		return
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Fprintf(out, "SKIP: cipher error: %v\n\n", err)
		return
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, data)

	show := plain
	if len(show) > 80 {
		show = show[:80]
	}
	fmt.Fprintf(out, "plain[0:80] hex: %s\n", hex.EncodeToString(show))

	if !printPKCS7Result(out, plain) {
		fmt.Fprintf(out, "PKCS7 INVALID (last_byte=0x%02x)\n", plain[len(plain)-1])
	}
	fmt.Fprintln(out)
}

// printPKCS7Result validates PKCS7 padding on plain, prints results to out,
// and returns true if padding is valid.
func printPKCS7Result(out io.Writer, plain []byte) bool {
	lastByte := plain[len(plain)-1]
	if lastByte < 1 || lastByte > 16 {
		return false
	}
	padLen := int(lastByte)
	for i := len(plain) - padLen; i < len(plain); i++ {
		if plain[i] != lastByte {
			return false
		}
	}
	unpadded := plain[:len(plain)-padLen]
	end := len(unpadded)
	if end > 120 {
		end = 120
	}
	fmt.Fprintf(out, "PKCS7 VALID (pad=%d, unpadded=%d bytes)\n", padLen, len(unpadded))
	fmt.Fprintf(out, "plaintext: %q\n", string(unpadded[:end]))
	fmt.Fprintf(out, "all printable ASCII: %v\n", isPrintableASCII(unpadded))
	return true
}

func isPrintableASCII(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

func trimNullBytes(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == 0x00 {
		return b[:len(b)-1]
	}
	return b
}
