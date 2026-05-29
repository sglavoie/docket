//go:build darwin

package appearance

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type darwinDetector struct{}

func newDetector() Detector { return darwinDetector{} }

// readTimeout bounds the defaults invocation so a hung lookup cannot wedge
// callers (notably the TUI's appearance poller).
const readTimeout = 2 * time.Second

var readAppleInterfaceStyle = func() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "defaults", "read", "-g", "AppleInterfaceStyle").CombinedOutput()
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
	switch strings.ToLower(strings.TrimSpace(string(out))) {
	case "light":
		return Light
	case "dark":
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
	if strings.Contains(text, "permission denied") || strings.Contains(text, "operation not permitted") {
		return false
	}
	if !strings.Contains(text, "appleinterfacestyle") {
		return false
	}
	// Require the CoreFoundation preferences domain marker so unrelated
	// failures that happen to mention the key (e.g. a permission error)
	// don't get silently reinterpreted as Light mode.
	return strings.Contains(text, "kcfpreferencesanyapplication")
}
