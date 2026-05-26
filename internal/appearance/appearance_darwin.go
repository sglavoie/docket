//go:build darwin

package appearance

import (
	"errors"
	"os/exec"
	"strings"
)

type darwinDetector struct{}

func newDetector() Detector { return darwinDetector{} }

var readAppleInterfaceStyle = func() ([]byte, error) {
	return exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").CombinedOutput()
}

// Detect queries macOS for the current interface style.
// An absent AppleInterfaceStyle key means Light mode (macOS convention).
func (d darwinDetector) Detect() SystemAppearance {
	out, err := readAppleInterfaceStyle()
	if err != nil {
		if isMissingAppleInterfaceStyle(err, out) {
			return Light
		}
		return Unknown
	}
	return parseAppleInterfaceStyle(out)
}

func parseAppleInterfaceStyle(out []byte) SystemAppearance {
	switch strings.TrimSpace(string(out)) {
	case "", "Light":
		return Light
	case "Dark":
		return Dark
	default:
		return Unknown
	}
}

func isMissingAppleInterfaceStyle(err error, out []byte) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	text := strings.ToLower(string(out))
	return strings.Contains(text, "appleinterfacestyle")
}
