//go:build !darwin

package handoff

import "os"

// System is nil off macOS, so Switch refuses and a second launch keeps the
// plain "already running" refusal.
func System() Platform { return nil }

// ListenForNudges receives nothing off macOS, where no launch sends one.
func ListenForNudges() (nudges <-chan os.Signal, stop func()) {
	return nil, func() {}
}
