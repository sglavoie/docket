package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readREADMEForContractTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}

	path := filepath.Join(filepath.Dir(file), "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestReadREADMEForContractTestUsesStableBaseDir(t *testing.T) {
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

	content := readREADMEForContractTest(t)
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
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"The Outliner is the only primary task presentation",
		"`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due",
		"capture, triage, plan, execute, and complete",
	})
	// The final section catalog is intentionally checked as one ordered string.
	requireDocumentationText(t, content, []string{
		"Pinned, Today, Doing, Review, Stale, Inbox, Next, Paused, Backlog, Waiting, Someday",
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
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"`a` capture, `A` child, `e` inline task edit, `E` description",
		"`$` opens the workflow chord",
		"`$i` Inbox, `$b` Backlog, `$n` toggle Next, `$t` toggle Today, `$g` Doing",
		"`$w` Waiting, `$p` Paused, `$m` Someday, `$d` Done, `$c` Canceled",
		"`!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date and defer; `#` tags and context; `*` pin",
		"`[` outdent, `]` indent, `>` Set Parent, `<` Adopt Children, `%` Move to Project",
		"`>` opens Set Parent, the picker form of the `>parent` edit sigil",
		"a leading `No parent (project root)` row that does what `>0` does",
		"is still listed, disabled and carrying the reason",
		"Set Parent never crosses a project boundary",
		"`<` opens Adopt Children, the picker form of the `<child` edit sigil",
		"because it has no clearing form, the list is strictly additive",
		"Tasks the cursor task already parents are absent from the list, so unchecking can never mean detaching",
		"`%` opens Move to Project, the picker form of the `%project` edit sigil",
		"the destination mints its own `#N` numbers for every task it receives",
		"skips the one the task already lives in, since a move to it is not a move",
		"drawn from every non-archived project rather than the ones the Outliner shows",
		"`p` remains Select Project, which changes what you are looking at rather than where a task lives",
		"`y` title, `Y` description, `Ctrl+Y` both",
		"`o` opens the sort chord: `o@` due date, `o!` priority, `oc` created, `ou` updated, `ot` title, `ox` clear; repeat a field key to reverse it",
		"`Ctrl+N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit",
		"`n`/`N` jump to the next/previous section or project header, wrapping at either end",
		"the old standalone `b`, `c`, `m`, `t`, `w`, and `P` actions, the comma state prefix, and " +
			"`:`, `C`, `R`, and `Ctrl+Esc`",
		"`d` is unbound: Done stays `$d` alone, `@` answers both dates, and root `z` is the scroll chord prefix",
		"`$` joins them as a chord prefix rather than a key of its own",
		"`$q is not a workflow action`; pressing a key that was never bound does nothing and says nothing",
		"Attachments retired `d` and `x` for its `Space` sheet, `o` for `Enter`, `e` for `E`, and " +
			"`O`, `Ctrl+E`, and `Ctrl+O` for that same sheet",
		"a panel with nothing to rename may not spend it on the editor",
		"once unlink moved to that sheet the key was free, so it now means here " +
			"what it means everywhere else",
		"`Ctrl+R` reach the same workspace ledger they reach everywhere else",
		"Outliner, Select Project, Notes, and the Attachments panel all answer both keys " +
			"against that one ledger",
		"the Comments panel is the one surface that answers neither",
		"Comments retired the hidden `n` add alias for `a`, and `d` and `x` for its `Space` sheet",
		"a comment may not stay one press from deletion while a task cannot be deleted that way",
		"They are the two structural keys that answer no sigil",
		"`@` opens one Set Date picker holding both fields",
		"On a terminal task the defer rows are disabled and say so rather than disappearing",
		"Review has no key of its own",
		"`a` creates and selects a new tag",
		"the `CONTEXTS` section is a radio group answering `#`",
		"the leading `No context` row clears the field the way `##0` does",
	})
	rejectDocumentationText(t, content, []string{
		"`z` snooze",
		"`fz`",
		"Snoozed-only",
		"`<` outdent, `>` indent",
	})
}

func TestREADMEDocumentsExactTaskMenu(t *testing.T) {
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"1. Comments 2. Attachments 3. Delete",
		"The list filters as soon as printable text is typed",
		"Delete always asks for confirmation",
		"The checklist is absent because it is edited on its own rows",
		"1. Reveal original 2. Open original 3. Open attachments folder 4. Unlink 5. Delete",
		"removing an attachment is now a deliberate menu row rather than a single press",
		"Its own `Space` sheet holds Delete alone, which confirms and has no direct key",
		"a comment leaves the list only through a deliberate menu row",
		"Append `%project` to move its subtree, `>parent` to reparent it, `>0` to move it to the project root, or `<child` once per task you want to adopt",
	})
}

func TestREADMEDocumentsFiltersPeekAndBulkSemantics(t *testing.T) {
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"`f!0`, `f!1`–`f!3`",
		"Unset never matches",
		"`f$` | Pick workflow states with OR semantics",
		"`f#` | Pick tags and task contexts with AND semantics; each selected value matches either field",
		"`fr` | Toggle the `review` tag facet",
		"`f@` | Open the due-date facet picker: overdue, today, this week, no due date, or deferred-only",
		"`fh` | Pick one hierarchy position: parent, child, root, or leaf; parent and leaf read open children",
		"`fs` | Toggle open, undeferred work untouched for at least 90 local-calendar days",
		"`fx` | Clear every facet",
		"`F#` | Pick project tags with AND semantics",
		"`Fp` | Add parked projects to the population",
		"`FP` | Isolate the parked projects",
		"`Fa` | Add archived projects to the population",
		"`FA` | Isolate the archived projects",
		"`Fx` | Clear every project filter",
		"Each population axis has three states — excluded, included, isolated",
		"`Fp`, `FP`, and selecting `parked` inside `F#` are the same switch",
		"`Space` over a tag row offers Rename everywhere, Remove everywhere, Park all members, and Unpark all members",
		"a second key the chord does not offer says so instead of doing nothing",
		"The Outliner uses the full terminal width at every supported size",
		"The bold `[context]` segment is reserved like the relation marker",
		"a row whose open children are all placed somewhere else ends with `›N`",
		"`Enter` focuses the cursor task's descendants",
		"`Shift+Enter` widens in the other direction",
		"`Ctrl+Enter` toggles a session-only Peek directly beneath the cursor task",
		"Entering multi-select closes Peek",
		"`v` toggles the cursor task in or out of the selection",
		"`V` starts a fresh inclusive range at the cursor",
		"`Ctrl+A` replaces the selection with all visible task rows",
		"Successful Delete clears selection; other successes, cancellations, and failures preserve it",
	})
}

func TestREADMEDocumentsFinalCaptureAndWorkflowContracts(t *testing.T) {
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"!1-3 #tag ##context @date @@defer $destination *pin %project >parent <child",
		"`@@defer` hides the task until an instant rather than a day",
		"`*pin` pins the task, exactly as the `*` key does on an existing row",
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
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"`Ctrl+N` opens All Notes",
		"On a project header row it opens that project's notes instead",
		"Global plus every unarchived project, including the protected Unfiled project and every parked project",
		"`p` opens a searchable All, Global, or single-project scope picker",
		"`/` applies a persistent title/body search",
		"`%` moves the selected note to another such owner as one atomic workspace transaction",
		"`y` and `Y` copy that note's title and its full file path, and because a global note and " +
			"a project note live in different directories the copied path says which collection it " +
			"came from without a word of context",
		"`Space` opens the note action sheet, which holds Delete alone and confirms before deleting; " +
			"deletion has no direct key",
		"Notes has no multi-select, so that sheet always acts on the one note under the cursor",
		"Notes support open/edit, create, rename, move, copy, delete, undo, and redo",
		"Note tags, task links, pins, bulk actions, previews, and archived-project ownership remain excluded",
		"All Notes retired `s`, `r`, `m`, `n`, and `x` for `/`, `e`, `%`, `a`, and the `Space` sheet, " +
			"and freed `v` rather than reassigning it, because Notes has no selection for it to toggle; " +
			"all six are silent there",
	})
	rejectDocumentationText(t, content, []string{
		"Global notes and, when a project is focused",
		"focused-project Notes",
		"search and moving are removed",
		"a `Ctrl+N` alias remain excluded",
		"`n` (or its `a` alias)",
	})
}

// TestREADMEDocumentsSelectProjectGrammar holds the Select Project vocabulary to
// the same standard as the Outliner's: every press is named, and the three
// spellings the browse-first grammar retired are named as silent rather than
// left to be rediscovered.
func TestREADMEDocumentsSelectProjectGrammar(t *testing.T) {
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"`p` opens Select Project on a live cursor",
		"carries its open task count beside its total and its tag chips, so a project whose tasks " +
			"are all finished reads `0 open · 2 tasks` instead of passing for one with work left",
		"`Enter` focuses it in the same Outliner, `a` creates one, `e` edits the one under the cursor, " +
			"`*` toggles its pin, `#` sets its tags, and `Ctrl+N` opens its Notes",
		"`Ctrl+T` cycles between every project, only those holding tasks, and only those holding none",
		"`/` opens project search and `Esc` leaves the query for the matches it found",
		"`v` selects the project under the cursor and `Ctrl+A` selects every project matching the current search",
		"`Space` offers Archive on an active project and Unarchive on an archived one",
		"the two lifecycle actions no direct key reaches",
		"`Esc` unwinds one rung at a time: the action sheet, then the query, then the selection, " +
			"then the surface itself",
		"`n` no longer creates a project, `A` no longer clears a selection, and bare `g` is the go-to prefix " +
			"rather than cursor-top, so all three are silent there",
	})
	rejectDocumentationText(t, content, []string{
		"Select Project with search focused",
		"`a`/`A` select or clear every",
	})
}

func TestREADMEDocumentsProjectTags(t *testing.T) {
	content := readREADMEForContractTest(t)
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
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"workspace.json",
		"planning.json",
		"schema-12 Markdown and generic `Task` headings",
		"Each task may carry an optional non-negative `defer_count`; zero is omitted",
		"Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the dark/light theme choice",
		"pre-simplification-v5-<UTC>.tar.gz",
		"pre-consolidation-v6-<UTC>",
		"pre-scope-v7-<UTC>.tar.gz",
		"pre-tag-v8-<UTC>.tar.gz",
		"pre-state-clock-v9-<UTC>.tar.gz",
		"pre-defer-rename-v10-<UTC>.tar.gz",
		"pre-defer-instant-v11-<UTC>.tar.gz",
		"pre-context-v12-<UTC>.tar.gz",
		"pre-defer-count-v13-<UTC>.tar.gz",
		"Schema 13 upgrades task Markdown to schema 12",
		"durable `state_changed` clock, backfilled from `updated`",
		"There is no down-migration",
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
