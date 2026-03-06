package tokens

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
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
	aesKeyLen           = 16
	cbcIVByte           = 0x20 // Chromium uses 16 space bytes as the AES-CBC IV
	v10Prefix           = "v10"
	v11Prefix           = "v11"
	// version >= 24 adds an extra 32-byte HMAC prefix after the v1x marker.
	chromiumVersion24 = 24
	hmacPrefixLen     = 32
)

// DecryptCookie copies the Chromium-style Cookies SQLite file at dbPath to a
// temp directory, reads the encrypted 'd' cookie for slack.com, and decrypts
// it using AES-128-CBC with a PBKDF2-derived key.  The key is derived from the
// password returned by kr using PBKDF2-HMAC-SHA1 with the given iterations.
func DecryptCookie(dbPath string, kr KeyringReader, iterations int) (string, error) {
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

	encryptedValue, err := readEncryptedCookie(db)
	if err != nil {
		return "", err
	}

	password, err := kr.ReadPassword(slackKeyringService, slackKeyringAccount)
	if err != nil {
		password, err = kr.ReadPassword(slackKeyringService, slackKeyringAccountFallback)
	}
	if err != nil {
		return "", fmt.Errorf("read keyring password for %q: %w", slackKeyringService, err)
	}

	plaintext, err := decryptAESCBC(encryptedValue, password, iterations, version)
	if err != nil {
		return "", fmt.Errorf("decrypt cookie: %w", err)
	}
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
func readEncryptedCookie(db *sql.DB) ([]byte, error) {
	var blob []byte
	err := db.QueryRow(
		`SELECT encrypted_value FROM cookies WHERE host_key LIKE '%slack.com%' AND name='d' LIMIT 1`,
	).Scan(&blob)
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

// decryptAESCBC derives the AES key from password and decrypts the encrypted
// cookie value, stripping the version prefix, decrypting, then removing the
// optional domain-hash prefix and PKCS7 padding.
func decryptAESCBC(encryptedValue, password []byte, iterations, dbVersion int) (string, error) {
	data := encryptedValue

	// Strip the 3-byte v10/v11 version prefix.
	if bytes.HasPrefix(data, []byte(v11Prefix)) {
		data = data[3:]
	} else if bytes.HasPrefix(data, []byte(v10Prefix)) {
		data = data[3:]
	} else {
		return "", fmt.Errorf("unrecognised cookie prefix (not v10/v11)")
	}

	key := pbkdf2.Key(password, []byte(cookieSalt), iterations, aesKeyLen, sha1.New)
	iv := bytes.Repeat([]byte{cbcIVByte}, aes.BlockSize)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create AES cipher: %w", err)
	}
	if len(data)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext length %d is not a multiple of AES block size", len(data))
	}

	plaintext := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, data)

	// Chromium DB version >= 24 prepends a 32-byte SHA-256 domain hash
	// inside the plaintext (after decryption), not in the ciphertext.
	if dbVersion >= chromiumVersion24 {
		if len(plaintext) < hmacPrefixLen {
			return "", fmt.Errorf("plaintext too short for version-%d domain hash prefix", dbVersion)
		}
		plaintext = plaintext[hmacPrefixLen:]
	}

	unpadded, err := pkcs7Unpad(plaintext)
	if err != nil {
		return "", fmt.Errorf("remove PKCS7 padding: %w", err)
	}
	return string(unpadded), nil
}

// pkcs7Unpad removes PKCS7 padding from data.
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
	return data[:len(data)-padLen], nil
}
