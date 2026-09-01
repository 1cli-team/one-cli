// Package userdirs centralizes cross-platform user directory resolution.
package userdirs

import (
	"errors"
	"os"
	"strings"
)

// Home returns the effective home directory. HOME wins when explicitly set,
// including on Windows, which keeps Git Bash, CI, and isolated test runs from
// unexpectedly writing to USERPROFILE. Native Windows sessions fall back to
// os.UserHomeDir and therefore still use USERPROFILE.
func Home() (string, error) {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("user home directory is empty")
	}
	return home, nil
}
