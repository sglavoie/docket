package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestREADMECommandPaletteBindingStaysCurrent(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	content := string(data)

	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?i)###\s*command palette\s*\(\s*ctrl\+p\s*\)`),
		regexp.MustCompile("(?i)command palette:\\s*`?ctrl\\+p`?"),
		regexp.MustCompile("(?i)`?ctrl\\+p`?\\s*:\\s*command palette"),
	}

	for _, pattern := range forbidden {
		if pattern.MatchString(content) {
			t.Fatalf("README contains stale command palette binding text matching %q", pattern.String())
		}
	}

	required := []string{
		"### Command Palette (:)",
		"`:`: Open command palette",
	}

	for _, line := range required {
		if !strings.Contains(content, line) {
			t.Fatalf("README missing expected command palette documentation: %q", line)
		}
	}
}

func TestREADMEUsesUppercaseVMultiSelectBinding(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "`v`: Toggle multi-select mode") {
		t.Fatal("README contains stale lowercase multi-select binding")
	}
	if !strings.Contains(content, "`V`: Toggle multi-select mode") {
		t.Fatal("README missing uppercase multi-select binding")
	}
}
