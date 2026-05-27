//go:build darwin

package appearance

import (
	"errors"
	"os/exec"
	"testing"
)

func TestDarwinDetectorDetect(t *testing.T) {
	original := readAppleInterfaceStyle
	defer func() { readAppleInterfaceStyle = original }()

	exitStatusOne := func(t *testing.T) error {
		t.Helper()
		err := exec.Command("sh", "-c", "exit 1").Run()
		if err == nil {
			t.Fatal("expected exit status 1 error")
		}
		return err
	}

	tests := []struct {
		name string
		out  []byte
		err  error
		want SystemAppearance
	}{
		{name: "dark mode", out: []byte("Dark\n"), want: Dark},
		{name: "dark mode lowercase", out: []byte("dark\n"), want: Dark},
		{name: "light output", out: []byte("Light\n"), want: Light},
		{name: "light output lowercase", out: []byte("light\n"), want: Light},
		{name: "no output defaults to light", out: nil, want: Light},
		{
			name: "missing key error defaults to light",
			out:  []byte("The domain/default pair of (kCFPreferencesAnyApplication, AppleInterfaceStyle) does not exist\n"),
			err:  exitStatusOne(t),
			want: Light,
		},
		{
			name: "localized missing key error defaults to light",
			out:  []byte("La paire domaine/valeur par défaut de (kCFPreferencesAnyApplication, AppleInterfaceStyle) n’existe pas\n"),
			err:  exitStatusOne(t),
			want: Light,
		},
		{name: "command failure returns unknown", err: errors.New("defaults missing"), want: Unknown},
		{name: "exit error without key name returns unknown", out: []byte("permission denied"), err: exitStatusOne(t), want: Unknown},
		{
			name: "exit error mentioning key name without domain marker returns unknown",
			out:  []byte("Permission denied reading AppleInterfaceStyle\n"),
			err:  exitStatusOne(t),
			want: Unknown,
		},
		{name: "unexpected successful output returns unknown", out: []byte("Sepia\n"), want: Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readAppleInterfaceStyle = func() ([]byte, error) {
				return tt.out, tt.err
			}

			if got := (darwinDetector{}).Detect(); got != tt.want {
				t.Fatalf("Detect() = %q, want %q", got, tt.want)
			}
		})
	}
}
