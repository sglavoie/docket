package outliner

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// peekProjectComment mints one stored comment. The instant is derived from the
// index rather than taken from the clock, so the ordering assertions below name
// a fixed sequence instead of whichever order two same-millisecond writes landed
// in.
func peekProjectComment(index int, text string) *models.ProjectComment {
	return &models.ProjectComment{
		ID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", index),
		Text:      text,
		CreatedAt: time.Date(2026, 3, 1+index, 9, 30, 0, 0, time.UTC),
	}
}

// peekCommentedProject is a project whose detail cannot fit one window: a long
// description, tags, and several comments of which one wraps. Storage order is
// oldest first, which is what the presentation reverses.
func peekCommentedProject(id string) *models.Project {
	project := focusedHeaderProject(id)
	project.Name = strings.ToUpper(id[:1]) + id[1:] + " Platform"
	project.Description = strings.Repeat("Wrapped project description content ", 6)
	project.Tags = []string{"internal", "platform"}
	project.Comments = []*models.ProjectComment{
		peekProjectComment(0, "Oldest decision about the vendor contract"),
		peekProjectComment(1, "Middle note"),
		peekProjectComment(2, strings.Repeat("Newest and deliberately long enough to wrap the pane. ", 4)),
	}
	return project
}

func peekProjectDocument(project *models.Project, width int) inlinePeekDocument {
	return inlinePeekProjectDocumentFor(inlinePeekProjectOptions{Project: project, Indent: "  ", Width: width})
}

func peekDocumentText(doc inlinePeekDocument) string {
	return ansi.Strip(strings.Join(doc.Body, "\n"))
}

// firstIndexOf is the ordering assertion's one read. It fails the test rather
// than returning a sentinel, because every caller below is naming content the
// fixture put in the document on purpose.
func firstIndexOf(t *testing.T, haystack, needle, label string) int {
	t.Helper()
	index := strings.Index(haystack, needle)
	if index < 0 {
		t.Fatalf("the document omits %s (%q):\n%s", label, needle, haystack)
	}
	return index
}

// TestProjectPeekNamesEveryStoredDetailInOrder is the document's content
// contract: the project's own name first, then description, tags, and the
// comment bodies newest first. Body order is asserted by position rather than by
// presence, because the sections are what a reader scans and a reordering is
// invisible to a membership check.
func TestProjectPeekNamesEveryStoredDetailInOrder(t *testing.T) {
	project := peekCommentedProject("alpha")
	doc := peekProjectDocument(project, 80)

	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	name := firstIndexOf(t, body, project.Name, "the project name")
	description := firstIndexOf(t, body, "Description: ", "the description label")
	if name >= description {
		t.Fatalf("the name does not lead the body: name %d, description %d:\n%s", name, description, body)
	}
	tags := firstIndexOf(t, body, "#platform", "the tag chips")
	comments := firstIndexOf(t, body, "Comments: ", "the comment label")
	if description >= tags || tags >= comments {
		t.Fatalf("body order = description %d, tags %d, comments %d:\n%s", description, tags, comments, body)
	}

	// Newest first, and every stamp drawn, which is what tells two comments of
	// the same length apart.
	newest := firstIndexOf(t, body, "Newest and deliberately long", "the newest comment")
	middle := firstIndexOf(t, body, "Middle note", "the middle comment")
	oldest := firstIndexOf(t, body, "Oldest decision", "the oldest comment")
	if newest >= middle || middle >= oldest {
		t.Fatalf("comment order = newest %d, middle %d, oldest %d:\n%s", newest, middle, oldest, body)
	}
	for _, comment := range project.Comments {
		stamp := ansi.Strip(chrome.FormatTimestampBracket(comment.CreatedAt))
		if !strings.Contains(body, stamp) {
			t.Fatalf("comment %s drew no stamp %q:\n%s", comment.ID, stamp, body)
		}
	}

	// The long comment is whole rather than elided: every word of it is reachable
	// somewhere in the document, because the page is what bounds the section.
	unwrapped := strings.Join(strings.Fields(body), " ")
	if !strings.Contains(unwrapped, strings.TrimSpace(project.Comments[2].Text)) {
		t.Fatalf("the newest comment was elided:\n%s", body)
	}
	if !doc.Paged() {
		t.Fatalf("a project with three comments fit one window: %d rows", len(doc.Body))
	}
}

// TestProjectPeekSpendsOneRowPerVisualLine holds the document to its own row
// accounting. The comment popup is the multiline TextEntry provider, so a stored
// body can carry newlines of its own; a row hiding one would count as a single
// line everywhere the height is measured and paint two on screen, which is a
// layout issue rather than a rendering choice.
func TestProjectPeekSpendsOneRowPerVisualLine(t *testing.T) {
	project := primaryRowsProject("wrapped")
	project.Name = "Wrapped"
	project.Description = "First paragraph.\n\nSecond paragraph."
	project.Comments = []*models.ProjectComment{
		peekProjectComment(0, "Opening line of the note.\n\nA blank line above this one."),
	}

	doc := peekProjectDocument(project, 80)
	for i, line := range doc.Body {
		if strings.Contains(line, "\n") {
			t.Fatalf("row %d carries a newline of its own: %q", i, ansi.Strip(line))
		}
	}
	// The blank row between the two halves is dropped rather than spent, for the
	// same reason every other Peek body drops one: five rows cannot afford it.
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"First paragraph.", "Second paragraph.", "Opening line of the note.", "A blank line above this one."} {
		if !strings.Contains(strings.Join(strings.Fields(body), " "), want) {
			t.Fatalf("the document omits %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\n\n") {
		t.Fatalf("a blank row survived into the document:\n%q", body)
	}
}

// TestProjectPeekAlwaysHasSomethingToSay covers the empty project, which is the
// state a header is most often parked on: a project created a minute ago has no
// description, no tags, and no comments, and a Peek that drew nothing there
// would read as a broken key rather than as an empty project.
func TestProjectPeekAlwaysHasSomethingToSay(t *testing.T) {
	empty := primaryRowsProject("empty")
	empty.Name = "Empty"
	doc := peekProjectDocument(empty, 80)

	if doc.Empty() {
		t.Fatal("an empty project produced no document")
	}
	if doc.Paged() {
		t.Fatalf("an empty project paged: %d rows", len(doc.Body))
	}
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	if want := "Comments: No comments yet."; !strings.Contains(strings.Join(strings.Fields(body), " "), want) {
		t.Fatalf("empty body = %q, want %q", body, want)
	}

	// A project the population no longer holds is the other end of the same
	// question, and it is the one case that draws nothing at all.
	if got := peekProjectDocument(nil, 80); !got.Empty() {
		t.Fatalf("an unresolved header produced %+v", got)
	}
	if got := peekProjectDocument(empty, 0); !got.Empty() {
		t.Fatalf("a zero-width pane produced %+v", got)
	}
}

// TestProjectPeekFingerprintReadsStoredValuesAlone is the reset rule's input
// half. The fingerprint says what the document is, so it must move when the
// project's stored detail moves and stay still for everything else — a re-wrap at
// another width, and the relative age beside every stamp, which drifts on the
// minute tick and would otherwise throw a reader back to the top mid-page.
func TestProjectPeekFingerprintReadsStoredValuesAlone(t *testing.T) {
	project := peekCommentedProject("alpha")
	base := peekProjectDocument(project, 80).Fingerprint

	if got := peekProjectDocument(project, 40).Fingerprint; got != base {
		t.Fatal("a narrower pane produced a different document")
	}
	if got := peekProjectDocument(project, 120).Fingerprint; got != base {
		t.Fatal("a wider pane produced a different document")
	}
	if got := peekProjectDocument(peekCommentedProject("alpha"), 80).Fingerprint; got != base {
		t.Fatal("rebuilding the same project produced a different document")
	}
	for _, drift := range []string{"ago", "from now"} {
		if strings.Contains(base, drift) {
			t.Fatalf("the fingerprint carries the drifting age %q: %q", drift, base)
		}
	}

	for _, test := range []struct {
		name   string
		mutate func(*models.Project)
	}{
		{name: "rename", mutate: func(p *models.Project) { p.Name = "Renamed" }},
		{name: "description", mutate: func(p *models.Project) { p.Description += " and one more sentence" }},
		{name: "tags", mutate: func(p *models.Project) { p.Tags = append(p.Tags, "parked") }},
		{name: "comment text", mutate: func(p *models.Project) { p.Comments[1].Text = "Edited" }},
		{name: "comment instant", mutate: func(p *models.Project) {
			p.Comments[1].CreatedAt = p.Comments[1].CreatedAt.Add(time.Second)
		}},
		{name: "comment added", mutate: func(p *models.Project) {
			p.Comments = append(p.Comments, peekProjectComment(3, "Newer still"))
		}},
		{name: "comment deleted", mutate: func(p *models.Project) { p.Comments = p.Comments[:2] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := peekCommentedProject("alpha")
			test.mutate(changed)
			if got := peekProjectDocument(changed, 80).Fingerprint; got == base {
				t.Fatalf("%s left the fingerprint unchanged: %q", test.name, got)
			}
		})
	}
}

// projectPeekOutliner puts a cursor on a project header with Peek open. Scoping
// picks the header form: an empty scope leaves the cross-project group headers
// All emits, and a scope produces the one focused header session 6 added.
func projectPeekOutliner(t *testing.T, scope string, width, height int, projects ...*models.Project) *OutlinerModel {
	t.Helper()
	outliner := scopeTestOutliner(projects...)
	outliner.SetSize(width, height)
	if scope != "" {
		outliner.PushProjectScope(scope)
	}
	outliner.RebuildRows()
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
			if !outliner.HalfPagePeek(1) || peekOffset(t, outliner) != inlinePeekHalfPageRows {
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
	if got := peekOffset(t, outliner); got != inlinePeekHalfPageRows {
		t.Fatalf("a second header for the same project read offset %d, want the shared %d",
			got, inlinePeekHalfPageRows)
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
	if got := peekOffset(t, outliner); got != inlinePeekHalfPageRows {
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
// a reading position. A rename is, because it is a different document.
func TestProjectPeekOffsetSurvivesReloadAndFollowsScope(t *testing.T) {
	alpha := peekCommentedProject("alpha")
	outliner := projectPeekOutliner(t, "", 80, 40, alpha, peekCommentedProject("beta"))
	if !outliner.HalfPagePeek(1) {
		t.Fatal("the fixture would not scroll")
	}

	outliner.View()
	if got := peekOffset(t, outliner); got != inlinePeekHalfPageRows {
		t.Fatalf("a redraw reset the offset to %d", got)
	}

	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if !moveCursorToProjectHeader(outliner, alpha.ID, 0) {
		t.Fatal("the reload dropped alpha's header")
	}
	if got := peekOffset(t, outliner); got != inlinePeekHalfPageRows {
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
	if got := peekOffset(t, outliner); got != inlinePeekHalfPageRows {
		t.Fatalf("changing header form reset the offset to %d", got)
	}

	// Renaming is a different document, so the window returns to the top.
	alpha.Name = "Renamed Platform"
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("a rename kept offset %d", got)
	}
}

// TestProjectPeekIsReservedByTheViewport is the geometry claim. The window
// authority subtracts exactly what the renderer draws, plus the one guard row it
// already reserved for a task Peek — at the top of the list, with the cursor
// pushed to the bottom of the viewport, and over a collapsed group, where the
// header is the only row its project has left.
func TestProjectPeekIsReservedByTheViewport(t *testing.T) {
	for _, test := range []struct {
		name       string
		scope      string
		occurrence int
		collapse   bool
	}{
		{name: "first group header"},
		{name: "later group header", occurrence: 1},
		{name: "collapsed group header", collapse: true},
		{name: "focused project header", scope: "alpha"},
	} {
		t.Run(test.name, func(t *testing.T) {
			alpha, beta := peekCommentedProject("alpha"), peekCommentedProject("beta")
			outliner := projectPeekOutliner(t, test.scope, 80, 18, alpha, beta)
			if !moveCursorToProjectHeader(outliner, alpha.ID, test.occurrence) {
				t.Fatalf("no header %d for alpha", test.occurrence)
			}
			if test.collapse {
				outliner.SetSectionRowCollapsed(outliner.Rows[outliner.Cursor], true)
				outliner.RebuildRows()
				if !moveCursorToProjectHeader(outliner, alpha.ID, test.occurrence) {
					t.Fatal("collapsing dropped alpha's header")
				}
			}
			header := outliner.Rows[outliner.Cursor]

			outliner.PeekMode = false
			plain := outliner.ComputeOutlinerViewport(80, 0).VisibleRows
			outliner.PeekMode = true
			peeked := outliner.ComputeOutlinerViewport(80, 0).VisibleRows
			drawn := outliner.inlinePeekLineCount(header, 79)
			if drawn == 0 {
				t.Fatal("the header drew no Peek to reserve")
			}
			if got := plain - peeked; got != drawn+1 {
				t.Fatalf("reserved %d rows for a %d-row Peek, want %d", got, drawn, drawn+1)
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
				row := outliner.Rows[outliner.Cursor]

				for step := 0; ; step++ {
					width := outliner.peekRenderWidth()
					drawn := outliner.InlinePeekLinesForRow(row, inlinePeekIndentForRow(row), width)
					if got := outliner.inlinePeekLineCount(row, size.width-1); got != len(drawn) {
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

// TestTaskPeekKeepsItsCommentCount is the boundary this session did not cross.
// A project header shows comment bodies because nothing else on the Outliner
// does; a task's comments have the `;` panel standing behind their count, so the
// task document still counts them and names none.
func TestTaskPeekKeepsItsCommentCount(t *testing.T) {
	task := pagedPeekTask()
	task.Comments = []*models.Comment{{Text: "First decision"}, {Text: "Second decision"}}
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 80})

	body := strings.Join(strings.Fields(ansi.Strip(strings.Join(doc.Body, "\n"))), " ")
	if !strings.Contains(body, "Comments: 2") {
		t.Fatalf("task Peek dropped its comment count:\n%s", body)
	}
	for _, comment := range task.Comments {
		if strings.Contains(body, comment.Text) {
			t.Fatalf("task Peek named comment body %q:\n%s", comment.Text, body)
		}
	}
}
