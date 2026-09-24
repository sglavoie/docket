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
//
// What is asserted is narrower than what the README asserted about itself. A
// phrase stays when it pins a key, chord, sigil, or command literal; a heading
// anchor or an ordered catalog; a number or threshold; a path or filename; or
// vocabulary that must not return. A phrase pinning only the wording of an
// explanatory sentence whose behavior a literal above already pins is gone. The
// consequence is stated rather than discovered: guide prose may now be reworded
// without a test noticing, and only a product/document disagreement over a
// literal, a catalog, or a retired name still fails.

func TestGuideDocumentsOutlinerGrammar(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"`a` capture, `A` child, `c` add a checklist item to the cursor's task, " +
			"`S` turn a checklist item into a child task or a child task back into a checklist item in its current parent, " +
			"`D` repeat the cursor task as a new draft, `e` edits whatever the cursor names",
		"`w` opens the workflow chord over the ten destinations the `$destination` capture sigil names",
		// Session 5 divided that chord into Plan and Status, so the catalog is
		// asserted as the two runs it is drawn as rather than as one flat list.
		"Plan: `wn` toggle Next, `wt` toggle Today",
		"Status: `wi` Inbox, `wb` Backlog, `wg` Doing, `ww` Waiting, `wp` Paused, " +
			"`wm` Someday, `wd` Done, `wc` Canceled",
		"planning a task for Today leaves it in Backlog and Peek says `plan today · status backlog`",
		"the keyboard spells a sigil differently: `$` stays what you type, `w` is what you press",
		"`!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date, defer, and check-in schedule; " +
			"`-` and `+` step the deadline one day earlier and one day later; " +
			"`#` tags and context; `*` pin",
		"earlier dates, yesterday, no deadline at all, today, tomorrow, later dates",
		"`+` off yesterday, `-` off today",
		"`[` outdent, `]` indent, `>` Set Parent, `<` Adopt Children, `%` Move to Project, " +
			"`~` Set Dependencies",
		"a leading `No parent (project root)` row that does what `>0` does",
		"the destination mints its own `#N` numbers for every task it receives",
		"`&` opens the cursor task's Attachments panel, and `;` opens the Comments panel of " +
			"whatever the cursor names — the task, or the project a group header names — the two " +
			"capture sigils reaching, as keys, the panels that own add and delete, with reorder " +
			"and unlink in Attachments alone",
		"`C` appends one comment to the same two things without opening that panel, in a " +
			"multiline popup where `Enter` starts a new line and `Ctrl+S` saves",
		"`;` on a project group header opens that same panel over the project's own comments",
		"a comment you edit or delete is the one you were looking at rather than the row it sat on",
		"`y` title, `Y` description, `Ctrl+Y` both",
		"`o` opens the sort chord: `o@` due date, `o!` priority, `oc` created, `ou` updated, `ot` title, `or` switch the held query between the flat ranked list slash opens and a narrowing of the mode, `ox` clear; repeat a field key to reverse it",
		"`\\` Named Lenses, `Ctrl+N` Notes, `T` theme, `M` menu bar, `/` search, `?` help, `u` undo, " +
			"`Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit",
		// Session 7 moved n/N off the header row onto the block: a group whose
		// heading is not drawn still starts one, so the guide names what is stepped
		// between rather than the row kind that used to declare it.
		"`n`/`N` jump to the next/previous block — a section, a project group, or a " +
			"folded group's summary — wrapping at either end",
		"a task with no checklist, no children, and no parent left to step to folds its " +
			"own project group",
		"`l` on the summary that replaces it puts the cursor back on the task the fold " +
			"covered, or on the group's first task when there is none",
		"`s`, `p`, and `t` are the three bulk fold toggles — every visible section header, " +
			"every project group header, every task subtree — and each expands its whole set if " +
			"any part of it is folded and otherwise folds the set",
		"Outliner, Select Project, Notes, and the Comments and Attachments panels all answer " +
			"both keys against that one ledger",
		"No surface offers the `u undo` a recovery toast advertises and then consumes the key in silence",
		"both it and `u` stay literal characters while a comment is being typed",
		"`@` opens one scheduling picker holding three sections",
		"CHECK-IN contains one Set check-in",
		"`a` creates and selects a new tag",
		"the `CONTEXTS` section is a radio group answering `#`",
		"the leading `No context` row clears the field the way `##0` does",
		"`~` opens Set Dependencies, the picker form of the `~task` edit sigil",
		"unlike Adopt Children, it arrives with the stored set already checked",
		"Dependents are never listed",
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
		"`Space` contains exactly: 1. Set Check-In 2. Delete",
		"Repeat offers Off, 1, 3, 7, 14, and 30 days, or a custom positive day count",
		"an empty bulk draft never clears schedules",
		"Comments and Attachments left this menu when they gained `;` and `&`",
		"Project Actions holds Archive project and Delete project under Danger, and nothing else",
		"Editing left this menu when the header gained `e`, and pinning left it when the " +
			"header gained `*`",
		"`Enter` and `Ctrl+N` already reach both from that row",
		"Delete always asks for confirmation",
		"1. Reveal original 2. Open original 3. Open attachments folder 4. Unlink 5. Delete",
		"Its own `Space` sheet holds Delete alone, which confirms and has no direct key",
		"Append `%project` to move its subtree, `>parent` to reparent it, `>0` to move it to the project root, or `<child` once per task you want to adopt",
	})
}

func TestGuideDocumentsPeekAndBulkSemantics(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"A task row is\nits title first",
		"so now are tags, context, the neutral Waiting and Doing ages, deferral markers,\nand the checklist, comment, and attachment counts",
		"a row whose open children are all placed somewhere else ends with `›N`",
		"`Enter` focuses the cursor task's descendants",
		"`Shift+Enter` widens in the other direction",
		"`b` toggles a session-only Peek in a pane docked below the list, whether the\n" +
			"cursor is on a task or on a project header",
		"Its height is a share of the terminal\n" +
			"— about a third of the outline's own room, never fewer than seven rows and never\n" +
			"more than twelve — counted off the list once when you open it",
		"What the pane never reads is the\nrow you are on",
		"Peek gives them up and comes back to its full height when that panel\ncloses",
		"the pane stays where it is and reads `No detail to\nshow for this row.`; `b` closes it again from any row",
		"Peek shows as many rows as its height allows and `Ctrl+D` and\n" +
			"`Ctrl+U` scroll through the rest half a pane a press, while `Ctrl+F` and `Ctrl+B`\n" +
			"move a whole page, one row short of the pane",
		"Entering multi-select closes Peek",
		"`v` toggles the cursor task in or out of the selection",
		"`V` starts a fresh inclusive range at the cursor",
		"`Ctrl+A` replaces the selection with all visible task rows",
		"Successful Delete clears selection; other successes, cancellations, and failures preserve it",
		"A tag row is checked when every selected task carries it, `[-]` when only some do, and empty when none do",
		"applying adds every row you checked, removes every row you unchecked, and leaves every `[-]` row exactly as it is",
		"The `CONTEXTS` rows read `[-]` the same way",
		"picking `No context` clears the field across the whole selection the way `##0` does",
		"A context named `0` is refused",
	})
}

func TestGuideDocumentsFiltersAndSearch(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"`f!0`, `f!1`–`f!3`",
		"Unset never matches",
		// Search composition: a query is one more facet over the mode's own rows,
		// the row facets and the sort axis compose with it either way, the flat
		// ranked list is the only presentation that takes the F family away, and
		// Esc drops the query and restores what it was typed over. The families
		// are named rather than globbed, because `f*` is now a literal key.
		"any `f` facet applies as an AND gate before scoring exactly like a query",
		"an `o` sort reorders within a relevance tier only",
		"The population facets, the `F` family, are the only thing the flat list\ntakes away",
		"Esc drops the query and restores the mode, facets, sort, cursor, scroll,",
		// One narrowing, one spelling: the chip is the query, the chord's
		// threshold is written out, repeats of a single-valued field union, and
		// project selection is one axis the population facets are not part of.
		"the title row shows what you narrowed by in the sigil spelling you would have typed",
		"`f!2` means\n`!2+`, medium-or-higher, and typing `!2` alone matches medium exactly",
		"Repeated `##context` names union",
		"`%slug` inside a query and the `f%` facet narrow the\nrows inside that frame instead",
		"which projects exist to be looked at at all, archived and parked included",
		"`fw` | Pick workflow states with OR semantics, including terminal states; " +
			"it follows the `w` workflow chord rather than the `$destination` sigil it narrows, " +
			"and `f$` is retired",
		"`f#` | Pick tags with AND semantics; the task context is a separate axis on `fc`",
		"`fc` | Pick task contexts with OR semantics, since a task carries at most one",
		"`fa` | Pick completed at least 7, 30, or 90 local-calendar days ago",
		"`f=` | Toggle the review facet, the same `=review` a typed query spells",
		"`f@` | Open the Filter by Due or Deferred picker: five shortcut rows writing `@overdue`, `@today`, `@week`, `@0`, and `?deferred`, each labeled with the token it writes",
		"`f*` | Toggle the pinned facet, the same `*pin` a typed query spells",
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
		// The vocabulary rename `f#` and `fc` offer. The four facts a reader
		// cannot get from the screen are pinned: the change is not scoped by the
		// view it was started from, the pseudo-row is a facet rather than a name,
		// saved lenses are reported and never rewritten, and the browse state the
		// steps ran over is still there afterwards.
		"`e` renames or merges the tag under the cursor",
		"`e` renames or merges the context under the cursor",
		"### Renaming a tag or a context",
		"the change rewrites every unarchived project, and\nthe confirmation says so",
		"It is one\ntransaction and one undo entry",
		"is a facet over the tasks the vocabulary does\nnot cover rather than a context anything stores, so `e` refuses it by name",
		"Saved lenses are left exactly as they were written",
		"the source row becomes\nthe destination row, a merge that had both checked keeps one, and the cursor\nlands on the row the rename produced",
	})
}

func TestGuideDocumentsNamedLenses(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"### Named lenses",
		"`\\` opens Named Lenses, the saved-projection list",
		"`1`–`9` | Apply the Nth saved lens shown, exactly as `Enter` on that row would",
		"the mode, the whole narrowing it applies — every task and\nproject facet, written as one query — and the sort axis",
		"none of them records an undo entry and `u` never takes a lens back",
		"gains a `*`\nas soon as the live view stops matching what was saved",
		"switching straight from one lens to another\nkeeps the original restoration point",
		"While a lens is active `/` is the ordinary scope-wide search, and it puts the\nlens down",
		"One `Esc` puts that lens back as you left it",
		"The visit is not an edit: the saved definition is untouched",
		"A lens owns project scope too, and saving is what decides whether it does",
		"Entering or leaving a project scope while a lens is live marks it\nmodified",
		"`Waiting`, `High Priority`, and `Stale`",
		"reapplies that lens's saved axes on top of it",
		"it lands on the Active Projects, All view a lens-free launch would have shown",
	})
}

func TestGuideDocumentsCaptureAndWorkflowContracts(t *testing.T) {
	content := readGuideCorpus(t)
	requireDocumentationText(t, content, []string{
		"!1-3 #tag ##context @date @@defer $destination *pin =review %project >parent <child ~task &link ;comment;",
		"`@@defer` hides the task until an instant rather than a day",
		"`*pin` pins the task, exactly as the `*` key does on an existing row",
		"The edit surface is also the only one that accepts `*unpin`",
		"a pinned task arrives carrying `*pin` and deleting that token unpins it",
		"`<child` adopts a task that already exists, and may be repeated",
		"There is no clear form: `<0` is an ordinary reference that resolves to nothing",
		"Aliases that name the same task collapse to one adoption, keeping the first occurrence",
		"`^type`, priorities `!4`–`!5`, `$ready`, and `$in_progress`",
		"The tilde no longer names recurrence, which stays retired",
		"`~task` records a soft dependency on another task and may be repeated",
		"The edit surfaces add `-~task`, which removes one reference, and `~0`, which clears them all",
		"On an existing row the `~` key opens Set Dependencies over the same set, " +
			"with the stored references already checked",
		"`#tag` creates and sets a lowercase ASCII kebab tag on the fly",
		"`inbox`, `backlog`, `today`, `next`, `doing`, `waiting`, `paused`, `someday`, `done`, or `canceled`",
		"A new child defaults to Backlog",
		"Exact case-insensitive `bug`, `question`, `epic`, `story`, `feature`, and `idea`",
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
		"`Esc` unwinds one rung at a time: the action sheet, then the query, then the selection, " +
			"then the surface itself",
		"`Space` on a project group header opens Project Actions",
		"Deleting a project is the one act with no second chance, and it asks first, naming the cascade " +
			"by name: the project's tasks, its notes, and its attachments all go",
		"The removal and the catalog and planning rewrites beside it are one transaction, so `u` brings " +
			"every byte back",
		"The system Unfiled project cannot be deleted, and an archived project still can be",
		// Project tags.
		"### Project tags",
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
		"`y` and `Y` copy that note's title and its full file path",
		"`Space` opens the note action sheet, which holds Delete alone and confirms before deleting; " +
			"deletion has no direct key",
		"Notes has no multi-select, so that sheet always acts on the one note under the cursor",
		"Notes support open/edit, create, rename, move, copy, delete, undo, and redo",
		"Note tags, task links, pins, bulk actions, and archived-project ownership remain excluded",
		"At 72 columns and wider, Notes shows the selected note beside the list",
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
		"schema-19 Markdown and generic `Task` headings",
		"A project file may also hold comments that belong to the project itself rather than to " +
			"any one task",
		"Schema 19 upgrades task Markdown to schema 17 by adding the optional `PROJECT-COMMENTS` " +
			"block",
		"`pre-project-comments-v19-<UTC>.tar.gz`",
		"Schema 21 upgrades task Markdown to schema 19 by adding the optional `check_in` date, " +
			"its `check_in_repeat` cadence, and the `reviewed` instant",
		"`pre-check-ins-v21-<UTC>.tar.gz`",
		"A task may also carry a check-in: an optional `check_in` calendar date saying when you " +
			"next mean to look at it",
		"Finishing or canceling a task drops the date and the cadence, because terminal work owes " +
			"nothing; the `reviewed` instant survives",
		"A task may also hold one resume note, a single line of at most 240 characters",
		"Each task may finally carry an optional `depends` list of soft references",
		"Each task may carry an optional non-negative `defer_count`; zero is omitted",
		"`lenses.json` contains your saved named-lens definitions",
		"Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the dark/light theme " +
			"choice, whether the Outliner draws its menu bar, whether it draws ambient advisories, and " +
			"a pointer to the lens that was last open",
		"A machine that has never pressed `M` writes no menu-bar key at all, and the absent key is what " +
			"says the bar is hidden",
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
		"pre-portable-lenses-v16-<UTC>.tar.gz",
		"pre-resume-note-v18-<UTC>.tar.gz",
		"Schema 13 upgrades task Markdown to schema 12",
		"Schema 14 upgrades task Markdown to schema 13 by adding the optional `due_time`",
		"Schema 15 upgrades task Markdown to schema 14 by adding the optional `depends`",
		"Schema 16 leaves task Markdown at schema 14",
		"durable `state_changed` clock, backfilled from `updated`",
		"There is no down-migration",
		"keep your own copy of the workspace directory before a version jump",
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
			"`:`, `C`, and `Ctrl+Esc`",
		"Two names came back with new acts rather than as aliases: `t` is the task-subtree fold and " +
			"`P` is Select Project",
		"`c` is a fourth name back with a new act: it appends a checklist item to the " +
			"cursor task, answering from the owner row and from an item row alike, and it is " +
			"the return that retired a key in exchange, since task-row `x` gave up starting a " +
			"checklist to it and checklist-row `A` gave up appending, both now silent with no " +
			"alias back",
		"`C` is a fifth name back with a new act: it appends a comment to whatever the " +
			"cursor names — the task, or the project a group header names — without leaving " +
			"the list, the write beside the read `;` already opens",
		"`b` is a sixth: it toggles Peek",
		"`R` is a seventh: it marks the cursor task reviewed and moves to the next candidate",
		"`d` is unbound: Done stays `wd` alone, `@` answers both dates, and root `z` is the scroll chord prefix",
		"`w` joins them as a chord prefix rather than a key of its own, and `$` left the keyboard " +
			"entirely when the workflow chord took that letter: the sigil is still what you type on " +
			"every mutation surface, but there is no root `$` and no `f$`, and neither is kept as an alias",
		"`wq is not a workflow action`; pressing a key that was never bound does nothing and says nothing",
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
		"Review answers to `=` on every surface",
		"`$review` stays a rejected destination",
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
		{file: "docs/guide/outliner-rows.md", heading: "## Inline checklist rows"},
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
