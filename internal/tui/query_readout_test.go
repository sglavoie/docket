package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// readoutTestApp builds a workspace with enough narrowable axes that the stack
// can be read, peeled, and overflowed by the tests below.
func readoutTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: "one", ShortID: 1, Title: "One", WorkflowState: models.StateBacklog, Priority: 1},
			{ID: "two", ShortID: 2, Title: "Two", WorkflowState: models.StateWaiting, Tags: []string{"deep"}},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

// readoutBodyText renders the stack the way the strip does and strips styling, so
// an assertion reads the characters a user sees.
func readoutBodyText(out *OutlinerModel, width int) string {
	return ansi.Strip(out.queryReadoutBody(width))
}

// TestQueryReadoutNamesEveryActiveAxisAndNothingElse holds the readout to the one
// thing it promises: every live narrowing gets a row spelled in the canonical
// query characters, and an axis at its default gets none. The mode row carries no
// clauses because the mode catalog declares none, so naming any would be the
// readout claiming what it cannot prove.
func TestQueryReadoutNamesEveryActiveAxisAndNothingElse(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	app := readoutTestApp(t)
	out := app.crossOutliner
	out.toggleQueryReadout()

	if body := readoutBodyText(out, 80); !strings.Contains(body, "nothing: every axis is at its default") {
		t.Fatalf("unnarrowed readout = %q, want the empty reading", body)
	}

	out.filter.Priority = models.PriorityHigh
	out.filter.Tags = map[string]bool{"deep": true}
	out.filter.Pinned = true
	out.adoptFilterCarriers()
	if _, _ = out.selectOutlinerMode(chrome.ModeBacklog); out.activeOutlinerModeEntry().ID != chrome.ModeBacklog {
		t.Fatal("the test could not reach a non-All mode")
	}

	rows := out.queryReadoutRows()
	if len(rows) != 4 {
		t.Fatalf("rows = %d (%+v), want mode plus three facets", len(rows), rows)
	}
	if rows[0].axis != queryReadoutAxisMode {
		t.Fatalf("first row = %+v, want the mode row above the facets", rows[0])
	}
	body := readoutBodyText(out, 80)
	for _, want := range []string{"Narrowing this view", "1 mode", "!1+", "#deep", "*pin"} {
		if !strings.Contains(body, want) {
			t.Errorf("readout body %q omits %q", body, want)
		}
	}
	// The three facets are three rows, not one aggregated query row: the peel unit
	// is the facet, which is what makes a single digit remove a single narrowing.
	for _, digit := range []string{"2 filter", "3 filter", "4 filter"} {
		if !strings.Contains(body, digit) {
			t.Errorf("readout body %q omits the peel digit %q", body, digit)
		}
	}
}

// TestQueryReadoutPeelRemovesOneAxisAndIsIdempotentAtZero covers the peel
// contract from both ends: one digit takes exactly one narrowing away and leaves
// the rest standing, and a digit past the stack writes nothing at all.
func TestQueryReadoutPeelRemovesOneAxisAndIsIdempotentAtZero(t *testing.T) {
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.filter.Priority = models.PriorityHigh
	out.filter.Stale = true
	out.adoptFilterCarriers()
	out.toggleQueryReadout()

	before := out.queryReadoutRows()
	if len(before) != 2 {
		t.Fatalf("rows = %d (%+v), want the two facets", len(before), before)
	}

	out.handleQueryReadoutKey("1")
	after := out.queryReadoutRows()
	if len(after) != 1 {
		t.Fatalf("rows after one peel = %d (%+v), want exactly one removed", len(after), after)
	}
	if out.filter.Priority != 0 {
		t.Errorf("Priority = %d after peeling it, want cleared", out.filter.Priority)
	}
	if !out.filter.Stale {
		t.Error("peeling the priority facet also cleared the stale facet")
	}
	if !out.queryReadout.expanded {
		t.Error("peeling closed the readout; the stack stays open so the next digit is honest")
	}

	out.handleQueryReadoutKey("9")
	if len(out.queryReadoutRows()) != 1 {
		t.Fatalf("a digit past the stack changed it: %+v", out.queryReadoutRows())
	}
	if !out.filter.Stale {
		t.Error("a digit past the stack cleared a facet")
	}
	if out.StatusMessage != "Nothing to peel there" {
		t.Errorf("status = %q, want the idempotent reading", out.StatusMessage)
	}

	out.handleQueryReadoutKey("1")
	if rows := out.queryReadoutRows(); len(rows) != 0 {
		t.Fatalf("rows after peeling the last facet = %+v, want none", rows)
	}
	out.handleQueryReadoutKey("1")
	if out.StatusMessage != "Nothing to peel there" {
		t.Errorf("peeling an empty stack = %q, want the idempotent reading", out.StatusMessage)
	}
}

// TestQueryReadoutPeelAllClearsEveryNarrowingAtOnce walks the same rows x walks,
// including the search axis, which peels above the facets so a ranked projection
// is gone before a facet is reached.
func TestQueryReadoutPeelAllClearsEveryNarrowingAtOnce(t *testing.T) {
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.filter.Priority = models.PriorityHigh
	out.filter.Tags = map[string]bool{"deep": true}
	out.adoptFilterCarriers()
	out.selectOutlinerMode(chrome.ModeBacklog)
	out.SetSearchQuery("one")
	out.toggleQueryReadout()

	if len(out.queryReadoutRows()) != 4 {
		t.Fatalf("rows = %+v, want mode, query, and two facets", out.queryReadoutRows())
	}
	out.handleQueryReadoutKey("x")
	if rows := out.queryReadoutRows(); len(rows) != 0 {
		t.Fatalf("rows after peel-all = %+v, want none", rows)
	}
	if out.SearchQuery() != "" || out.activeOutlinerModeEntry().ID != chrome.ModeAll {
		t.Errorf("peel-all left query %q and mode %v", out.SearchQuery(), out.activeOutlinerModeEntry().ID)
	}
	// Five, not four: clearing a search restores the presentation snapshot it was
	// opened over, which brings the mode back, and the walk takes it away again.
	// The count reports acts performed rather than rows first seen.
	if !strings.Contains(out.StatusMessage, "Peeled 5") {
		t.Errorf("status = %q, want the count it removed", out.StatusMessage)
	}
}

// TestQueryReadoutOwnsInputWhileOpen keeps the stack modal. Its digits are a
// gated keymap the footer advertises, so a root key underneath must not act: it
// closes the readout instead, and the footer says so while it is open.
func TestQueryReadoutOwnsInputWhileOpen(t *testing.T) {
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.filter.Stale = true
	out.adoptFilterCarriers()
	out.toggleQueryReadout()

	guidance, ok := out.ContextualGuidance()
	if !ok {
		t.Fatal("the open readout supplied no footer")
	}
	rendered := guidanceHintLabels(guidance)
	for _, want := range []string{"peel one", "x peel all", "save as lens"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("readout footer %q omits %q", rendered, want)
		}
	}

	// v opens a visual selection everywhere else in the Outliner; here it is an
	// unlisted key, so it may only put the stack down.
	if _, _ = out.handleQueryReadoutKey("v"); out.queryReadout.expanded {
		t.Error("an unlisted key left the readout open")
	}
	if out.selectionCounts().Total > 0 {
		t.Error("an unlisted key ran the act its root binding names")
	}

	out.toggleQueryReadout()
	if _, cmd := out.handleQueryReadoutKey("esc"); cmd != nil || out.queryReadout.expanded {
		t.Error("Esc did not close the readout on its own")
	}
}

// TestQueryReadoutSaveAsLensRoutesThroughTheShell holds the save path to the one
// executor: the readout raises a request and the shell runs Create, so the name
// prompt and the lens transaction stay where every other lens write happens.
func TestQueryReadoutSaveAsLensRoutesThroughTheShell(t *testing.T) {
	app := readoutTestApp(t)
	if err := app.registerLensOperations(); err != nil {
		t.Fatal(err)
	}
	out := app.crossOutliner
	out.toggleQueryReadout()

	_, cmd := out.handleQueryReadoutKey("s")
	if cmd == nil {
		t.Fatal("s raised no save request")
	}
	if out.queryReadout.expanded {
		t.Error("s left the readout open over the name prompt")
	}
	msg := cmd()
	if _, ok := msg.(SaveViewAsLensRequestMsg); !ok {
		t.Fatalf("s raised %T, want SaveViewAsLensRequestMsg", msg)
	}
	handled, saveCmd := app.handleOperationOverlayMessage(msg)
	if !handled || saveCmd == nil {
		t.Fatalf("the shell = handled %v cmd %T, want the name prompt opened", handled, saveCmd)
	}
	if app.operationTextPrompt == nil {
		t.Fatal("saving from the readout did not reach the lens name prompt")
	}
}

// TestQueryReadoutGrowsTheStatusStripBelowTheConflictLine pins the placement: the
// stack is part of the strip whose height statusStripHeight reserves, and the
// outstanding-conflict count keeps the lead above it.
func TestQueryReadoutGrowsTheStatusStripBelowTheConflictLine(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	const width = 80
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.filter.Stale = true
	out.adoptFilterCarriers()
	out.setOutstandingConflicts(func() int { return 2 })

	collapsedHeight := out.statusStripHeight(width)
	out.toggleQueryReadout()
	expandedHeight := out.statusStripHeight(width)
	if expandedHeight <= collapsedHeight {
		t.Fatalf("strip height = %d expanded and %d collapsed, want the stack reserved", expandedHeight, collapsedHeight)
	}

	lines := strings.Split(ansi.Strip(out.statusStripBody(width)), "\n")
	if !strings.Contains(lines[0], "2 unresolved merge conflicts") {
		t.Fatalf("first strip line = %q, want the conflict count leading", lines[0])
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Narrowing this view") || !strings.Contains(joined, "?stale") {
		t.Fatalf("strip body omits the readout:\n%s", joined)
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > width {
			t.Errorf("strip line overflows the width: %q", line)
		}
	}
}

// TestQueryReadoutPeelReloadsThePopulationThroughTheShell keeps the one facet
// class that decides which projects load honest: peeling it asks the shell to
// reload rather than rebuilding rows over a snapshot the peel invalidated.
func TestQueryReadoutPeelReloadsThePopulationThroughTheShell(t *testing.T) {
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.projectFilter.ArchivedMode = query.ProjectPopulationInclude
	out.adoptFilterCarriers()
	out.toggleQueryReadout()

	rows := out.queryReadoutRows()
	if len(rows) != 1 || rows[0].label != "population" {
		t.Fatalf("rows = %+v, want the one population row", rows)
	}
	cmd := out.peelQueryReadoutRow(0)
	if cmd == nil {
		t.Fatal("peeling a population facet raised no reload")
	}
	msg, ok := cmd().(QueryReadoutReloadRequestMsg)
	if !ok {
		t.Fatalf("population peel raised %T, want a reload request", msg)
	}
	if out.projectFilter.ArchivedMode == query.ProjectPopulationInclude {
		t.Error("the peel did not write the population fields before asking for the reload")
	}
	if handled, reload := app.handleOperationOverlayMessage(msg); !handled || reload == nil {
		t.Fatalf("the shell = handled %v cmd %T, want the population reloaded", handled, reload)
	}
}
