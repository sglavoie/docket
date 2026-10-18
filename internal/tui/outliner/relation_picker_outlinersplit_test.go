package outliner

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

func TestRelationPickerSearchEditsInPlaceAndScrolls(t *testing.T) {
	alpha, beta := relationFixture(t)
	view := scopeTestOutliner(alpha, beta)
	relationPickerOver(t, view, "proj-a", "source")
	for _, key := range []tea.KeyPressMsg{
		{Code: '/', Text: "/"},
		{Code: 'c', Text: "contrct"},
		{Code: tea.KeyLeft},
		{Code: tea.KeyLeft},
		{Code: 'a', Text: "a"},
	} {
		view.HandleKeyMsg(key)
	}
	if got := view.RelationPicker.Query; got != "contract" || len(view.RelationPicker.Surface.Matches()) != 2 {
		t.Fatalf("search = %q, rows = %v", got, relationRowLabels(view))
	}
	guidance := ansi.Strip(chrome.RenderGuidanceText(view.relationPickerGuidance(), 160))
	if strings.Contains(guidance, "j/k") || !strings.Contains(guidance, "↑/↓") {
		t.Fatalf("search guidance = %q", guidance)
	}
	view.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnd})
	view.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: strings.Repeat("界", 50) + "TAIL"})
	for _, width := range []int{100, 60, 80} {
		popup := view.renderRelationPicker(width)
		if lipgloss.Width(popup) > width || !strings.Contains(ansi.Strip(popup), "TAIL") {
			t.Fatalf("width %d lost the query tail or overflowed: %s", width, popup)
		}
	}
}

// relationPickerOver opens the picker over one loaded task.
func relationPickerOver(t *testing.T, outlinerView *OutlinerModel, projectID, taskID string) {
	t.Helper()
	for _, project := range outlinerView.Population.Projects {
		if project.ID != projectID {
			continue
		}
		task := project.GetTaskByID(taskID)
		if task == nil {
			t.Fatalf("project %q holds no task %q", projectID, taskID)
		}
		if err := outlinerView.OpenRelationPicker(task, project); err != nil {
			t.Fatalf("OpenRelationPicker(%q): %v", taskID, err)
		}
		return
	}
	t.Fatalf("population holds no project %q", projectID)
}

// relationRowLabels reads the picker's list as headers and primary text, in
// render order, so an assertion names what a reader sees.
func relationRowLabels(outlinerView *OutlinerModel) []string {
	labels := make([]string, 0, 8)
	for _, row := range outlinerView.RelationPicker.Surface.Rows() {
		if row.Kind == listsurface.ListRowCategory {
			labels = append(labels, "["+row.Category.Label+"]")
			continue
		}
		labels = append(labels, row.Match.Item.PrimaryText)
	}
	return labels
}

func TestRelationPickerListsBothDirectionsInAuthoritativeOrder(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	relationPickerOver(t, outlinerView, "proj-a", "source")

	got := relationRowLabels(outlinerView)
	want := []string{
		"[" + relationSectionOutgoingLabel + "]",
		"Alpha #2 Agree the schema",
		"Beta #1 Publish the contract",
		"proj-z:ghost",
		"[" + relationSectionIncomingLabel + "]",
		"Beta #1 Publish the contract",
		"Beta #2 Retire the shim",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("picker rows =\n%v\nwant\n%v", got, want)
	}

	// The dependent header names its own limit, because the list is computed over
	// the loaded population and can never be the whole truth.
	if !strings.Contains(relationSectionIncomingLabel, "loaded projects only") {
		t.Fatalf("the dependents header does not name its limit: %q", relationSectionIncomingLabel)
	}
}

// TestRelationPickerKeepsReciprocalRowsDistinct is the identity rule. Two tasks
// naming each other produce the same locator in both directions, so direction
// has to be part of the row ID or the surface would hold one row where the user
// is owed two.
func TestRelationPickerKeepsReciprocalRowsDistinct(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	relationPickerOver(t, outlinerView, "proj-a", "source")

	seen := make(map[listsurface.ListItemID]int)
	reciprocal := 0
	for _, item := range outlinerView.RelationPicker.Surface.Items() {
		seen[item.ID]++
		candidate, ok := item.Data.(relationCandidate)
		if !ok {
			t.Fatalf("row %q carries no candidate", item.ID)
		}
		if candidate.Reference == (models.TaskReference{ProjectID: "proj-b", TaskID: "remote"}) ||
			(candidate.Direction == relationIncoming && candidate.Target.TaskID == "remote") {
			reciprocal++
		}
	}
	for id, count := range seen {
		if count > 1 {
			t.Fatalf("row ID %q appears %d times", id, count)
		}
	}
	if reciprocal != 2 {
		t.Fatalf("the reciprocal pair produced %d rows, want one per direction", reciprocal)
	}
}

// TestRelationPickerRetainsUnavailableLocatorAndRefusesIt covers the disabled
// half of the contract: the stored locator stays listed because it is what
// storage holds, and Enter on it explains rather than moves.
func TestRelationPickerRetainsUnavailableLocatorAndRefusesIt(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	relationPickerOver(t, outlinerView, "proj-a", "source")

	before := outlinerView.CaptureOutlinerPresentationCheckpoint()
	if !outlinerView.RelationPicker.Surface.SetCursorToID(
		listsurface.ListItemID(relationPickerItemPrefix + "out:proj-z:ghost"),
	) {
		t.Fatal("the unavailable locator is not listed")
	}
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !outlinerView.RelationPickerOpen() {
		t.Fatal("a refused row closed the picker")
	}
	if outlinerView.RelationPicker.Notice != relationUnavailableReason {
		t.Fatalf("notice = %q, want %q", outlinerView.RelationPicker.Notice, relationUnavailableReason)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a refused row published a return frame")
	}
	if got := outlinerView.CaptureOutlinerPresentationCheckpoint(); got.Projection.Mode != before.Projection.Mode ||
		got.scope.projectID != before.scope.projectID {
		t.Fatal("a refused row moved the view")
	}
}

// TestRelationPickerRefusesATargetThatWentAway is the revalidation rule: the
// row was built from an earlier reading, so Enter resolves the reference again
// and refuses in place when the answer changed.
func TestRelationPickerRefusesATargetThatWentAway(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	relationPickerOver(t, outlinerView, "proj-a", "source")
	if !outlinerView.RelationPicker.Surface.SetCursorToID(
		listsurface.ListItemID(relationPickerItemPrefix + "out:proj-b:remote"),
	) {
		t.Fatal("the cross-project target is not listed")
	}

	// The task disappears underneath the open picker.
	beta.Tasks = []*models.Task{beta.GetTaskByID("closed")}
	beta.BuildTaskTree()
	outlinerView.invalidateDependencyIndex()

	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !outlinerView.RelationPickerOpen() {
		t.Fatal("a stale target closed the picker")
	}
	if outlinerView.RelationPicker.Notice != relationStaleTargetRefusal {
		t.Fatalf("notice = %q, want %q", outlinerView.RelationPicker.Notice, relationStaleTargetRefusal)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a stale target published a return frame")
	}
}

// TestRelationPickerSearchNarrowsWithoutReordering pins the reason the picker
// filters its own rows: the two sections keep their authoritative orders, and
// only the matching rows survive.
func TestRelationPickerSearchNarrowsWithoutReordering(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	relationPickerOver(t, outlinerView, "proj-a", "source")

	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !outlinerView.RelationPicker.Searching {
		t.Fatal("`/` did not open the picker search")
	}
	for _, r := range "contract" {
		outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	got := relationRowLabels(outlinerView)
	want := []string{
		"[" + relationSectionOutgoingLabel + "]",
		"Beta #1 Publish the contract",
		"[" + relationSectionIncomingLabel + "]",
		"Beta #1 Publish the contract",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("narrowed rows =\n%v\nwant\n%v", got, want)
	}

	// A query nothing matches says so, rather than claiming the task has no
	// relationships: the picker holds the query the surface never sees, so it is
	// the picker that has to tell the two states apart.
	for range "contract" {
		outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	for _, r := range "zzzz" {
		outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	empty, ok := outlinerView.RelationPicker.Surface.EmptyContext()
	if !ok || empty.State.Title != relationNoMatchTitle {
		t.Fatalf("empty copy = %+v, want the no-match state", empty)
	}

	// Esc clears the query, the next leaves search, and only browse mode closes.
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	if outlinerView.RelationPicker.Query != "" || !outlinerView.RelationPicker.Searching {
		t.Fatalf("the first Esc left query=%q searching=%v",
			outlinerView.RelationPicker.Query, outlinerView.RelationPicker.Searching)
	}
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	if outlinerView.RelationPicker.Searching || !outlinerView.RelationPickerOpen() {
		t.Fatal("the second Esc did not leave search with the picker open")
	}
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	if outlinerView.RelationPickerOpen() {
		t.Fatal("the third Esc did not close the picker")
	}
}

// TestRelationPickerOwnsInputWhileOpen is the isolation rule. The Outliner's own
// grammar is refused whole while the picker is up, and a printable key is text
// while its search is editing rather than a movement.
func TestRelationPickerOwnsInputWhileOpen(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	before := outlinerView.PrimaryMode
	relationPickerOver(t, outlinerView, "proj-a", "source")

	if !outlinerView.SimplifiedInlineControlOwnsInput() {
		t.Fatal("the picker does not own the simplified input")
	}
	if !outlinerView.ShouldCaptureEsc() {
		t.Fatal("the picker does not capture Esc")
	}

	// `1` is the All jump in the ordinary grammar and nothing at all here.
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: '1', Text: "1"})
	if outlinerView.PrimaryMode != before {
		t.Fatalf("a picker key reached the mode grammar: %s", outlinerView.PrimaryMode)
	}

	// `j` moves in browse mode and types in search mode.
	outlinerView.RelationPicker.Surface.MoveCursorToStart()
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: 'j', Text: "j"})
	moved := outlinerView.RelationPicker.Surface.Cursor()
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: '/', Text: "/"})
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if outlinerView.RelationPicker.Query != "j" {
		t.Fatalf("query = %q, want the typed rune", outlinerView.RelationPicker.Query)
	}
	if outlinerView.RelationPicker.Surface.Cursor() != 0 && moved == 0 {
		t.Fatal("browse mode did not move the cursor")
	}

	// The footer while the picker owns input describes the picker, not the list
	// underneath it.
	guidance, ok := outlinerView.ContextualGuidance()
	labels := make([]string, 0, len(guidance.Hints))
	for _, hint := range guidance.Hints {
		labels = append(labels, hint.Key+" "+hint.Label)
	}
	if !ok || !strings.Contains(strings.Join(labels, " · "), "search") {
		t.Fatalf("contextual guidance did not describe the picker: %v", labels)
	}
}

// TestRelationPickerOpensOverNoRelationships keeps the key answerable on a task
// with nothing to show: the empty state is the answer, and refusing the key
// would leave it unreachable.
func TestRelationPickerOpensOverNoRelationships(t *testing.T) {
	lonely := renderFixtureTask(t, "lonely", 1, "Nothing points here", models.StateBacklog, 0, models.StateBacklog)
	project := renderFixtureProject(t, "proj-a", "alpha", "Alpha", []*models.Task{lonely})
	outlinerView := scopeTestOutliner(project)
	relationPickerOver(t, outlinerView, "proj-a", "lonely")

	if !outlinerView.RelationPickerOpen() {
		t.Fatal("the picker refused a task with no relationships")
	}
	empty, ok := outlinerView.RelationPicker.Surface.EmptyContext()
	if !ok || empty.State.Title != relationEmptyTitle {
		t.Fatalf("empty copy = %+v, want the empty state", empty)
	}
	if rendered := ansi.Strip(outlinerView.renderRelationPicker(60)); !strings.Contains(rendered, relationEmptyTitle) {
		t.Fatalf("the narrow render dropped the empty state:\n%s", rendered)
	}
}

// TestRelationPickerRendersInsideANarrowTerminal keeps the overlay within its
// budget at the smallest width the product supports.
func TestRelationPickerRendersInsideANarrowTerminal(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetSize(60, 20)
	relationPickerOver(t, outlinerView, "proj-a", "source")

	rendered := outlinerView.renderRelationPicker(60)
	if rendered == "" {
		t.Fatal("an open picker rendered nothing")
	}
	for _, line := range strings.Split(ansi.Strip(rendered), "\n") {
		if width := ansi.StringWidth(line); width > 60 {
			t.Fatalf("line %q is %d columns wide", line, width)
		}
	}
	if !strings.Contains(ansi.Strip(rendered), "Wire the importer") {
		t.Fatalf("the overlay does not name its subject:\n%s", ansi.Strip(rendered))
	}
}
