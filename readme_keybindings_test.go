package main

import (
	"os"
	"path/filepath"
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
	if !strings.Contains(content, "# tt — Terminal Tasks") {
		t.Fatal("expected README content to load regardless of cwd")
	}
}

func requireDocumentationText(t *testing.T, content string, required []string) {
	t.Helper()
	content = strings.Join(strings.Fields(content), " ")
	for _, needle := range required {
		if !strings.Contains(content, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("documentation missing %q", needle)
		}
	}
}

func rejectDocumentationText(t *testing.T, content string, forbidden []string) {
	t.Helper()
	content = strings.Join(strings.Fields(content), " ")
	for _, needle := range forbidden {
		if strings.Contains(content, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("documentation contains retired text %q", needle)
		}
	}
}

func TestREADMEDocumentsFinalDailyLoop(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"The Outliner is the only primary task presentation",
		"`Tab` cycles exactly All → Inbox → Today → Next → Backlog → Due",
		"capture, triage, plan, execute, and complete",
	})
	// The final section catalog is intentionally checked as one ordered string.
	requireDocumentationText(t, content, []string{
		"Pinned, Today, Doing, Inbox, Next, Paused, Backlog, Waiting, Someday",
	})
	rejectDocumentationText(t, content, []string{
		"Next → Backlog → Active → All",
		"Command Palette",
		"Inbox Processor",
		"Weekly Review",
		"`Ctrl+Esc` reset",
		"Press `:` for Finder",
	})
}

func TestREADMEDocumentsOneOutlinerGrammar(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"`a` capture, `A` child, `e` title, `E` description",
		"`s` opens the state/planning chord",
		"`si` Inbox, `sb` Backlog, `sn` toggle Next, `st` toggle Today, `sg` Doing",
		"`sw` Waiting, `sp` Paused, `sm` Someday, `sd` Done, `sc` Canceled",
		"`!0` clears priority; `!1`–`!5` set it; `@` due date; `#` tags; `z` snooze; `P` pin",
		"`y` title, `Y` description, `Ctrl+Y` both",
		"`c` Scope Hub, `C` Select Project, `N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit",
		"The old standalone `b`, `d`, `n`, and `t` actions and comma state prefix have no compatibility action",
		"`a` creates and selects a new tag",
	})
}

func TestREADMEDocumentsExactTaskMenu(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"1. Move Project 2. Set Parent 3. Checklist 4. Comments 5. Attachments 6. Delete",
		"The list filters as soon as printable text is typed",
		"Delete always asks for confirmation",
	})
}

func TestREADMEDocumentsFiltersPeekAndBulkSemantics(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"`fp0`, `fp1`–`fp5`",
		"`fs` | Pick workflow states with OR semantics",
		"`ft` | Pick tags with AND semantics",
		"`fd` | Open the due-date facet picker: overdue, today, this week, or no due date",
		"`fb` | Blocked-only",
		"`fz` | Snoozed-only",
		"`fx` | Clear every facet",
		"The Outliner uses the full terminal width at every supported size",
		"`Enter` toggles a session-only Peek directly beneath the cursor task",
		"Entering multi-select closes Peek",
		"In multi-select, `Space` offers Delete only",
	})
}

func TestREADMEDocumentsFinalCaptureAndWorkflowContracts(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"!1-3 #tag @date $destination %project >parent",
		"`^type`, `~recurrence`, priorities `!4`–`!5`, `$ready`, and `$in_progress`",
		"`#tag` creates and sets any valid tag on the fly",
		"`inbox`, `backlog`, `today`, `next`, `doing`, `waiting`, `paused`, `someday`, `done`, or `canceled`",
		"A new child defaults to Backlog",
		"Today and Next are mutually exclusive",
		"Unresolved dependencies block Done",
		"hierarchy depth is capped at six",
	})
	rejectDocumentationText(t, content, []string{
		"^type !1-5 #tag @date ~interval $state",
		"paused review someday",
		"completed.md",
		".completed.index.json",
	})
}

func TestREADMEDocumentsWorkspaceWideNotesManager(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"`N` opens All Notes",
		"Global plus every project returned by the active-project catalog",
		"`v` opens a searchable All, Global, or active-project scope picker",
		"`s` applies a persistent title/body search",
		"`m` moves the selected note to another such owner as one atomic workspace transaction",
		"Notes support open/edit, create, rename, move, delete, undo, and redo",
		"Note tags, task links, pins, bulk actions, previews, archived-project ownership, and a `Ctrl+N` alias remain excluded",
	})
	rejectDocumentationText(t, content, []string{
		"Global notes and, when a project is focused",
		"focused-project Notes",
		"search and moving are removed",
	})
}

func TestREADMEWorkspaceAndMigrationContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"workspace.json",
		"planning.json",
		"schema-5 Markdown and generic `Task` headings",
		"machine-local `~/.tt/runtime/<workspace-id>/machine.json`",
		"pre-simplification-v5-<UTC>.tar.gz",
		"Invalid or future-schema input aborts without changing workspace data",
		"`u` and `Ctrl+R` use one workspace-wide ledger capped at 50 committed transactions",
		"./scripts/run_tui_scenarios.sh",
		"docs/architecture.md",
	})
	rejectDocumentationText(t, content, []string{
		"Configuration is stored at `~/.tt.json`",
		"## Settings",
		"## Finder",
	})
}

func TestCLIReferenceContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "cmd/cli-reference.md")

	rejectDocumentationText(t, content, []string{
		"## tt add",
		"## tt attach",
		"## tt context",
		"## tt dash",
		"## tt focus",
		"## tt note",
		"## tt redo",
		"## tt sandbox",
		"## tt search",
		"## tt select",
		"## tt task",
		"## tt theme",
		"## tt tui-test",
		"## tt undo",
		"## tt project archive",
		"## tt project delete",
		"## tt project edit",
		"## tt project list",
		"## tt project new",
		"## tt project restore",
		"--config",
		"--verbose",
	})

	requireDocumentationText(t, content, []string{
		"workspace data directory (env TT_DATA_DIR; default: ~/.tt",
		"## tt completion",
		"## tt conflict",
		"## tt conflict list",
		"## tt conflict resolve",
		"## tt conflict show",
		"## tt doctor",
		"## tt project",
		"## tt project export",
		"## tt workspace",
		"## tt workspace location",
		"Usage: tt workspace move <destination> [flags]",
		"--no-color bool",
	})
}
