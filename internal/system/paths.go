package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDir returns SoloSet's per-user config directory, creating it if needed.
// It holds the extracted compose file and the generated secret key.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating user config dir: %w", err)
	}
	dir := filepath.Join(base, "SoloSet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	return dir, nil
}

// SecretKey returns a stable, per-machine Superset secret key, generating and
// persisting one on first use. Keeping it stable means sessions survive
// restarts; keeping it local-only is fine because everything runs on loopback.
func SecretKey() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "secret_key")

	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return string(data), nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating secret key: %w", err)
	}
	key := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
		return "", fmt.Errorf("saving secret key: %w", err)
	}
	return key, nil
}
