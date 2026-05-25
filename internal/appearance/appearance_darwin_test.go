//go:build darwin

package appearance

import (
	"errors"
	"testing"
)

func TestDarwinDetectorDetect(t *testing.T) {
	original := readAppleInterfaceStyle
	defer func() { readAppleInterfaceStyle = original }()

	tests := []struct {
		name string
		out  []byte
		err  error
		want SystemAppearance
	}{
		{name: "dark mode", out: []byte("Dark\n"), want: Dark},
		{name: "light output", out: []byte("Light\n"), want: Light},
		{name: "no output defaults to light", out: nil, want: Light},
		{
			name: "missing key error defaults to light",
			out:  []byte("The domain/default pair of (kCFPreferencesAnyApplication, AppleInterfaceStyle) does not exist\n"),
			err:  errors.New("exit status 1"),
			want: Light,
		},
		{name: "command failure returns unknown", err: errors.New("defaults missing"), want: Unknown},
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
