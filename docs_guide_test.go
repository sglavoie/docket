package main

import (
	"strings"
	"testing"
)

// The behavior guide under docs/guide/ holds the reference material the README
// used to carry inline. These tests read the guide as one corpus, because a
// contract belongs wherever it reads best and several of them span files that a
// reader would open separately. The exceptions are deliberate: the retired-key
// inventory and the four anchored headings are asserted per file, because their
// placement is itself the claim.

func TestGuideDocumentsOutlinerGrammar(t *testing.T) {
	content := readGuideCorpus(t)
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
		"`P` remains Select Project, which changes what you are looking at rather than where a task lives",
		"`&` opens the cursor task's Attachments panel, `;` its Comments panel — the two capture " +
			"sigils reaching, as keys, the panels that own add, reorder, unlink, and delete",
		"`y` title, `Y` description, `Ctrl+Y` both",
		"`o` opens the sort chord: `o@` due date, `o!` priority, `oc` created, `ou` updated, `ot` title, `ox` clear; repeat a field key to reverse it",
		"`\\` Named Lenses, `Ctrl+N` Notes, `T` theme, `/` search, `?` help, `u` undo, " +
			"`Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit",
		"`n`/`N` jump to the next/previous section or project header, wrapping at either end",
		"`s`, `p`, and `t` are the three bulk fold toggles — every visible section header, " +
			"every project group header, every task subtree — and each expands its whole set if " +
			"any part of it is folded and otherwise folds the set",
		"`Ctrl+R` reach the same workspace ledger they reach everywhere else",
		"Outliner, Select Project, Notes, and the Comments and Attachments panels all answer " +
			"both keys against that one ledger",
		"No surface offers the `u undo` a recovery toast advertises and then consumes the key in silence",
		"`u` and `Ctrl+R` reach the same workspace ledger here too",
		"both it and `u` stay literal characters while a comment is being typed",
		"They are the two structural keys that answer no sigil",
		"`@` opens one Set Due or Defer picker holding both fields",
		"On a terminal task the defer rows are disabled and say so rather than disappearing",
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

func TestGuideDocumentsExactTaskMenu(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"`Space` contains exactly: 1. Delete",
		"Comments and Attachments left this menu when they gained `;` and `&`",
		"Project Actions holds Edit project, then Archive project and Delete project under Danger",
		"Pinning left this menu when the header gained `*`",
		"`Enter` and `Ctrl+N` already reach both from that row",
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

func TestGuideDocumentsPeekAndBulkSemantics(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
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
		"A tag row is checked when every selected task carries it, `[-]` when only some do, and empty when none do",
		"applying adds every row you checked, removes every row you unchecked, and leaves every `[-]` row exactly as it is",
		"a mixed row can be left but not re-entered, and canceling the overlay is how you restore an untouched set",
		"The `CONTEXTS` rows read `[-]` the same way",
		"they write nothing unless you pick one",
		"picking `No context` clears the field across the whole selection the way `##0` does",
		"A context named `0` is refused",
	})
}

func TestGuideDocumentsFiltersAndSearch(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"`f!0`, `f!1`–`f!3`",
		"Unset never matches",
		// Search composition: the facets and the sort axis compose with a live
		// projection, F* still refuses, and Esc restores the pre-search pair.
		"the row facets and the sort axis compose with a live search",
		"`f*` applies as an AND gate before scoring exactly like a query sigil",
		"`o*` reorders within a relevance tier only",
		"The population facets (`F*`) still refuse",
		"mode switches, folds, and reordering still ask you to exit search first",
		"Esc restores the pre-search facets and sort wholesale",
		"`f$` | Pick workflow states with OR semantics",
		"`f#` | Pick tags with AND semantics; the task context is a separate axis on `fc`",
		"`fc` | Pick task contexts with OR semantics, since a task carries at most one",
		"`fa` | Pick completed at least 7, 30, or 90 local-calendar days ago",
		"`fr` | Toggle the `review` tag facet",
		"`f@` | Open the Filter by Due or Deferred picker: overdue, today, this week, no due date, or deferred-only",
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
	})
}

func TestGuideDocumentsNamedLenses(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"### Named lenses",
		"`\\` opens Named Lenses, the saved-projection list",
		"`1`–`9` | Apply the Nth saved lens shown, exactly as `Enter` on that row would",
		"Both action rows are unnumbered, and a narrowed list renumbers from one",
		"the mode, its own narrowing query, both filter families,\nand the sort axis",
		"an ordinary search cannot survive underneath a\nlens",
		"none of them records an undo entry and `u` never takes a lens back",
		"gains a `*`\nas soon as the live view stops matching what was saved",
		"switching straight from one lens to another\nkeeps the original restoration point",
		"While a lens is active `/` edits the lens's own narrowing query",
		"A lens owns project scope too, and saving is what decides whether it does",
		"refuses by name instead of opening\nsomewhere else",
		"Entering or leaving a project scope while a lens is live marks it\nmodified",
		"`Waiting`, `High Priority`, and `Stale`",
		"Definitions live in machine-local state and survive a restart, and so does the lens you " +
			"had open",
		"reapplies that lens's saved axes on top of it",
		"it lands on the Active Projects, All view a lens-free launch would have shown",
	})
}

func TestGuideDocumentsCaptureAndWorkflowContracts(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"!1-3 #tag ##context @date @@defer $destination *pin %project >parent <child ~task &link ;comment;",
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
		"`^type`, priorities `!4`–`!5`, `$ready`, and `$in_progress`",
		"The tilde no longer names recurrence, which stays retired",
		"`~task` records a soft dependency on another task and may be repeated",
		"The edit surfaces add `-~task`, which removes one reference, and `~0`, which clears them all",
		"`#tag` creates and sets a lowercase ASCII kebab tag on the fly",
		"`inbox`, `backlog`, `today`, `next`, `doing`, `waiting`, `paused`, `someday`, `done`, or `canceled`",
		"A new child defaults to Backlog",
		"Exact case-insensitive `bug`, `question`, `epic`, `story`, `feature`, and `idea`",
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

// TestGuideDocumentsProjectsNotesAndTags merges what were three tests over one
// README stretch: the Select Project grammar, the project-tag axes, and the
// workspace-wide Notes manager. They land in one guide file and read as one
// subject, so one gate is honest where three were an artifact of the old layout.
func TestGuideDocumentsProjectsNotesAndTags(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		// Select Project.
		"`P` opens Select Project on a live cursor",
		"carries its open task count beside its total and its tag chips, so a project whose tasks " +
			"are all finished reads `0 open · 2 tasks` instead of passing for one with work left",
		"`Enter` focuses it in the same Outliner, `a` creates one, `e` edits the one under the cursor, " +
			"`*` toggles its pin, `#` sets its tags, and `Ctrl+N` opens its Notes",
		"`Ctrl+T` cycles between every project, only those holding open tasks, and only those holding none open",
		"`/` opens project search and `Esc` leaves the query for the matches it found",
		"`v` selects the project under the cursor and `Ctrl+A` selects every project matching the current search",
		"`Space` offers Archive on an active project and Unarchive on an archived one",
		"the two lifecycle actions no direct key reaches",
		"`Esc` unwinds one rung at a time: the action sheet, then the query, then the selection, " +
			"then the surface itself",
		"`Space` on a project group header opens Project Actions",
		"Archiving is one-way from there — an Outliner header is only ever drawn for an active project — " +
			"so Unarchive stays here, where an archived project is visible",
		"Deleting a project is the one act with no second chance, and it asks first, naming the cascade " +
			"by name: the project's tasks, its notes, and its attachments all go",
		"The removal and the catalog and planning rewrites beside it are one transaction, so `u` brings " +
			"every byte back",
		"The system Unfiled project cannot be deleted, and an archived project still can be",
		// Project tags.
		"### Project tags",
		"A project carries any number of tags",
		"tag a project `parked` and it leaves the base Outliner entirely",
		"Today and Next entries are kept exactly as they were",
		"still accepts `%project` during capture and inline editing",
		"anchored with the `parked` tag",
		"The protected Unfiled project cannot be parked",
		"It preserves a project's tags and restoring returns them unchanged",
		// All Notes.
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
		"Note tags, task links, pins, bulk actions, and archived-project ownership remain excluded",
		"At 72 columns and wider, Notes shows the selected note beside the list",
		"its title, owner, and modification time above the note's rendered Markdown",
		"Narrower terminals render the list alone",
		"a note whose frontmatter failed to parse shows the reason as a warning",
		"`Ctrl+F` and `Ctrl+B` page that body with a row of overlap",
		"both presses work while you are still typing a search, and are offered only " +
			"where they would move something, never on an unparsed note's warning and " +
			"never when the pane is off screen",
		"A search that matched the body opens the pane around its first match with one " +
			"line of context above it, under the same case reading the result list used",
		"a title-only match, no query at all, and a match the rendered text cannot be " +
			"located in all open at the top",
		"re-anchors whenever the note, its bytes, the query, the scope, or the layout " +
			"changes, holds your manual paging while none of those move",
	})
	rejectDocumentationText(t, content, []string{
		"Global notes and, when a project is focused",
		"focused-project Notes",
		"search and moving are removed",
		"a `Ctrl+N` alias remain excluded",
		"`n` (or its `a` alias)",
		"### Project boundaries",
		"Boundary Manager",
		"Select Project with search focused",
		"`a`/`A` select or clear every",
	})
}

func TestGuideDocumentsStorageAndMigration(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"workspace.json",
		"planning.json",
		"schema-14 Markdown and generic `Task` headings",
		"Each task may finally carry an optional `depends` list of soft references",
		"Each task may carry an optional non-negative `defer_count`; zero is omitted",
		"Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the dark/light theme " +
			"choice, the saved named-lens definitions, and a pointer to the lens that was last open",
		"The active-lens pointer is a hint rather than a record: it is checked for shape alone when " +
			"the file loads",
		"pre-simplification-v5-<UTC>.tar.gz",
		"pre-consolidation-v6-<UTC>",
		"pre-scope-v7-<UTC>.tar.gz",
		"pre-tag-v8-<UTC>.tar.gz",
		"pre-state-clock-v9-<UTC>.tar.gz",
		"pre-defer-rename-v10-<UTC>.tar.gz",
		"pre-defer-instant-v11-<UTC>.tar.gz",
		"pre-context-v12-<UTC>.tar.gz",
		"pre-defer-count-v13-<UTC>.tar.gz",
		"pre-due-time-v14-<UTC>.tar.gz",
		"pre-dependencies-v15-<UTC>.tar.gz",
		"Schema 13 upgrades task Markdown to schema 12",
		"Schema 14 upgrades task Markdown to schema 13 by adding the optional `due_time`",
		"Schema 15 upgrades task Markdown to schema 14 by adding the optional `depends`",
		"durable `state_changed` clock, backfilled from `updated`",
		"There is no down-migration",
		"schema-4/5 workspaces through the whole chain",
		"folds the retired per-project grouping fields",
		"existing `unfiled` project or directory also aborts before snapshot publication",
		"Invalid or future-schema input aborts without changing workspace data",
		"`u` and `Ctrl+R` use one workspace-wide ledger capped at 50 committed transactions",
	})
	rejectDocumentationText(t, content, []string{
		"Configuration is stored at `~/.tt.json`",
	})
}

// TestGuideDocumentsRetiredKeyInventory is per-file on purpose. The inventory's
// own prose is the contract — every surface's retirements are stated in one
// place rather than beside the surface that dropped them — so a guide that
// scattered these sentences back across its files would still satisfy the
// corpus while breaking the claim.
func TestGuideDocumentsRetiredKeyInventory(t *testing.T) {
	content := readRepositoryDoc(t, "docs/guide/retired-keys.md")
	requireDocumentationText(t, content, []string{
		"every surface's retirements are inventoried here rather than left " +
			"beside the surface that dropped them",
		"the old standalone `b`, `c`, `m`, and `w` actions, the comma state prefix, and " +
			"`:`, `C`, `R`, and `Ctrl+Esc`",
		"Two names came back with new acts rather than as aliases: `t` is the task-subtree fold and " +
			"`P` is Select Project",
		"`d` is unbound: Done stays `$d` alone, `@` answers both dates, and root `z` is the scroll chord prefix",
		"`$` joins them as a chord prefix rather than a key of its own",
		"`$q is not a workflow action`; pressing a key that was never bound does nothing and says nothing",
		"`n` no longer creates a project, `A` no longer clears a selection, and bare `g` is the go-to prefix " +
			"rather than cursor-top, so all three are silent there",
		"All Notes retired `s`, `r`, `m`, `n`, and `x` for `/`, `e`, `%`, `a`, and the `Space` sheet, " +
			"and freed `v` rather than reassigning it, because Notes has no selection for it to toggle; " +
			"all six are silent there",
		"Attachments retired `d` and `x` for its `Space` sheet, `o` for `Enter`, `e` for `E`, and " +
			"`O`, `Ctrl+E`, and `Ctrl+O` for that same sheet",
		"a panel with nothing to rename may not spend it on the editor",
		"once unlink moved to that sheet the key was free, so it now means here " +
			"what it means everywhere else",
		"Comments retired the hidden `n` add alias for `a`, and `d` and `x` for its `Space` sheet",
		"a comment may not stay one press from deletion while a task cannot be deleted that way",
		"Review has no key of its own",
	})
}

// TestGuideFilesCarryRequiredHeadings pins the four headings whose file identity
// something else reads: the checklist anchor
// internal/tui/checklist_row_operations_docs_test.go matches exactly, and the
// three subject anchors a reader follows a link to.
func TestGuideFilesCarryRequiredHeadings(t *testing.T) {
	for _, anchor := range []struct {
		file    string
		heading string
	}{
		{file: "docs/guide/outliner.md", heading: "### Inline checklist rows"},
		{file: "docs/guide/search-and-lenses.md", heading: "### Named lenses"},
		{file: "docs/guide/projects-and-notes.md", heading: "### Project tags"},
		{file: "docs/guide/projects-and-notes.md", heading: "### All Notes"},
	} {
		content := readRepositoryDoc(t, anchor.file)
		found := false
		for _, line := range strings.Split(content, "\n") {
			if strings.TrimSpace(line) == anchor.heading {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s no longer carries the heading %q", anchor.file, anchor.heading)
		}
	}
}
