package tokens

// KeyringReader reads a password from the OS keyring or secret store.
type KeyringReader interface {
	ReadPassword(service, account string) ([]byte, error)
}

// MockKeyringReader implements KeyringReader for use in tests by returning a
// fixed password.
type MockKeyringReader struct {
	Password []byte
	Err      error
}

// ReadPassword returns the configured Password, or Err if it is non-nil.
func (m *MockKeyringReader) ReadPassword(_, _ string) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Password, nil
}
