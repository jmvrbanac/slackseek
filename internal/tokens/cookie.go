package tokens

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO
	"golang.org/x/crypto/pbkdf2"
)

const (
	slackKeyringService         = "Slack Safe Storage"
	slackKeyringAccount         = "Slack"
	slackKeyringAccountFallback = "Slack Key"
	cookieSalt                  = "saltysalt"
	aesKeyLen                   = 16
	cbcIVByte                   = 0x20 // Chromium uses 16 space bytes as the AES-CBC IV
	v10Prefix                   = "v10"
	v11Prefix                   = "v11"
	// chromePrefixLen is the length of the binary binding prefix prepended by
	// Chrome 127+ to plaintext before AES-CBC encryption. It must be stripped
	// after decryption to recover the actual cookie value.
	chromePrefixLen = 32
	// peanutsFallback is Chromium's hardcoded OSCrypt password used when the OS
	// keyring is unavailable (e.g. i3/non-GNOME desktops without GNOME Keyring).
	peanutsFallback = "peanuts"
)

// DecryptCookie copies the Chromium-style Cookies SQLite file at dbPath to a
// temp directory, reads the encrypted 'd' cookie for workspaceHost, and decrypts
// it using AES-128-CBC with a PBKDF2-derived key.  The key is derived from the
// password returned by kr using PBKDF2-HMAC-SHA1 with the given iterations.
// Diagnostic output is written to diag (pass io.Discard for silence).
func DecryptCookie(dbPath string, kr KeyringReader, iterations int, workspaceHost string, diag io.Writer) (string, error) {
	tmpFile, err := copyToTemp(dbPath)
	if err != nil {
		return "", fmt.Errorf("copy cookie DB %s to temp: %w", dbPath, err)
	}
	defer os.Remove(tmpFile)

	db, err := sql.Open("sqlite", tmpFile)
	if err != nil {
		return "", fmt.Errorf("open cookie DB copy: %w", err)
	}
	defer db.Close()

	version, err := readDBVersion(db)
	if err != nil {
		return "", err
	}
	diagf(diag, "[cookie] DB version: %d", version)

	encryptedValue, err := readEncryptedCookie(db, workspaceHost)
	if err != nil {
		return "", err
	}
	diagf(diag, "[cookie] encrypted blob: %d bytes", len(encryptedValue))

	password, err := readKeyringPassword(kr, diag)
	if err != nil {
		return "", err
	}

	plaintext, err := decryptAESCBC(encryptedValue, password, iterations)
	if err != nil {
		diagf(diag, "[decrypt] error: %v", err)
		return "", fmt.Errorf("decrypt cookie: %w", err)
	}
	diagf(diag, "[decrypt] ok (%d bytes)", len(plaintext))
	return plaintext, nil
}

// copyToTemp copies the file at src to a new temp file and returns its path.
func copyToTemp(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open source file %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.CreateTemp("", "slackseek-cookies-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(out.Name())
		return "", fmt.Errorf("copy to temp: %w", err)
	}
	return out.Name(), nil
}

// readDBVersion queries the meta table for the Chromium cookie DB version.
func readDBVersion(db *sql.DB) (int, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM meta WHERE key='version'`).Scan(&val)
	if err != nil {
		return 0, fmt.Errorf("read DB version from meta table: %w", err)
	}
	v, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse DB version %q: %w", val, err)
	}
	return v, nil
}

// readEncryptedCookie fetches the encrypted_value blob for the Slack 'd' cookie.
// It first queries for the workspace-specific host (e.g. "lw.slack.com"), then
// falls back to the generic "slack.com" filter if no row is found. This handles
// both workspace-specific cookies and the common case where Slack stores the 'd'
// cookie with a domain-wide host_key (e.g. ".slack.com").
func readEncryptedCookie(db *sql.DB, workspaceHost string) ([]byte, error) {
	const q = `SELECT encrypted_value FROM cookies WHERE host_key LIKE '%'||?||'%' AND name='d' LIMIT 1`
	if workspaceHost != "" {
		var blob []byte
		err := db.QueryRow(q, workspaceHost).Scan(&blob)
		if err == nil {
			return blob, nil
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("query Slack cookie: %w", err)
		}
		// Fall through to generic slack.com filter below.
	}
	var blob []byte
	err := db.QueryRow(q, "slack.com").Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf(
			"no Slack 'd' cookie found in cookies DB: "+
				"ensure you are logged into Slack in the desktop app",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query Slack cookie: %w", err)
	}
	return blob, nil
}

// readKeyringPassword tries the primary keyring account then the fallback,
// emitting diagnostic output to diag.
func readKeyringPassword(kr KeyringReader, diag io.Writer) ([]byte, error) {
	password, err := kr.ReadPassword(slackKeyringService, slackKeyringAccount)
	if err != nil {
		password, err = kr.ReadPassword(slackKeyringService, slackKeyringAccountFallback)
	}
	if err != nil {
		diagf(diag, "[keyring] error: %v", err)
		return nil, fmt.Errorf("read keyring password for %q: %w", slackKeyringService, err)
	}
	diagf(diag, "[keyring] lookup: ok")
	return password, nil
}

// candidateKeys returns AES keys to try in order, preserving backward
// compatibility. The original PBKDF2 path is always first so that existing
// environments that already work are never broken by the newer derivations.
//
//  1. PBKDF2-HMAC-SHA1(keyring_password) — the original behavior
//  2. raw base64-decoded keyring_password — newer Electron stores the AES key
//     this way; only added when the password is valid standard base64 that
//     decodes to exactly 16 bytes
//  3. PBKDF2-HMAC-SHA1("peanuts") — Chromium's hardcoded OSCrypt fallback for
//     environments without a keyring (e.g. i3/bare X11)
func candidateKeys(password []byte, iterations int) [][]byte {
	keys := [][]byte{
		pbkdf2.Key(password, []byte(cookieSalt), iterations, aesKeyLen, sha1.New),
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(password)); err == nil && len(decoded) == aesKeyLen {
		keys = append(keys, decoded)
	}
	keys = append(keys, pbkdf2.Key([]byte(peanutsFallback), []byte(cookieSalt), iterations, aesKeyLen, sha1.New))
	return keys
}

// decryptAESCBC decrypts the encrypted cookie value. The format used by
// Slack's Electron app on Linux/macOS is:
// v10/v11 prefix (3 bytes) + AES-128-CBC ciphertext + PKCS7 padding.
//
// Keys are tried in the order returned by candidateKeys. The first candidate
// that produces valid PKCS7 padding wins.
//
// Chrome 127+ prepends a 32-byte binary prefix to the plaintext before
// encryption; it is stripped after a successful decrypt.
func decryptAESCBC(encryptedValue, password []byte, iterations int) (string, error) {
	data := encryptedValue

	// Strip the 3-byte v10/v11 version prefix.
	if bytes.HasPrefix(data, []byte(v11Prefix)) {
		data = data[3:]
	} else if bytes.HasPrefix(data, []byte(v10Prefix)) {
		data = data[3:]
	} else {
		return "", fmt.Errorf("unrecognised cookie prefix (not v10/v11)")
	}

	if len(data)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext length %d is not a multiple of AES block size", len(data))
	}

	iv := bytes.Repeat([]byte{cbcIVByte}, aes.BlockSize)

	var lastErr error
	for _, key := range candidateKeys(password, iterations) {
		block, err := aes.NewCipher(key)
		if err != nil {
			return "", fmt.Errorf("create AES cipher: %w", err)
		}
		plaintext := make([]byte, len(data))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, data)
		unpadded, err := pkcs7Unpad(plaintext)
		if err != nil {
			lastErr = err
			continue
		}
		return stripChromeBinaryPrefix(unpadded), nil
	}
	return "", fmt.Errorf("remove PKCS7 padding: %w", lastErr)
}

// stripChromeBinaryPrefix removes the 32-byte binary binding prefix prepended
// by Chrome 127+ before the actual cookie value. The strip is conditional:
// it only applies when the first byte is non-printable (indicating a binary
// prefix) and the byte at offset 32 is printable ASCII (indicating a valid
// cookie value follows). This preserves compatibility with pre-Chrome-127
// installs where no prefix is present.
func stripChromeBinaryPrefix(plain []byte) string {
	if len(plain) > chromePrefixLen &&
		!isPrintableASCII(plain[0]) &&
		isPrintableASCII(plain[chromePrefixLen]) {
		return string(plain[chromePrefixLen:])
	}
	return string(plain)
}

func isPrintableASCII(b byte) bool {
	return b >= 0x20 && b <= 0x7e
}

// pkcs7Unpad removes and validates PKCS7 padding from data.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize {
		return nil, fmt.Errorf("invalid PKCS7 pad byte %d", padLen)
	}
	if len(data) < padLen {
		return nil, fmt.Errorf("data length %d shorter than pad length %d", len(data), padLen)
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid PKCS7 padding at byte %d", i)
		}
	}
	return data[:len(data)-padLen], nil
}
