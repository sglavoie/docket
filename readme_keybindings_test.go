package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readFileForContractTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestREADMECommandPaletteBindingStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "README.md")

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

	// The lean README mentions `:` for the command palette inline;
	// detailed keybinding docs live in-app (tutor / ? help).
	if !strings.Contains(content, "`:`") {
		t.Fatal("README should mention the `:` command palette shortcut")
	}
}

func TestREADMEUsesUppercaseVMultiSelectBinding(t *testing.T) {
	content := readFileForContractTest(t, "README.md")

	// The lean README no longer lists individual keybindings;
	// just ensure stale lowercase binding isn't present.
	if strings.Contains(content, "`v`: Toggle multi-select mode") {
		t.Fatal("README contains stale lowercase multi-select binding")
	}
}

func TestREADMECLIContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "README.md")

	// Ensure stale patterns from the old exhaustive README are gone.
	forbidden := []string{
		"tt project new [name]",
		"tt project show <name|id>",
		"tt project edit <name|id>",
		"tt project archive <name|id>",
		"tt project delete <name|id>",
		"tt project open <name|id>",
		"tt project pin <name|id>",
		"interactive if no name",
		"tt task move-to <id> <target-id>",
		"tt task depends add <id> <blocker-id>",
		"tt note add [title]",
		"--tags <t1,t2>",
		"Configuration is stored at `~/.tt.json`",
	}

	for _, needle := range forbidden {
		if strings.Contains(content, needle) {
			t.Fatalf("README contains stale CLI contract text: %q", needle)
		}
	}

	// The lean README mentions the data directory and key commands,
	// but no longer has an exhaustive CLI reference.
	required := []string{
		"~/.tt/",
		"tt task add",
		"tt project",
	}

	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("README missing expected text: %q", needle)
		}
	}
}

func TestCLIReferenceContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "cmd/cli-reference.md")

	forbidden := []string{
		"config file path (default: ~/.tt.json)",
		"'editor' field in ~/.tt.json",
		"Usage: tt project archive [name|id] [flags]",
		"Usage: tt project delete [name|id] [flags]",
		"Usage: tt project open [name|id] [flags]",
		"Usage: tt project pin [name|id] [flags]",
		"Usage: tt project show [name|id] [flags]",
		"Usage: tt project unarchive [name|id] [flags]",
		"The project directory is moved to ~/.tt-data/.trash/ and can be restored later.",
		"Projects can be specified by name, slug, or ID.",
		"tt select my-project   # Select by name/slug directly",
	}

	for _, needle := range forbidden {
		if strings.Contains(content, needle) {
			t.Fatalf("CLI reference contains stale generated text: %q", needle)
		}
	}

	required := []string{
		"config file path (default: ~/.tt/.tt.json)",
		"'editor' field in ~/.tt/.tt.json",
		"Usage: tt project archive <slug-or-id> [flags]",
		"Usage: tt project delete <slug-or-id> [flags]",
		"Usage: tt project open [slug-or-id] [flags]",
		"Usage: tt project pin <slug-or-id> [flags]",
		"Usage: tt project show [slug-or-id] [flags]",
		"Usage: tt project unarchive <slug-or-id> [flags]",
		"The project directory is moved to ~/.tt/trash/ and can be restored later.",
		"Projects can be specified by slug or ID.",
		"tt select my-project   # Select by slug or ID directly",
	}

	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("CLI reference missing expected generated text: %q", needle)
		}
	}
}
