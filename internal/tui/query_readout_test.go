package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

// readoutBodyText renders the stack the way the strip does and strips styling, so
// an assertion reads the characters a user sees.
func readoutBodyText(out *outliner.OutlinerModel, width int) string {
	return ansi.Strip(out.QueryReadoutBody(width))
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
	out.ToggleQueryReadout()

	if body := readoutBodyText(out, 80); !strings.Contains(body, "nothing: every axis is at its default") {
		t.Fatalf("unnarrowed readout = %q, want the empty reading", body)
	}

	out.Filter.Priority = models.PriorityHigh
	out.Filter.Tags = map[string]bool{"deep": true}
	out.Filter.Pinned = true
	adoptFilterCarriers(out)
	if _, _ = out.SelectOutlinerMode(chrome.ModeBacklog); out.ActiveOutlinerModeEntry().ID != chrome.ModeBacklog {
		t.Fatal("the test could not reach a non-All mode")
	}

	rows := out.QueryReadoutRows()
	if len(rows) != 4 {
		t.Fatalf("rows = %d (%+v), want mode plus three facets", len(rows), rows)
	}
	if rows[0].Axis != outliner.QueryReadoutAxisMode {
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
	out.Filter.Priority = models.PriorityHigh
	out.Filter.Stale = true
	adoptFilterCarriers(out)
	out.ToggleQueryReadout()

	before := out.QueryReadoutRows()
	if len(before) != 2 {
		t.Fatalf("rows = %d (%+v), want the two facets", len(before), before)
	}

	out.HandleQueryReadoutKey("1")
	after := out.QueryReadoutRows()
	if len(after) != 1 {
		t.Fatalf("rows after one peel = %d (%+v), want exactly one removed", len(after), after)
	}
	if out.Filter.Priority != 0 {
		t.Errorf("Priority = %d after peeling it, want cleared", out.Filter.Priority)
	}
	if !out.Filter.Stale {
		t.Error("peeling the priority facet also cleared the stale facet")
	}
	if !out.QueryReadout.Expanded {
		t.Error("peeling closed the readout; the stack stays open so the next digit is honest")
	}

	out.HandleQueryReadoutKey("9")
	if len(out.QueryReadoutRows()) != 1 {
		t.Fatalf("a digit past the stack changed it: %+v", out.QueryReadoutRows())
	}
	if !out.Filter.Stale {
		t.Error("a digit past the stack cleared a facet")
	}
	if out.StatusMessage != "Nothing to peel there" {
		t.Errorf("status = %q, want the idempotent reading", out.StatusMessage)
	}

	out.HandleQueryReadoutKey("1")
	if rows := out.QueryReadoutRows(); len(rows) != 0 {
		t.Fatalf("rows after peeling the last facet = %+v, want none", rows)
	}
	out.HandleQueryReadoutKey("1")
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
	out.Filter.Priority = models.PriorityHigh
	out.Filter.Tags = map[string]bool{"deep": true}
	adoptFilterCarriers(out)
	out.SelectOutlinerMode(chrome.ModeBacklog)
	out.SetSearchQuery("one")
	out.ToggleQueryReadout()

	if len(out.QueryReadoutRows()) != 4 {
		t.Fatalf("rows = %+v, want mode, query, and two facets", out.QueryReadoutRows())
	}
	out.HandleQueryReadoutKey("x")
	if rows := out.QueryReadoutRows(); len(rows) != 0 {
		t.Fatalf("rows after peel-all = %+v, want none", rows)
	}
	if out.SearchQuery() != "" || out.ActiveOutlinerModeEntry().ID != chrome.ModeAll {
		t.Errorf("peel-all left query %q and mode %v", out.SearchQuery(), out.ActiveOutlinerModeEntry().ID)
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
	out.Filter.Stale = true
	adoptFilterCarriers(out)
	out.ToggleQueryReadout()

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
	if _, _ = out.HandleQueryReadoutKey("v"); out.QueryReadout.Expanded {
		t.Error("an unlisted key left the readout open")
	}
	if out.SelectionCounts().Total > 0 {
		t.Error("an unlisted key ran the act its root binding names")
	}

	out.ToggleQueryReadout()
	if _, cmd := out.HandleQueryReadoutKey("esc"); cmd != nil || out.QueryReadout.Expanded {
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
	out.ToggleQueryReadout()

	_, cmd := out.HandleQueryReadoutKey("s")
	if cmd == nil {
		t.Fatal("s raised no save request")
	}
	if out.QueryReadout.Expanded {
		t.Error("s left the readout open over the name prompt")
	}
	msg := cmd()
	if _, ok := msg.(outliner.SaveViewAsLensRequestMsg); !ok {
		t.Fatalf("s raised %T, want SaveViewAsLensRequestMsg", msg)
	}
	handled, saveCmd := app.handleOperationOverlayMessage(msg)
	if !handled || saveCmd == nil {
		t.Fatalf("the shell = handled %v cmd %T, want the name prompt opened", handled, saveCmd)
	}
	if app.operations.TextPrompt == nil {
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
	out.Filter.Stale = true
	adoptFilterCarriers(out)
	out.SetOutstandingConflicts(func() int { return 2 })

	collapsedHeight := out.StatusStripHeight(width)
	out.ToggleQueryReadout()
	expandedHeight := out.StatusStripHeight(width)
	if expandedHeight <= collapsedHeight {
		t.Fatalf("strip height = %d expanded and %d collapsed, want the stack reserved", expandedHeight, collapsedHeight)
	}

	lines := strings.Split(ansi.Strip(out.StatusStripBody(width)), "\n")
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

// overflowingReadoutApp narrows the view past what a short terminal can draw at
// once, which is the only state in which the readout's window has a position to
// move. The size is the lever rather than the axis count: the budget is what the
// view has left over, so a small frame reaches the overflow with a stack a
// reader can still check by hand.
func overflowingReadoutApp(t *testing.T) *AppModel {
	t.Helper()
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.SetSize(60, 14)
	out.Filter.Priority = models.PriorityHigh
	out.Filter.Tags = map[string]bool{"deep": true}
	out.Filter.Pinned = true
	out.Filter.Stale = true
	out.Filter.Review = true
	out.Filter.Deferred = true
	adoptFilterCarriers(out)
	out.SelectOutlinerMode(chrome.ModeBacklog)
	out.Sort.Field = query.SortByPriority
	out.MultiSelectMode = true
	out.Selected = map[taskref.TaskIdentity]bool{{ProjectID: "project", TaskID: "one"}: true}
	out.ToggleQueryReadout()
	if !strings.Contains(readoutBodyText(out, 60), "below") {
		t.Fatalf("the staged readout fits its window, so there is nothing to scroll:\n%s",
			readoutBodyText(out, 60))
	}
	return app
}

// TestQueryReadoutScrollsItsOwnWindowAndNothingElse is the bounded viewport's
// contract from both sides: the six keys walk the readout's own window to each
// document bound, and the outline they are borrowed from does not move while
// they do. A reader scrolling a readout is reading, not navigating.
func TestQueryReadoutScrollsItsOwnWindowAndNothingElse(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	app := overflowingReadoutApp(t)
	out := app.crossOutliner
	cursor, scroll := out.Cursor, out.ViewScrollOffset

	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("the readout opens mid-document:\n%s", got)
	}
	out.HandleQueryReadoutKey("j")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "1 above") {
		t.Fatalf("j did not move the window:\n%s", got)
	}
	out.HandleQueryReadoutKey("k")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("k did not move the window back:\n%s", got)
	}
	// k at the top and j at the bottom are the clamp, not a wrap: the document has
	// two ends and the window stops at each of them.
	out.HandleQueryReadoutKey("k")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("k past the top left the window off the document:\n%s", got)
	}
	out.HandleQueryReadoutKey("end")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 below") {
		t.Fatalf("End did not reach the document's end:\n%s", got)
	}
	out.HandleQueryReadoutKey("j")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 below") {
		t.Fatalf("j past the end left the window off the document:\n%s", got)
	}
	out.HandleQueryReadoutKey("pgup")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("a page up from the end did not reach the top of this short document:\n%s", got)
	}
	out.HandleQueryReadoutKey("pgdown")
	if got := readoutBodyText(out, 60); strings.Contains(got, "0 above") {
		t.Fatalf("PgDn did not move the window:\n%s", got)
	}
	out.HandleQueryReadoutKey("home")
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("Home did not return to the first line:\n%s", got)
	}

	if !out.QueryReadout.Expanded {
		t.Fatal("a scrolling key closed the readout; all six are listed")
	}
	if out.Cursor != cursor || out.ViewScrollOffset != scroll {
		t.Fatalf("the outline moved to cursor %d offset %d while the readout scrolled, want %d/%d",
			out.Cursor, out.ViewScrollOffset, cursor, scroll)
	}
	if out.SelectionCounts().Total != 1 {
		t.Fatalf("selection = %d after scrolling the readout, want the staged one untouched",
			out.SelectionCounts().Total)
	}
	guidance, ok := out.ContextualGuidance()
	if !ok || !strings.Contains(guidanceHintLabels(guidance), "j/k scroll") {
		t.Fatalf("the overflowing readout's footer omits the scroll keys: %q", guidanceHintLabels(guidance))
	}
}

const (
	// readoutResizeWidth is the one width the resize test renders and sizes at, so
	// the wrapped document it measures is the document the model scrolls.
	readoutResizeWidth = 60
	// readoutResizeShortHeight and readoutResizeTallHeight are the frame before and
	// after the growth. At this width the document is 14 wrapped lines either way:
	// the short frame draws a 6-line window (End seats the offset at 8) and the
	// tall one a 7-line window (maximum 7), so the growth leaves the stored offset
	// exactly one line past the end and a page turn is the window's own 7.
	readoutResizeShortHeight = 12
	readoutResizeTallHeight  = 13
)

// deeplyOverflowingReadoutApp stages a readout whose document is more than twice
// the window drawn over it, which is the shape a page turn needs before it has
// room to come up short. overflowingReadoutApp narrows enough to overflow but not
// enough to page, so the extra facets are the lever here the way the frame size
// is there.
func deeplyOverflowingReadoutApp(t *testing.T, height int) *AppModel {
	t.Helper()
	app := overflowingReadoutApp(t)
	out := app.crossOutliner
	out.Filter.Contexts = map[string]bool{"errand": true, "home": true}
	out.Filter.NoContext = true
	out.Filter.DueDate = query.DueDateThisWeek
	out.Filter.Hierarchy = query.HierarchyLeaf
	out.Filter.Dependency = query.DependencyFilterBlocked
	adoptFilterCarriers(out)
	out.SetSize(readoutResizeWidth, height)
	if _, below := readoutWindowPosition(t, out); below == 0 {
		t.Fatalf("the staged readout opens at its end, so there is nothing below to scroll to:\n%s",
			readoutBodyText(out, readoutResizeWidth))
	}
	return app
}

// readoutWindowPosition reads the counts off the readout's own position line,
// which is the only statement of where the window sits that a reader has — and
// therefore the only thing a contract about what the first key moves may assert.
func readoutWindowPosition(t *testing.T, out *outliner.OutlinerModel) (above, below int) {
	t.Helper()
	lines := strings.Split(readoutBodyText(out, readoutResizeWidth), "\n")
	last := lines[len(lines)-1]
	if _, err := fmt.Sscanf(last, "%d above · %d below", &above, &below); err != nil {
		t.Fatalf("readout's last line = %q, want its position line: %v", last, err)
	}
	return above, below
}

// TestQueryReadoutRelativeScrollStartsFromTheVisibleLine holds a relative scroll
// to the line the reader can see rather than to the number the model stored. The
// render clamps the offset without writing it back, so growing the terminal
// leaves the stored offset past the new maximum while the screen already shows
// the clamped line; a delta added to that stale number spends the first k
// normalizing it and the readout does not move, and shortens a page turn by the
// same overrun. Both halves are staged fresh, because one relative key is enough
// to consume the staleness the other one is about.
func TestQueryReadoutRelativeScrollStartsFromTheVisibleLine(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	out := deeplyOverflowingReadoutApp(t, readoutResizeShortHeight).crossOutliner
	cursor, scroll := out.Cursor, out.ViewScrollOffset
	out.HandleQueryReadoutKey("end")
	stale, below := readoutWindowPosition(t, out)
	if below != 0 {
		t.Fatalf("End left %d lines below the window, want the document's end", below)
	}
	out.SetSize(readoutResizeWidth, readoutResizeTallHeight)
	displayed, below := readoutWindowPosition(t, out)
	if below != 0 || displayed >= stale {
		t.Fatalf("the grown frame shows %d above · %d below against %d before it, "+
			"so the stored offset is not past the new end and there is nothing here to hold",
			displayed, below, stale)
	}

	out.HandleQueryReadoutKey("k")
	if above, below := readoutWindowPosition(t, out); above != displayed-1 || below != 1 {
		t.Fatalf("the first k after the growth left %d above · %d below, want %d above · 1 below:\n%s",
			above, below, displayed-1, readoutBodyText(out, readoutResizeWidth))
	}
	if out.Cursor != cursor || out.ViewScrollOffset != scroll {
		t.Fatalf("the outline moved to cursor %d offset %d while the readout scrolled, want %d/%d",
			out.Cursor, out.ViewScrollOffset, cursor, scroll)
	}
	if out.SelectionCounts().Total != 1 {
		t.Fatalf("selection = %d after scrolling the readout, want the staged one untouched",
			out.SelectionCounts().Total)
	}

	// The reference is the same document in the same window reached without a
	// stale offset: End at the grown size seats the window exactly where the
	// resize already drew it, so what a page turn does from there is what a page
	// turn after the resize owes, and the assertion needs no copy of the page
	// arithmetic to say so.
	reference := deeplyOverflowingReadoutApp(t, readoutResizeTallHeight).crossOutliner
	reference.HandleQueryReadoutKey("end")
	reference.HandleQueryReadoutKey("pgup")
	wantAbove, wantBelow := readoutWindowPosition(t, reference)
	if wantAbove >= displayed {
		t.Fatalf("PgUp from the end of the grown readout stayed at %d above, "+
			"so this document cannot page and the comparison below proves nothing", wantAbove)
	}

	paged := deeplyOverflowingReadoutApp(t, readoutResizeShortHeight).crossOutliner
	paged.HandleQueryReadoutKey("end")
	paged.SetSize(readoutResizeWidth, readoutResizeTallHeight)
	paged.HandleQueryReadoutKey("pgup")
	if above, below := readoutWindowPosition(t, paged); above != wantAbove || below != wantBelow {
		t.Fatalf("PgUp after the growth left %d above · %d below, want the %d above · %d below "+
			"the same press reaches from the same visible line:\n%s",
			above, below, wantAbove, wantBelow, readoutBodyText(paged, readoutResizeWidth))
	}
}

// TestQueryReadoutDigitsStayOnTheFirstNineAxesWhileScrolled keeps the digits a
// peel vocabulary rather than a numbering of what is on screen. A reader who
// scrolled away from row one still removes row one by pressing 1, which is what
// lets the window move without minting a binding.
func TestQueryReadoutDigitsStayOnTheFirstNineAxesWhileScrolled(t *testing.T) {
	app := overflowingReadoutApp(t)
	out := app.crossOutliner
	first := out.QueryReadoutRows()[0]
	if first.Axis != outliner.QueryReadoutAxisMode {
		t.Fatalf("first row = %+v, want the mode row", first)
	}

	out.HandleQueryReadoutKey("end")
	out.HandleQueryReadoutKey("1")
	if out.ActiveOutlinerModeEntry().ID != chrome.ModeAll {
		t.Fatalf("1 from a scrolled window peeled %v, want the first axis", out.ActiveOutlinerModeEntry().ID)
	}
	if !out.QueryReadout.Expanded {
		t.Fatal("peeling from a scrolled window closed the readout")
	}
	// The document just got shorter, so the window it left is re-seated rather than
	// stranded past the end where the next key would look inert.
	if got := readoutBodyText(out, 60); strings.Contains(got, "above") && !strings.Contains(got, "0 below") {
		t.Fatalf("the window was not re-seated after the peel shortened the document:\n%s", got)
	}
}

// TestQueryReadoutNamesTheAxesNothingPeels is the other half of what the context
// row promises: an axis it shortened or dropped is readable here whether or not
// a digit could take it away. They are listed last and carry no digit, because a
// digit is a promise the read-only axes cannot keep.
func TestQueryReadoutNamesTheAxesNothingPeels(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	app := readoutTestApp(t)
	out := app.crossOutliner
	out.Sort.Field = query.SortByPriority
	out.MultiSelectMode = true
	out.Selected = map[taskref.TaskIdentity]bool{{ProjectID: "project", TaskID: "one"}: true}
	out.ToggleQueryReadout()

	body := readoutBodyText(out, 80)
	if !strings.Contains(body, "Also in view") {
		t.Fatalf("readout omits the read-only block:\n%s", body)
	}
	for _, entry := range out.BuildViewContextEntries(80) {
		if !entry.ReadOnly {
			continue
		}
		if !strings.Contains(body, entry.Value) {
			t.Errorf("readout omits the %s axis %q:\n%s", entry.Label, entry.Value, body)
		}
	}
	// The peel vocabulary is the peelable rows alone, so a read-only axis takes no
	// digit and the digits below it do not shift when one appears.
	details := strings.Split(body[strings.Index(body, "Also in view"):], "\n")
	for _, line := range details[1:] {
		if line != "" && line[0] >= '1' && line[0] <= '9' {
			t.Errorf("a read-only row opens on a peel digit: %q", line)
		}
	}
}

// TestQueryReadoutScrollableEmptyStackAdvertisesNoPeel is the seam between the
// two halves of the readout. A document long enough to scroll says nothing about
// whether anything on it can be peeled: the read-only block is what made it long,
// and the footer over it must name the scroll keys without offering a digit that
// would take nothing away.
func TestQueryReadoutScrollableEmptyStackAdvertisesNoPeel(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	app := readoutTestApp(t)
	out := app.crossOutliner
	out.SetSize(60, 8)
	out.Sort.Field = query.SortByPriority
	out.MultiSelectMode = true
	out.Selected = map[taskref.TaskIdentity]bool{{ProjectID: "project", TaskID: "one"}: true}
	out.ToggleQueryReadout()

	if got := out.QueryReadoutRows(); len(got) != 0 {
		t.Fatalf("staged stack has %d peelable rows, want an empty one: %+v", len(got), got)
	}
	if got := readoutBodyText(out, 60); !strings.Contains(got, "below") {
		t.Fatalf("the staged readout fits its window, so there is nothing to scroll:\n%s", got)
	}

	guidance, ok := out.ContextualGuidance()
	if !ok {
		t.Fatal("the open readout produced no guidance")
	}
	labels := guidanceHintLabels(guidance)
	if !strings.Contains(labels, "j/k scroll") {
		t.Fatalf("the scrollable readout's footer omits the scroll keys: %q", labels)
	}
	for _, dead := range []string{"peel one", "peel all"} {
		if strings.Contains(labels, dead) {
			t.Errorf("the empty stack's footer names %q, which can take nothing away: %q", dead, labels)
		}
	}
}

// TestQueryReadoutPeelReloadsThePopulationThroughTheShell keeps the one facet
// class that decides which projects load honest: peeling it asks the shell to
// reload rather than rebuilding rows over a snapshot the peel invalidated.
func TestQueryReadoutPeelReloadsThePopulationThroughTheShell(t *testing.T) {
	app := readoutTestApp(t)
	out := app.crossOutliner
	out.ProjectFilter.ArchivedMode = query.ProjectPopulationInclude
	adoptFilterCarriers(out)
	out.ToggleQueryReadout()

	rows := out.QueryReadoutRows()
	if len(rows) != 1 || rows[0].Label != "population" {
		t.Fatalf("rows = %+v, want the one population row", rows)
	}
	cmd := out.PeelQueryReadoutRow(0)
	if cmd == nil {
		t.Fatal("peeling a population facet raised no reload")
	}
	msg, ok := cmd().(outliner.QueryReadoutReloadRequestMsg)
	if !ok {
		t.Fatalf("population peel raised %T, want a reload request", msg)
	}
	if out.ProjectFilter.ArchivedMode == query.ProjectPopulationInclude {
		t.Error("the peel did not write the population fields before asking for the reload")
	}
	if handled, reload := app.handleOperationOverlayMessage(msg); !handled || reload == nil {
		t.Fatalf("the shell = handled %v cmd %T, want the population reloaded", handled, reload)
	}
}

// TestQueryReadoutPeelReseatsAgainstTheStripItWillDraw is the peel's re-seat
// measured against the frame the peel produces rather than the one it passes
// through. The readout's window is what the status strip leaves, and the strip is
// a row taller while it holds a message — so a re-seat taken between the axis
// write and the status the peel publishes clamps the offset against a window the
// frame never draws.
//
// That is a one-way loss: the render's own clamp can only reduce an offset
// further, so the reader's window ends at the top of a document that still
// overflows and every landmark they had scrolled to is gone. The assertion is
// therefore the whole of it — the peel leaves the window at the end of the
// shortened document, which is where it already was.
func TestQueryReadoutPeelReseatsAgainstTheStripItWillDraw(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	out := overflowingReadoutApp(t).crossOutliner
	out.HandleQueryReadoutKey("end")
	if above, below := readoutWindowPosition(t, out); above == 0 || below != 0 {
		t.Fatalf("End left %d above · %d below, so this document cannot be scrolled "+
			"away from the top and the peel below proves nothing", above, below)
	}

	// Peeling the mode row is the case that used to lose the window: it routes
	// through SelectOutlinerMode, which clears the status on its way past.
	before, _ := readoutWindowPosition(t, out)
	out.HandleQueryReadoutKey("1")
	above, below := readoutWindowPosition(t, out)
	if below != 0 {
		t.Fatalf("the peel left %d above · %d below, so the window was re-seated "+
			"against a strip this frame does not draw:\n%s",
			above, below, readoutBodyText(out, readoutResizeWidth))
	}
	// The window is not asserted to move by exactly one row, because the status
	// this peel publishes takes a strip row of its own: the document loses a line
	// and the window loses a line with it. What is asserted is that the reader
	// kept a scrolled window at all rather than being sent back to row one.
	if above == 0 {
		t.Fatalf("the peel sent the window to the top of a document with %d rows "+
			"below it, from %d above:\n%s", below, before,
			readoutBodyText(out, readoutResizeWidth))
	}
	if !strings.HasPrefix(out.StatusMessage, "Peeled ") {
		t.Fatalf("the peel published no status (%q), so the strip height it "+
			"re-seated against was never in question", out.StatusMessage)
	}
}
