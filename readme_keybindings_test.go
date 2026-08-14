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
		"`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due",
		"capture, triage, plan, execute, and complete",
	})
	// The final section catalog is intentionally checked as one ordered string.
	requireDocumentationText(t, content, []string{
		"Pinned, Today, Doing, Review, Inbox, Next, Paused, Backlog, Waiting, Someday",
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
		"`a` capture, `A` child, `e` inline task edit, `E` description",
		"`w` opens the workflow chord",
		"`wi` Inbox, `wb` Backlog, `wn` toggle Next, `wt` toggle Today, `wg` Doing",
		"`wr` toggle Review",
		"`ww` Waiting, `wp` Paused, `wm` Someday, `wd` Done, `wc` Canceled",
		"`!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date; `#` tags; `d` defer; `P` pin",
		"`y` title, `Y` description, `Ctrl+Y` both",
		"`o` opens the sort chord: `o@` due date, `op` priority, `oc` created, `ou` updated, `ot` title, `ox` clear; repeat a field key to reverse it",
		"`Ctrl+N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit",
		"`n`/`N` jump to the next/previous section or project header, wrapping at either end",
		"The old standalone `b` and `t` actions and comma state prefix have no compatibility action",
		"`d` is bound again, but to Set Defer and never to Done: Done stays `wd` alone, and root `z` is now unbound",
		"`a` creates and selects a new tag",
	})
	rejectDocumentationText(t, content, []string{
		"`z` snooze",
		"`fz`",
		"Snoozed-only",
	})
}

func TestREADMEDocumentsExactTaskMenu(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"1. Comments 2. Attachments 3. Delete",
		"The list filters as soon as printable text is typed",
		"Delete always asks for confirmation",
		"The checklist is absent because it is edited on its own rows",
		"Append `%project` to move its subtree, `>parent` to reparent it, `>0` to move it to the project root, or `<child` once per task you want to adopt",
	})
}

func TestREADMEDocumentsFiltersPeekAndBulkSemantics(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"`fp0`, `fp1`–`fp3`",
		"Unset never matches",
		"`fs` | Pick workflow states with OR semantics",
		"`ft` | Pick tags with AND semantics",
		"`fr` | Toggle the `review` tag facet",
		"`f@` | Open the due-date facet picker: overdue, today, this week, or no due date",
		"`fd` | Deferred-only",
		"`fx` | Clear every facet",
		"`Ft` | Pick project tags with AND semantics",
		"`Fp` | Add parked projects to the population",
		"`FP` | Isolate the parked projects",
		"`Fa` | Add archived projects to the population",
		"`FA` | Isolate the archived projects",
		"`Fx` | Clear every project filter",
		"Each population axis has three states — excluded, included, isolated",
		"`Fp`, `FP`, and selecting `parked` inside `Ft` are the same switch",
		"`Space` over a tag row offers Rename everywhere, Remove everywhere, Park all members, and Unpark all members",
		"a second key the chord does not offer says so instead of doing nothing",
		"The Outliner uses the full terminal width at every supported size",
		"`Enter` focuses the cursor task's descendants",
		"`Shift+Enter` toggles a session-only Peek directly beneath the cursor task",
		"Entering multi-select closes Peek",
		"`v` toggles the cursor task in or out of the selection",
		"`V` starts a fresh inclusive range at the cursor",
		"`Ctrl+A` replaces the selection with all visible task rows",
		"Successful Delete clears selection; other successes, cancellations, and failures preserve it",
	})
}

func TestREADMEDocumentsFinalCaptureAndWorkflowContracts(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"!1-3 #tag @date $destination *pin %project >parent <child",
		"`*pin` pins the task, exactly as the `P` key does on an existing row",
		"The edit surface is also the only one that accepts `*unpin`",
		"a pinned task arrives carrying `*pin` and deleting that token unpins it",
		"`<child` adopts a task that already exists, and may be repeated",
		"both resolve in the effective project",
		"Adoption is additive",
		"There is no clear form: `<0` is an ordinary reference that resolves to nothing",
		"Aliases that name the same task collapse to one adoption, keeping the first occurrence",
		"two lines cannot adopt the same task",
		"`^type`, `~recurrence`, priorities `!4`–`!5`, `$ready`, and `$in_progress`",
		"`#tag` creates and sets a lowercase ASCII kebab tag on the fly",
		"`inbox`, `backlog`, `today`, `next`, `doing`, `waiting`, `paused`, `someday`, `done`, or `canceled`",
		"A new child defaults to Backlog",
		"Exact case-insensitive `bug`, `epic`, `story`, `feature`, and `idea` tags",
		"add no prefix or marker icon",
		"Today and Next are mutually exclusive",
		"Waiting is an explicit lifecycle state",
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
		"`Ctrl+N` opens All Notes",
		"On a project header row it opens that project's notes instead",
		"Global plus every unarchived project, including the protected Unfiled project and every parked project",
		"`v` opens a searchable All, Global, or single-project scope picker",
		"`s` applies a persistent title/body search",
		"`m` moves the selected note to another such owner as one atomic workspace transaction",
		"Notes support open/edit, create, rename, move, delete, undo, and redo",
		"Note tags, task links, pins, bulk actions, previews, and archived-project ownership remain excluded",
	})
	rejectDocumentationText(t, content, []string{
		"Global notes and, when a project is focused",
		"focused-project Notes",
		"search and moving are removed",
		"a `Ctrl+N` alias remain excluded",
	})
}

func TestREADMEDocumentsProjectTags(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"### Project tags",
		"A project carries any number of tags",
		"tag a project `parked` and it leaves the base Outliner entirely",
		"Today and Next entries are kept exactly as they were",
		"still accepts `%project` during capture and inline editing",
		"anchored with the `parked` tag",
		"The protected Unfiled project cannot be parked",
		"It preserves a project's tags and restoring returns them unchanged",
	})
	rejectDocumentationText(t, content, []string{
		"### Project boundaries",
		"Boundary Manager",
	})
}

func TestREADMEWorkspaceAndMigrationContractStaysCurrent(t *testing.T) {
	content := readFileForContractTest(t, "README.md")
	requireDocumentationText(t, content, []string{
		"workspace.json",
		"planning.json",
		"schema-8 Markdown and generic `Task` headings",
		"Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the dark/light theme choice",
		"pre-simplification-v5-<UTC>.tar.gz",
		"pre-consolidation-v6-<UTC>",
		"pre-scope-v7-<UTC>.tar.gz",
		"pre-tag-v8-<UTC>.tar.gz",
		"pre-state-clock-v9-<UTC>.tar.gz",
		"durable `state_changed` clock, backfilled from `updated`",
		"schema-4/5 workspaces through the whole chain",
		"folds the retired per-project grouping fields",
		"existing `unfiled` project or directory also aborts before snapshot publication",
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
		"Startup validates schema-6 data before loading",
		"immutable snapshot-first boundaries",
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
