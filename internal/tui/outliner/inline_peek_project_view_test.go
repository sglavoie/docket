package outliner

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// projectPeekOutliner puts a cursor on a project header with Peek open. Scoping
// picks the header form: an empty scope folds the cross-project groups, whose
// summaries are the only project rows All draws now that an expanded group
// spends no row on a heading, and a scope produces the one focused header
// session 6 added.
func projectPeekOutliner(t *testing.T, scope string, width, height int, projects ...*models.Project) *OutlinerModel {
	t.Helper()
	outliner := scopeTestOutliner(projects...)
	outliner.SetSize(width, height)
	if scope != "" {
		outliner.PushProjectScope(scope)
	}
	outliner.RebuildRows()
	if scope == "" {
		for _, group := range outliner.VisibleProjectGroups() {
			outliner.SetProjectGroupFolded(group, true)
		}
		outliner.RebuildRows()
	}
	if !moveCursorToProjectHeader(outliner, projects[0].ID, 0) {
		t.Fatalf("no project header for %q", projects[0].ID)
	}
	outliner.PeekMode = true
	// LastViewport has to hold a row width before the scrolling keys can read one,
	// exactly as it does when a key arrives in production.
	outliner.View()
	return outliner
}

// moveCursorToProjectHeader parks the cursor on the occurrence'th header naming
// a project. All emits one per state section, so the index is what lets a test
// name a second header for the same project.
func moveCursorToProjectHeader(m *OutlinerModel, projectID string, occurrence int) bool {
	seen := 0
	for index, row := range m.Rows {
		if !row.IsProjectHeader || row.ProjectID != projectID {
			continue
		}
		if seen == occurrence {
			m.Cursor = index
			return true
		}
		seen++
	}
	return false
}

// TestProjectPeekOpensOnBothHeaderForms is the acceptance claim, made through
// the model rather than on a document: the cross-project group header and the
// focused-project header are both rows that own a project, so both open, page,
// and reach the end of the same detail.
func TestProjectPeekOpensOnBothHeaderForms(t *testing.T) {
	for _, test := range []struct {
		name  string
		scope string
	}{
		{name: "cross-project group header"},
		{name: "focused project header", scope: "alpha"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outliner := projectPeekOutliner(t, test.scope, 80, 24, peekCommentedProject("alpha"), peekCommentedProject("beta"))
			header := outliner.Rows[outliner.Cursor]
			if !header.IsProjectHeader || header.ProjectID != "alpha" {
				t.Fatalf("cursor row = %+v, want alpha's header", header)
			}
			if test.scope != "" && !header.IsFocusedProjectHeader {
				t.Fatalf("scoped cursor row = %+v, want the focused header", header)
			}
			if test.scope == "" && header.IsFocusedProjectHeader {
				t.Fatalf("unscoped cursor row = %+v, want a group header", header)
			}

			doc := cursorPeekDocument(t, outliner)
			if !strings.Contains(peekDocumentText(doc), "Newest and deliberately long") {
				t.Fatalf("the header's Peek omits alpha's newest comment:\n%s", peekDocumentText(doc))
			}
			if !doc.Paged() {
				t.Fatalf("the fixture fit one window: %d rows", len(doc.Body))
			}

			// The same four keys, over the same window implementation.
			if !outliner.HalfPagePeek(1) || peekOffset(t, outliner) != outliner.peekDockGeometry().HalfPageRows() {
				t.Fatalf("a half page moved to %d", peekOffset(t, outliner))
			}
			for outliner.PagePeek(1) { //nolint:revive // Reaching the end is the point of the loop.
			}
			if got := peekOffset(t, outliner); got != doc.MaxOffset() {
				t.Fatalf("paging stopped at %d, want the document end %d", got, doc.MaxOffset())
			}
			if outliner.PagePeek(1) || outliner.HalfPagePeek(1) {
				t.Fatal("scrolling past the end moved")
			}
			if peek := outliner.PeekCommandContext(); !peek.Visible || peek.CanPageDown || !peek.CanPageUp {
				t.Fatalf("end context = %+v, want up only", peek)
			}
		})
	}
}

// TestProjectPeekBelongsToTheProjectAndNotToTheHeader is the owner rule. All
// emits one header per state section, so a project has several rows standing for
// one detail; a page turn under any of them is the same page under the rest, and
// none of them is ever the same owner as a task.
func TestProjectPeekBelongsToTheProjectAndNotToTheHeader(t *testing.T) {
	alpha, beta := peekCommentedProject("alpha"), peekCommentedProject("beta")
	outliner := projectPeekOutliner(t, "", 80, 40, alpha, beta)

	first := outliner.Cursor
	if !moveCursorToProjectHeader(outliner, alpha.ID, 1) {
		t.Fatal("All emitted only one header for alpha")
	}
	second := outliner.Cursor
	if outliner.Rows[first].EffectiveCollapseKey() == outliner.Rows[second].EffectiveCollapseKey() {
		t.Fatal("the two headers share a fold key, so they are not distinct rows")
	}

	outliner.Cursor = first
	if !outliner.HalfPagePeek(1) {
		t.Fatal("the fixture would not scroll")
	}
	outliner.Cursor = second
	if got := peekOffset(t, outliner); got != outliner.peekDockGeometry().HalfPageRows() {
		t.Fatalf("a second header for the same project read offset %d, want the shared %d",
			got, outliner.peekDockGeometry().HalfPageRows())
	}

	// Another project reads its own top, and so does a task row, whichever
	// project it belongs to.
	if !moveCursorToProjectHeader(outliner, beta.ID, 0) {
		t.Fatal("no header for beta")
	}
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("beta inherited offset %d", got)
	}
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alpha.Tasks[0].ID})
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("a task inherited its project's offset %d", got)
	}
	outliner.Cursor = second
	if got := peekOffset(t, outliner); got != outliner.peekDockGeometry().HalfPageRows() {
		t.Fatalf("returning to the header lost its offset: %d", got)
	}

	// The two populations cannot collide even when their identity strings agree.
	if projectPeekOwner(alpha.ID).sameAs(taskPeekOwner(taskref.TaskIdentity{ProjectID: alpha.ID})) {
		t.Fatal("a project owner matched a task owner")
	}
	if projectPeekOwner("").sameAs(projectPeekOwner("")) {
		t.Fatal("an anonymous header agreed with another anonymous header")
	}
}

// TestProjectPeekOffsetSurvivesReloadAndFollowsScope is the session-only
// offset's remaining lifetime on this document kind: a reload rebuilds the rows
// and the pointers behind them without changing what the project says, and the
// two header forms are two rows over one project, so neither is a reason to lose
// a reading position. A rename also preserves a passage in another section.
func TestProjectPeekOffsetSurvivesReloadAndFollowsScope(t *testing.T) {
	alpha := peekCommentedProject("alpha")
	outliner := projectPeekOutliner(t, "", 80, 40, alpha, peekCommentedProject("beta"))
	if !outliner.HalfPagePeek(1) {
		t.Fatal("the fixture would not scroll")
	}

	outliner.View()
	if got := peekOffset(t, outliner); got != outliner.peekDockGeometry().HalfPageRows() {
		t.Fatalf("a redraw reset the offset to %d", got)
	}

	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if !moveCursorToProjectHeader(outliner, alpha.ID, 0) {
		t.Fatal("the reload dropped alpha's header")
	}
	if got := peekOffset(t, outliner); got != outliner.peekDockGeometry().HalfPageRows() {
		t.Fatalf("a reload reset the offset to %d", got)
	}

	// Scoping swaps the group header for the focused one over the same project,
	// which is the same detail and therefore the same window.
	outliner.PushProjectScope(alpha.ID)
	outliner.RebuildRows()
	if !moveCursorToProjectHeader(outliner, alpha.ID, 0) {
		t.Fatal("the scoped projection has no header for alpha")
	}
	if !outliner.Rows[outliner.Cursor].IsFocusedProjectHeader {
		t.Fatalf("scoped header = %+v, want the focused form", outliner.Rows[outliner.Cursor])
	}
	outliner.View()
	if got := peekOffset(t, outliner); got != outliner.peekDockGeometry().HalfPageRows() {
		t.Fatalf("changing header form reset the offset to %d", got)
	}

	// Renaming does not lose the passage in the description.
	alpha.Name = "Renamed Platform"
	if got := peekOffset(t, outliner); got != outliner.peekDockGeometry().HalfPageRows() {
		t.Fatalf("a rename lost offset: %d", got)
	}
}

// TestProjectPeekIsReservedByTheViewport is the geometry claim. The window
// authority subtracts exactly what the renderer draws, plus the one guard row it
// already reserved for a task Peek — at the top of the list and with the cursor
// pushed to the bottom of the viewport, over the collapsed summaries that are
// the only project rows a cross-project projection draws.
func TestProjectPeekIsReservedByTheViewport(t *testing.T) {
	for _, test := range []struct {
		name       string
		scope      string
		occurrence int
	}{
		{name: "first group header"},
		{name: "later group header", occurrence: 1},
		{name: "focused project header", scope: "alpha"},
	} {
		t.Run(test.name, func(t *testing.T) {
			alpha, beta := peekCommentedProject("alpha"), peekCommentedProject("beta")
			outliner := projectPeekOutliner(t, test.scope, 80, 18, alpha, beta)
			if !moveCursorToProjectHeader(outliner, alpha.ID, test.occurrence) {
				t.Fatalf("no header %d for alpha", test.occurrence)
			}
			header := outliner.Rows[outliner.Cursor]

			outliner.PeekMode = false
			plain := outliner.ComputeOutlinerViewport(80, 0).VisibleRows
			outliner.PeekMode = true
			peeked := outliner.ComputeOutlinerViewport(80, 0).VisibleRows
			drawn := len(outliner.PeekDockLines())
			dockRows := outliner.PeekDockRows()
			if drawn != dockRows {
				t.Fatalf("the header's dock drew %d rows, want %d", drawn, dockRows)
			}
			if len(outliner.PeekDocumentBodyForRow(header)) == 0 {
				t.Fatal("the header owns no document for the dock to draw")
			}
			if want := dockRows + peekDockFooterWrapRows; plain-peeked != want {
				t.Fatalf("reserved %d rows for the dock, want %d", plain-peeked, want)
			}

			// The whole frame still fits, which is what the reservation buys.
			outliner.ViewScrollOffset = 0
			assertProjectPeekFrameFits(t, outliner, 80, 18, test.name+" at the top")

			// It fits with the header on the last visible row too, which is where an
			// unreserved Peek would hang off the bottom of the terminal. The last
			// header in the projection is the one deep enough for the window to be
			// scrolled past it; a focused projection leads with its single header, so
			// there is no such row and nothing to assert.
			deepest := lastProjectHeaderIndex(outliner)
			if deepest < peeked {
				return
			}
			outliner.Cursor = deepest
			outliner.ViewScrollOffset = deepest - peeked + 1
			assertProjectPeekFrameFits(t, outliner, 80, 18, test.name+" at the bottom")
			if got := outliner.LastViewport.endIdx; got != outliner.Cursor+1 {
				t.Fatalf("%s window ends at %d, want the header row %d last",
					test.name, got, outliner.Cursor+1)
			}
		})
	}
}

func lastProjectHeaderIndex(m *OutlinerModel) int {
	last := -1
	for index, row := range m.Rows {
		if row.IsProjectHeader {
			last = index
		}
	}
	return last
}

func assertProjectPeekFrameFits(t *testing.T, m *OutlinerModel, width, height int, label string) {
	t.Helper()
	content := m.View().Content
	if got := lipgloss.Height(content); got > height {
		t.Fatalf("%s view height = %d, want <= %d", label, got, height)
	}
	for number, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("%s line %d = %d columns, want <= %d", label, number, got, width)
		}
	}
}

// TestProjectPeekKeepsOneMeasurementAndOneRender extends the task Peek's
// invariant to the second document kind, and for the same reason: the height the
// viewport reserved and the height the renderer drew come from one document, so
// they must be one number at every stop of every window. Both themes, both
// header forms, and a narrow terminal are swept because a palette, a focused
// header's different row content, and a width that forces every section to wrap
// are each a way for the two to drift apart.
func TestProjectPeekKeepsOneMeasurementAndOneRender(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
		for _, scope := range []string{"", "alpha"} {
			for _, size := range []struct{ width, height int }{{40, 12}, {60, 20}, {80, 24}, {120, 40}} {
				label := fmt.Sprintf("%s scope=%q %dx%d", themeName, scope, size.width, size.height)
				outliner := projectPeekOutliner(t, scope, size.width, size.height,
					peekCommentedProject("alpha"), peekCommentedProject("beta"))
				for step := 0; ; step++ {
					width := size.width
					drawn := outliner.PeekDockLines()
					if got := outliner.PeekDockRows(); got != len(drawn) {
						t.Fatalf("%s step %d measured %d rows and drew %d", label, step, got, len(drawn))
					}
					for number, line := range drawn {
						for kind, measured := range map[string]int{
							"styled":   lipgloss.Width(line),
							"no-color": lipgloss.Width(ansi.Strip(line)),
						} {
							if measured > width {
								t.Fatalf("%s step %d row %d %s = %d columns, want <= %d",
									label, step, number, kind, measured, width)
							}
						}
					}
					if step%2 == 0 {
						if !outliner.HalfPagePeek(1) {
							break
						}
						continue
					}
					if !outliner.PagePeek(1) {
						break
					}
				}
				assertProjectPeekFrameFits(t, outliner, size.width, size.height, label)
			}
		}
	}
	theme.ApplyTheme(theme.ThemeConfig{})
}
