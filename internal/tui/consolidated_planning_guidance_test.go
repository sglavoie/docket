package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestConsolidatedPlanningGuidanceMatchesRuntime is the completion-contract
// regression for the consolidated-planning-guidance repair (FR-012-003). It
// resolves the authoritative live routing for the ;q / ;w / ;D shortcuts, then
// checks the three maintained guidance surfaces against it and against the
// consolidated vocabulary:
//
//  1. Semicolon prefix hints must label q as Planner Next, w as Planner Today,
//     and D as All Tasks by Due Date (never the retired "backlog" / "overview").
//  2. Palette command names, descriptions, and aliases must unambiguously
//     distinguish Planner Next from the ranked Backlog, and the inbox router to
//     the Next section must never call it a Today section.
//  3. The maintained Focus tutorial must teach StateNext membership and the
//     ordering-only index and point at the Planner, and no maintained planning
//     tutor topic may reference the deleted Focus View.
func TestConsolidatedPlanningGuidanceMatchesRuntime(t *testing.T) {
	// --- 1. Resolve authoritative live routing. ---
	qTab := routeSemicolonPlannerTab(t, 'q')
	wTab := routeSemicolonPlannerTab(t, 'w')
	if qTab != "Next" {
		t.Fatalf(";q opens Planner %q tab, want Next", qTab)
	}
	if wTab != "Today" {
		t.Fatalf(";w opens Planner %q tab, want Today", wTab)
	}
	if grp := routeSemicolonAllTasksGroup(t, 'D'); grp != ATGroupByDueDate {
		t.Fatalf(";D opens All Tasks group %v, want ATGroupByDueDate", grp)
	}

	// --- 2. Semicolon prefix hints must match the routes with the consolidated
	//        vocabulary. ---
	hints := map[string]PrefixHint{}
	for _, h := range SemicolonPrefixHints {
		hints[h.Key] = h
	}
	assertHint := func(key, wantLabel string, forbidden ...string) {
		t.Helper()
		h, ok := hints[key]
		if !ok {
			t.Fatalf("SemicolonPrefixHints missing key %q", key)
		}
		joined := h.Description + " " + h.Detail
		if !strings.Contains(joined, wantLabel) {
			t.Errorf(";%s hint = %q / %q, want label %q", key, h.Description, h.Detail, wantLabel)
		}
		low := strings.ToLower(joined)
		for _, bad := range forbidden {
			if strings.Contains(low, strings.ToLower(bad)) {
				t.Errorf(";%s hint = %q / %q, must not contain retired term %q", key, h.Description, h.Detail, bad)
			}
		}
	}
	assertHint("q", "Planner Next", "backlog")
	assertHint("w", "Planner Today")
	assertHint("D", "All Tasks by Due Date", "overview")

	// --- 3. Palette commands: Planner Next, Planner Today, and the ranked
	//        Backlog must be unambiguous. ---
	app := newTestAppModel()
	cmds := app.viewCommands()

	nextCmd := commandByShortcut(t, cmds, ";q")
	todayCmd := commandByShortcut(t, cmds, ";w")
	backlogCmd := findCommand(cmds, "planner backlog")
	if backlogCmd == nil {
		t.Fatal("expected ranked \"planner backlog\" command")
	}

	assertOpensPlannerTab(t, nextCmd, boardTabNext)
	assertOpensPlannerTab(t, todayCmd, boardTabToday)
	assertOpensPlannerTab(t, backlogCmd, boardTabBacklog)

	// The ;q command must reference Next and must not be named after the ranked
	// Backlog (the historical "backlog" collision this repair removes).
	if strings.Contains(strings.ToLower(nextCmd.Name), "backlog") {
		t.Errorf(";q command is named %q, which collides with the ranked Backlog", nextCmd.Name)
	}
	if !strings.Contains(strings.ToLower(nextCmd.Name+" "+nextCmd.Description), "next") {
		t.Errorf(";q command %q/%q never names Next", nextCmd.Name, nextCmd.Description)
	}

	// Planner Next and the ranked Backlog must share no name/alias, and the bare
	// term "backlog" must resolve only to the ranked Backlog.
	nextVocab := commandVocabulary(nextCmd)
	backlogVocab := commandVocabulary(backlogCmd)
	for term := range nextVocab {
		if backlogVocab[term] {
			t.Errorf("Planner Next and ranked Backlog both answer to %q; the label is ambiguous", term)
		}
	}
	if nextVocab["backlog"] {
		t.Errorf("Planner Next answers to bare %q, which belongs to the ranked Backlog", "backlog")
	}

	// The inbox command that routes a task to the Next section must never call
	// it a Today section.
	sendNext := findCommand(app.inboxProcessorCommands(), "send to next")
	if sendNext == nil {
		t.Fatal("expected inbox \"send to next\" command")
	}
	if blob := strings.ToLower(sendNext.Description + " " + strings.Join(sendNext.Aliases, " ")); strings.Contains(blob, "today") {
		t.Errorf("%q routes to the Next section but calls it Today: %q / %v", sendNext.Name, sendNext.Description, sendNext.Aliases)
	}

	// --- 4. Maintained planning tutor topics. ---
	focus := findTutorTopicByTitle(t, "Focus & Suggestions")
	for _, want := range []string{"StateNext", "Planner", "ordering-only"} {
		if !strings.Contains(focus.Content, want) {
			t.Errorf("Focus & Suggestions tutorial missing consolidated term %q:\n%s", want, focus.Content)
		}
	}
	// Every maintained planning tutorial must reject the pre-consolidation
	// framing where Next/queue membership is independent of workflow state and
	// q leaves the state unchanged. These phrases are distinctive of the
	// obsolete copy and never appear in the corrected state-driven wording.
	stateIndependentQueueWording := []string{
		"independent, ordered",
		"without changing its section",
		"does not mark",
		"leaves existing queue items alone",
	}
	for title, content := range maintainedPlanningTutorTopics(t) {
		if strings.Contains(strings.ToLower(content), "focus view") {
			t.Errorf("planning tutor topic %q references the deleted Focus View:\n%s", title, content)
		}
		low := strings.ToLower(content)
		for _, bad := range stateIndependentQueueWording {
			if strings.Contains(low, bad) {
				t.Errorf("planning tutor topic %q teaches state-independent queue wording %q:\n%s", title, bad, content)
			}
		}
		snippet := map[string]string{title: content}
		assertShortcutTabPairing(t, snippet, ";q", qTab)
		assertShortcutTabPairing(t, snippet, ";w", wTab)
	}
}

// TestPlanningDocumentationMatchesStateDrivenNext prevents the maintained
// public documentation from drifting back to the retired model where Focus
// Queue membership was independent of workflow state.
func TestPlanningDocumentationMatchesStateDrivenNext(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))

	documents := map[string]string{}
	for _, name := range []string{"README.md", "docs/architecture.md"} {
		contents, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			t.Fatalf("read maintained planning document %s: %v", name, err)
		}
		documents[name] = string(contents)
	}

	required := map[string][]string{
		"README.md": {
			"workflow state `StateNext`",
			"ordering-only index",
			"does not confer Next membership",
			"config-backed Daily Plan",
			"mutually exclusive",
		},
		"docs/architecture.md": {
			"`StateNext`",
			"ordering-only",
			"config-backed planning collection",
			"mutually exclusive",
			"transitions it from `StateNext` to `StateReady`",
			"Entering Next atomically removes any Today membership",
		},
	}
	for name, phrases := range required {
		for _, phrase := range phrases {
			if !strings.Contains(documents[name], phrase) {
				t.Errorf("%s missing state-driven planning wording %q", name, phrase)
			}
		}
	}

	retired := []string{
		"separate queue",
		"focus queue is a separate",
		"independent of workflow state",
		"state-independent membership",
		"queue membership does not change",
		"neither changes a task's workflow state or section",
		"without rewriting its lifecycle",
	}
	for name, contents := range documents {
		low := strings.ToLower(contents)
		for _, phrase := range retired {
			if strings.Contains(low, phrase) {
				t.Errorf("%s retains retired state-independent planning wording %q", name, phrase)
			}
		}
	}
}

// routeSemicolonAllTasksGroup drives the ;<key> dispatcher and returns the All
// Tasks grouping the resulting navigation command opens.
func routeSemicolonAllTasksGroup(t *testing.T, key rune) AllTasksGroupBy {
	t.Helper()
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	handled, cmd := app.handlePendingSemicolonPrefix(tea.KeyPressMsg{Code: key, Text: string(key)})
	if !handled || cmd == nil {
		t.Fatalf(";%c did not produce a navigation command", key)
	}
	msg := cmd()
	show, ok := msg.(ShowAllTasksMsg)
	if !ok {
		t.Fatalf(";%c routed to %T, want ShowAllTasksMsg", key, msg)
	}
	if !show.SetGroup {
		t.Fatalf(";%c produced ShowAllTasksMsg without SetGroup: %+v", key, show)
	}
	return show.Group
}

// commandByShortcut returns the palette command bound to the given shortcut.
func commandByShortcut(t *testing.T, cmds []Command, shortcut string) *Command {
	t.Helper()
	for i := range cmds {
		if cmds[i].Shortcut == shortcut {
			return &cmds[i]
		}
	}
	t.Fatalf("no palette command bound to shortcut %q", shortcut)
	return nil
}

// assertOpensPlannerTab executes a command's handler and fails unless it opens
// the Planner at the expected tab.
func assertOpensPlannerTab(t *testing.T, cmd *Command, wantTab int) {
	t.Helper()
	msg := cmd.Handler()
	show, ok := msg.(ShowFocusBoardMsg)
	if !ok {
		t.Fatalf("command %q routed to %T, want ShowFocusBoardMsg", cmd.Name, msg)
	}
	if !show.SetTab || show.Tab != wantTab {
		t.Errorf("command %q opened tab %d (SetTab=%v), want tab %d", cmd.Name, show.Tab, show.SetTab, wantTab)
	}
}

// commandVocabulary returns the lowercased set of a command's name and aliases
// — the terms palette search resolves it by.
func commandVocabulary(c *Command) map[string]bool {
	v := map[string]bool{strings.ToLower(c.Name): true}
	for _, a := range c.Aliases {
		v[strings.ToLower(a)] = true
	}
	return v
}

// maintainedPlanningTutorTopics returns the planning-related tutor topics whose
// copy must stay consistent with the consolidated Planner model.
func maintainedPlanningTutorTopics(t *testing.T) map[string]string {
	t.Helper()
	titles := []string{
		"Focus & Suggestions",
		"Sections (GTD)",
		"Focus",
		"The pipeline",
		"The Task Pipeline",
	}
	out := map[string]string{}
	for _, title := range titles {
		out[title] = findTutorTopicByTitle(t, title).Content
	}
	return out
}
