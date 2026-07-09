package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func readFileForContractTest(t *testing.T, path string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestReadFileForContractTestUsesStableBaseDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})

	if err := os.Chdir("cmd"); err != nil {
		t.Fatalf("chdir to cmd: %v", err)
	}

	content := readFileForContractTest(t, "README.md")
	if !strings.Contains(content, "# tt - Terminal Tasks") {
		t.Fatal("expected README content to load regardless of cwd")
	}
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

func TestREADMEDescribesTodayCockpitDefault(t *testing.T) {
	content := readFileForContractTest(t, "README.md")

	required := []string{
		"`tt` opens on the Today cockpit by default",
		"press `;w` to return to it",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("README missing Today cockpit default text: %q", needle)
		}
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
		"iCloud Drive",
		"never synced",
		"tt task add",
		"tt project",
		"tt task archive list",
		"completed.md",
		".completed.index.json",
	}

	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("README missing expected text: %q", needle)
		}
	}
}

func TestREADMEUndoCommandDescribesMutation(t *testing.T) {
	content := readFileForContractTest(t, "README.md")

	if strings.Contains(content, "tt undo                         # Inspect undo history") {
		t.Fatal("README describes mutating undo command as a read-only history inspection")
	}

	if !strings.Contains(content, "tt undo                         # Restore previous task state") {
		t.Fatal("README should describe tt undo as restoring previous task state")
	}
}

func TestREADMEDocumentsParkedFiltering(t *testing.T) {
	content := readFileForContractTest(t, "README.md")

	required := []string{
		"tt task list --parked",
		"`Fk`",
		"Filter parked tasks",
		"All Tasks includes a built-in `Parked` tab",
		"the seven built-in All Tasks tabs fill the visible cap",
		"the first pinned saved-filter tab can put `By Score` and saved-filter tabs behind `More…`",
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("README missing parked filtering text: %q", needle)
		}
	}
}

func TestCLIReferenceContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "cmd/cli-reference.md")

	forbidden := []string{
		"config file path (default: ~/.tt.json)",
		"config file path (default: ~/.tt/.tt.json)",
		"'editor' field in ~/.tt.json",
		"'editor' field in ~/.tt/.tt.json",
		"The project directory is moved to ~/.tt/trash/ and can be restored later.",
		"Export a project to a human-readable Markdown file in ~/.tt/exports/<slug>.md.",
		"Usage: tt project archive [name|id] [flags]",
		"Usage: tt project delete [name|id] [flags]",
		"Usage: tt project open [name|id] [flags]",
		"Usage: tt project pin [name|id] [flags]",
		"Usage: tt project show [name|id] [flags]",
		"Usage: tt project unarchive [name|id] [flags]",
		"The project directory is moved to ~/.tt-data/.trash/ and can be restored later.",
		"Projects can be specified by name, slug, or ID.",
		"tt select my-project   # Select by name/slug directly",
		"Manage focus board actions",
		"Commands for managing focus board actions",
		"Add a new action to the focus board",
		"Tasks are added to the daily plan by pressing 'w'",
	}

	for _, needle := range forbidden {
		if strings.Contains(content, needle) {
			t.Fatalf("CLI reference contains stale generated text: %q", needle)
		}
	}

	required := []string{
		"config file path (default: ~/.tt/.tt.json, or the iCloud Drive tt folder when available)",
		"'editor' field in the tt config file",
		"The project directory is moved to the trash/ folder in your tt data directory and can be restored later.",
		"Export a project to a human-readable Markdown file in the exports/ folder of your tt data directory.",
		"## tt task archive list",
		"Usage: tt task archive show <id-or-prefix> [flags]",
		"## tt task archive rebuild-index",
		"completed.md and completed.N.md",
		"Usage: tt project archive <slug-or-id> [flags]",
		"Usage: tt project delete <slug-or-id> [flags]",
		"Usage: tt project open [slug-or-id] [flags]",
		"Usage: tt project pin <slug-or-id> [flags]",
		"Usage: tt project show [slug-or-id] [flags]",
		"Usage: tt project unarchive <slug-or-id> [flags]",
		"Projects can be specified by slug or ID.",
		"tt select my-project   # Select by slug or ID directly",
		"Manage action board items",
		"Plan work from the Today cockpit",
		"tt task list --parked     # Show parked tasks only",
		"--parked bool    show parked tasks only",
	}

	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("CLI reference missing expected generated text: %q", needle)
		}
	}
}
