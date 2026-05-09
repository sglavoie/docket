//go:build darwin

package appearance

import (
	"os/exec"
	"strings"
)

type darwinDetector struct{}

func newDetector() Detector { return darwinDetector{} }

// Detect queries macOS for the current interface style.
// An absent AppleInterfaceStyle key means Light mode (macOS convention).
func (d darwinDetector) Detect() SystemAppearance {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return Light
	}
	if strings.TrimSpace(string(out)) == "Dark" {
		return Dark
	}
	return Light
}
