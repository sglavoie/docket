package gotree

import "testing"

func TestIgnoredMatchesTheGoTool(t *testing.T) {
	for name, want := range map[string]bool{
		".git":      true,
		".cache":    true,
		".tt":       true,
		"_scratch":  true,
		"testdata":  true,
		"internal":  false,
		"macos":     false,
		"vendor":    false,
		"testdata2": false,
		"cmd":       false,
	} {
		if got := Ignored(name); got != want {
			t.Errorf("Ignored(%q) = %v, want %v", name, got, want)
		}
	}
}
