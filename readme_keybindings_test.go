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
	if !strings.Contains(content, "# tt - Terminal Tasks") {
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
		"`tt` opens on the Outliner in global All by default",
		"press `Ctrl+Esc` to return to that root from anywhere",
		"capture, triage, plan, execute, and complete",
		"`Tab` cycles All → Inbox → Next → Backlog",
		"All, Inbox, Next, Backlog, Due, Priority, Blocked, Parked,",
		"Daily Review, and Activity",
		"Today, In Progress, Inbox, Next, Backlog, Review,",
		"Waiting, Paused, and Someday",
	})
	rejectDocumentationText(t, content, []string{
		"Next → Backlog → Active → All",
		"workflow perspective",
		"Command Palette",
		"command palette",
		"Inbox Processor",
		"prefix menu",
		"`Ctrl+X`",
		"`Ctrl+G`",
		"`q` to toggle",
		"`w` to toggle",
	})
}

func TestREADMEDocumentsStableKeysAndActionSheets(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"`a` capture, `e` edit, `s` start, `d` done, `b` Backlog, `t` Today, `n` Next, `Space` actions",
		"`/` visible search, `:` Finder, `m` modes, `c` Scope Hub, `v` multi-select",
		"`N` Notes, `R` Weekly Review, `T` Tags, `!` Notifications",
		"`u` undo, `Ctrl+R` redo, `Esc` back, `Ctrl+Esc` reset, `?` help, `Ctrl+C` quit",
		"In multi-select, `Space` toggles the current task and `Enter` opens bulk actions",
		"Delete is available only from an action sheet and always requires confirmation",
	})
}

func TestREADMEDocumentsScopeFinderAndPromotedTools(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"Press `c` to open the Scope Hub",
		"Scope is session-owned and never persisted across runs",
		"Notes opens project notes in project focus and global scratch notes in a base scope",
		"Weekly Review and Tags receive the active scope's project set",
		"Notifications remains global",
		"Press `:` for Finder",
		"Empty Finder input shows a short, bounded list of recent destinations",
		"Pinned, recent, and saved-filter entries are discovery results",
	})
	rejectDocumentationText(t, strings.ToLower(content), []string{
		"`f p`",
		"project filter popup",
		"saved-filter tabs",
		"pinned tag tabs",
	})
}

func TestREADMEDocumentsCanonicalStateAndPlanning(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"inbox  ready  next  in_progress  waiting",
		"paused  review  someday  done  canceled",
		"Dependency blocking is derived",
		"`b` sets Ready and removes Today and Next membership",
		"`n` sets Next and removes Today",
		"`t` toggles Today",
		"Today and Next are mutually exclusive",
		"Focus Queue is only an optional ordering index",
		"Lifecycle input uses `$state` or `state:<value>`",
	})
	rejectDocumentationText(t, content, []string{
		"Active section",
		"Standby",
		"task status",
		"--section",
		"--status",
		"/inbox",
		"/next",
	})
}

func TestREADMECLIAndStorageContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"tt task add \"Deploy\" --state next",
		"tt task list --state ready",
		"tt task state in_progress 12",
		"tt task reopen 12 # Return terminal work to Inbox",
		"tt task archive list",
		"completed.md",
		".completed.index.json",
		"tt undo # Restore previous task state",
		"local `~/.tt/` workspace by default",
		"iCloud is used only after explicit selection",
		"not converted automatically",
		"`--force` skips only the confirmation prompt",
		"docs/architecture.md",
		"just tui-scenarios",
	})
	rejectDocumentationText(t, content, []string{
		"Configuration is stored at `~/.tt.json`",
		"tt task move-to <id> <target-id>",
		"tt task depends add <id> <blocker-id>",
	})
}

func TestCLIReferenceContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "cmd/cli-reference.md")

	rejectDocumentationText(t, content, []string{
		"config file path (default: ~/.tt.json)",
		"config file path (default: ~/.tt/.tt.json)",
		"'editor' field in ~/.tt.json",
		"'editor' field in ~/.tt/.tt.json",
		"Usage: tt project archive [name|id] [flags]",
		"Usage: tt project delete [name|id] [flags]",
		"Tasks are added to the daily plan by pressing 'w'",
		"Press 'q' to toggle",
		"Press 'w' on a task",
		"built-in Today section",
		"Sections are preserved if they are built-in GTD sections",
		"changes to status, priority, section",
		"Set one or more tasks back to todo status",
		"Tags group tasks across the built-in GTD sections",
		"cross-project tag tabs",
	})

	requireDocumentationText(t, content, []string{
		"deprecated: isolate a workspace at this config file's directory",
		"workspace data directory (env TT_DATA_DIR; default: ~/.tt",
		"## tt workspace",
		"Usage: tt workspace move <destination> [flags]",
		"'editor' field in the tt config file",
		"## tt task archive list",
		"Usage: tt task archive show <id-or-prefix> [flags]",
		"completed.md and completed.N.md",
		"Usage: tt project archive <slug-or-id> [flags]",
		"Projects can be specified by slug or ID.",
		"Show the Outliner's Next shortlist (StateNext tasks)",
		"Press 'n' to toggle",
		"selected task's Next membership",
		"synthetic Today group at the top of the Outliner's",
		"Press 't' on a task",
		"Canonical workflow state is preserved",
		"records changes to state, priority, tags",
		"Reopen one or more terminal tasks in Inbox for triage",
		"tt task list --parked     # Show parked tasks only",
		"--parked bool    show parked tasks only",
	})
}
