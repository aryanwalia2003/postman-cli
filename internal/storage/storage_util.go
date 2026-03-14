package storage

import (
	"os"
	"path/filepath"
)

// GetDefaultConfigDir returns the default ~/.reqx/ directory.
func GetDefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".postman-cli"), nil
}

// EnsureDirExists creates the directory if it doesn't already exist.
func EnsureDirExists(path string) error {
	return os.MkdirAll(path, 0755)
}
