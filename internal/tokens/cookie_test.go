package tokens

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

// pkcs7Pad pads src to a multiple of blockSize using PKCS7 rules.
func pkcs7Pad(src []byte, blockSize int) []byte {
	padLen := blockSize - (len(src) % blockSize)
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(src, padding...)
}

// encryptCookie derives an AES key from password + "saltysalt" via PBKDF2,
// prepends the "v10" prefix, and returns the ciphertext ready to store in the
// cookies SQLite table. Used only by tests.
func encryptCookie(plaintext string, password []byte, iterations int) []byte {
	key := pbkdf2.Key(password, []byte("saltysalt"), iterations, 16, sha1.New)
	padded := pkcs7Pad([]byte(plaintext), 16)
	iv := bytes.Repeat([]byte{0x20}, 16)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // only in tests
	}
	enc := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(enc, padded)
	return append([]byte("v10"), enc...)
}

// createSyntheticCookieDB writes a minimal Chromium-style cookies SQLite
// database at dbPath with one encrypted 'd' cookie for slack.com.
func createSyntheticCookieDB(t *testing.T, dbPath string, encryptedValue []byte, version int) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("create cookie DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE meta (key TEXT, value TEXT)`)
	if err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO meta VALUES (?, ?)`, "version", version)
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE cookies (
		host_key TEXT, name TEXT, encrypted_value BLOB
	)`)
	if err != nil {
		t.Fatalf("create cookies table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO cookies VALUES (?, ?, ?)`,
		".slack.com", "d", encryptedValue)
	if err != nil {
		t.Fatalf("insert cookie: %v", err)
	}
}

// mockKR is a simple KeyringReader that always returns a fixed password.
type mockKR struct {
	password []byte
}

func (m *mockKR) ReadPassword(_, _ string) ([]byte, error) {
	return m.password, nil
}

// mockFallbackKR returns an error for the primary account and the password for
// the fallback account, allowing tests to exercise the fallback code path.
type mockFallbackKR struct {
	primaryAccount  string
	fallbackAccount string
	password        []byte
}

func (m *mockFallbackKR) ReadPassword(_, account string) ([]byte, error) {
	if account == m.primaryAccount {
		return nil, fmt.Errorf("account %q not found in keyring", account)
	}
	if account == m.fallbackAccount {
		return m.password, nil
	}
	return nil, fmt.Errorf("unknown account %q", account)
}

func TestDecryptCookie_FallbackKeyringAccount(t *testing.T) {
	const plaintext = "test-session-cookie-value"
	const password = "test-password"
	const iterations = 1

	encrypted := encryptCookie(plaintext, []byte(password), iterations)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	createSyntheticCookieDB(t, dbPath, encrypted, 20)

	kr := &mockFallbackKR{
		primaryAccount:  slackKeyringAccount,
		fallbackAccount: slackKeyringAccountFallback,
		password:        []byte(password),
	}
	result, err := DecryptCookie(dbPath, kr, iterations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != plaintext {
		t.Errorf("expected %q, got %q", plaintext, result)
	}
}

func TestDecryptCookie_BothAccountsFail(t *testing.T) {
	const iterations = 1

	encrypted := encryptCookie("value", []byte("pw"), iterations)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	createSyntheticCookieDB(t, dbPath, encrypted, 20)

	// fallbackAccount set to "no-match" so both lookups fail
	kr := &mockFallbackKR{
		primaryAccount:  slackKeyringAccount,
		fallbackAccount: "no-match",
		password:        []byte("pw"),
	}
	_, err := DecryptCookie(dbPath, kr, iterations)
	if err == nil {
		t.Fatal("expected error when both accounts fail")
	}
	if !containsStr(err.Error(), slackKeyringService) {
		t.Errorf("error %q should mention service name %q", err.Error(), slackKeyringService)
	}
}

// containsStr reports whether substr appears within s.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDecryptCookie_HappyPath(t *testing.T) {
	const plaintext = "test-session-cookie-value"
	const password = "test-password"
	const iterations = 1

	encrypted := encryptCookie(plaintext, []byte(password), iterations)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	createSyntheticCookieDB(t, dbPath, encrypted, 20)

	kr := &mockKR{password: []byte(password)}
	result, err := DecryptCookie(dbPath, kr, iterations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != plaintext {
		t.Errorf("expected %q, got %q", plaintext, result)
	}
}

func TestDecryptCookie_MissingCookieFile(t *testing.T) {
	dir := t.TempDir()
	kr := &mockKR{password: []byte("pw")}
	_, err := DecryptCookie(filepath.Join(dir, "NoCookies"), kr, 1)
	if err == nil {
		t.Fatal("expected error for missing cookie file")
	}
}

func TestDecryptCookie_NoCookieForSlack(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("create DB: %v", err)
	}
	_, _ = db.Exec(`CREATE TABLE meta (key TEXT, value TEXT)`)
	_, _ = db.Exec(`INSERT INTO meta VALUES ('version', 20)`)
	_, _ = db.Exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, encrypted_value BLOB)`)
	db.Close()

	kr := &mockKR{password: []byte("pw")}
	_, err = DecryptCookie(dbPath, kr, 1)
	if err == nil {
		t.Fatal("expected error when no slack cookie present")
	}
}

func TestDecryptCookie_WritesToTempAndCleansUp(t *testing.T) {
	// Verify the original file is not modified; test only checks no side effects.
	const plaintext = "side-effect-test"
	const password = "pw"
	const iterations = 1

	encrypted := encryptCookie(plaintext, []byte(password), iterations)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	createSyntheticCookieDB(t, dbPath, encrypted, 20)

	originalStat, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat original: %v", err)
	}

	kr := &mockKR{password: []byte(password)}
	_, err = DecryptCookie(dbPath, kr, iterations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	afterStat, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if originalStat.ModTime() != afterStat.ModTime() {
		t.Error("original cookie file was modified — should use a temp copy")
	}
}
