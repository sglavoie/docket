package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// Session 5 (Today Cockpit Layout) locks down the integration between the
// Next Action strip (session 4) and the existing Today task list. The wiring
// itself already lives in FocusBoardModel.View(); these tests enforce the
// layout contract:
//
//  1. The recommendation appears above the task list, not below or beside it.
//  2. The selected row and the strip are visually distinct (different style
//     signature: selected row uses a colored background, the strip uses bold
//     foreground with no background).
//  3. The strip is shown only on the Today tab in list mode, suppressed on
//     Next/Closed/Tunnel/Zen, and suppressed below NextActionStripMinWidth.
//  4. Existing add-mode, quick-edit mode, and queue/daily-plan toggles keep
//     working with the strip present.

// cockpitWorkOnTaskFixture builds a single-row Today plan with a deterministic
// task title the engine will select as the WorkOnTask recommendation. The
// task is overdue so the engine surfaces the urgent-actionable path with a
// clear "Next: <title>" string in the strip — the same fixture shape used by
// the baseline cockpit tests in baseline_cockpit_test.go.
func cockpitWorkOnTaskFixture(t *testing.T, title string) *FocusBoardModel {
	t.Helper()
	project := cockpitProject(t, "Cockpit Layout")
	task := cockpitTask(t, project, title, models.StatusTodo, models.SectionNext)
	pastDue := time.Now().Add(-48 * time.Hour)
	task.DueDate = &pastDue
	row := cockpitBoardRow(task, project)

	dailyPlanIDs := map[string]bool{task.ID: true}
	openLoops := OpenLoops{TodayTotal: 1, HasTasks: true}
	m := cockpitModel([]boardRow{row}, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
	m.SetSize(120, 30)
	return m
}

func cockpitRichPlannedFixture(t *testing.T, title string) (*FocusBoardModel, *models.Task) {
	t.Helper()
	project := cockpitProject(t, "Cockpit Hierarchy")
	task := cockpitTask(t, project, title, models.StatusTodo, models.SectionNext)
	task.Description = "Metadata should sit below the selected row."
	task.Checklist = []models.ChecklistItem{
		{Text: "draft", Done: true},
		{Text: "ship", Done: false},
	}
	tomorrow := time.Now().AddDate(0, 0, 1)
	task.DueDate = &tomorrow
	row := cockpitBoardRow(task, project)

	dailyPlanIDs := map[string]bool{task.ID: true}
	openLoops := OpenLoops{TodayTotal: 1, HasTasks: true}
	m := cockpitModel([]boardRow{row}, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
	m.cursor = 0
	m.applyFilter()
	return m, task
}

// TestFocusBoardCockpitHierarchyAcrossWidths locks the Session 3 visual order:
// title/progress, Next Action strip, Today list row, selected metadata strip,
// then footer. It exercises narrow, normal, and wide widths with a rich selected
// task so the metadata strip is part of the layout budget.
func TestFocusBoardCockpitHierarchyAcrossWidths(t *testing.T) {
	const title = "Ship cockpit plan"
	sizes := []baselineSize{
		{"narrow", 48, 18},
		{"normal", 100, 24},
		{"wide", 160, 32},
	}

	for _, sz := range sizes {
		t.Run(sz.label, func(t *testing.T) {
			m, task := cockpitRichPlannedFixture(t, title)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			plain := ansi.Strip(rendered)
			label := "cockpit hierarchy " + sz.label

			assertRenderedLinesWithinVisualWidth(t, label, rendered, sz.width)
			assertNoFooterContentOverlap(t, label, rendered, sz.height)

			titleIdx := strings.Index(plain, "Today")
			if titleIdx < 0 {
				t.Fatalf("%s: title/progress band missing:\n%s", label, plain)
			}
			stripIdx := strings.Index(plain, "Next:")
			if stripIdx < 0 {
				t.Fatalf("%s: Next Action strip missing:\n%s", label, plain)
			}
			rowIdxRel := strings.Index(plain[stripIdx+len("Next:"):], "1.")
			if rowIdxRel < 0 {
				t.Fatalf("%s: Today list row missing below strip:\n%s", label, plain)
			}
			rowIdx := stripIdx + len("Next:") + rowIdxRel
			metadataCue := ansi.Strip(renderDescriptionIndicator(task))
			metadataIdx := strings.LastIndex(plain, metadataCue)
			if metadataIdx < 0 {
				t.Fatalf("%s: selected metadata strip missing cue %q:\n%s", label, metadataCue, plain)
			}
			footerIdx := strings.LastIndex(plain, "x done")
			if footerIdx < 0 {
				t.Fatalf("%s: footer hint missing:\n%s", label, plain)
			}

			if titleIdx >= stripIdx || stripIdx >= rowIdx || rowIdx >= metadataIdx || metadataIdx >= footerIdx {
				t.Fatalf("%s: hierarchy order violated: title=%d strip=%d row=%d metadata=%d footer=%d\n%s",
					label, titleIdx, stripIdx, rowIdx, metadataIdx, footerIdx, plain)
			}
		})
	}
}

func TestFocusBoardCockpitLayoutCountsActualMetadataRows(t *testing.T) {
	layout := focusBoardCockpitLayout{
		topChrome: "title\nstrip\n\n",
		footer:    "divider\nfooter",
	}
	withoutMetadata := layout.listHeight(12)
	layout.selectedMetadata = "metadata"
	withMetadata := layout.listHeight(12)

	if withoutMetadata != withMetadata+1 {
		t.Fatalf("metadata strip should reserve exactly one list row: without=%d with=%d", withoutMetadata, withMetadata)
	}
	layout.selectedMetadata = ""
	if got := layout.listHeight(12); got != withoutMetadata {
		t.Fatalf("empty metadata strip should not reserve a row: got=%d want=%d", got, withoutMetadata)
	}
}

func TestTodayCockpitEscalationHintCountsAgainstListHeight(t *testing.T) {
	project := cockpitProject(t, "Escalation QA")
	task := cockpitTask(t, project, "Ship tight priority render", models.StatusTodo, models.SectionNext)
	task.Priority = 3
	dueSoon := time.Now().Add(24 * time.Hour)
	task.DueDate = &dueSoon
	row := cockpitBoardRow(task, project)
	dailyPlanIDs := map[string]bool{task.ID: true}
	m := cockpitModel([]boardRow{row}, nil, nil, dailyPlanIDs, OpenLoops{TodayTotal: 1, HasTasks: true}, boardTabToday)
	m.SetSize(80, 10)

	rendered := m.View().Content
	plain := ansi.Strip(rendered)

	assertRenderedLinesWithinVisualWidth(t, "today escalation compact height", rendered, 80)
	assertNoFooterContentOverlap(t, "today escalation compact height", rendered, 10)
	if !strings.Contains(plain, "Ship tight priority render") {
		t.Fatalf("selected task row missing in compact render:\n%s", plain)
	}
	if strings.Contains(plain, "Due soon") {
		t.Fatalf("secondary escalation hint should be clipped before the footer at height 10:\n%s", plain)
	}
	if !strings.Contains(plain, "x done") {
		t.Fatalf("footer should remain visible in compact render:\n%s", plain)
	}
}

func TestTodayCockpitCompactVisualStatesRespectWidthAndFooter(t *testing.T) {
	unicodeProject := cockpitProject(t, "QA 界面")
	unicodeTask := cockpitTask(t, unicodeProject, "Fix 宽度 regression ✅", models.StatusTodo, models.SectionNext)
	unicodeTask.Description = "Metadata strip should stay inside compact width."
	unicodeTask.Checklist = []models.ChecklistItem{
		{Text: "audit", Done: true},
		{Text: "ship", Done: false},
	}
	unicodeTask.Tags = []string{"国际化", "qa"}
	unicodeTask.Priority = 2
	tomorrow := time.Now().AddDate(0, 0, 1)
	unicodeTask.DueDate = &tomorrow
	unicodeRow := cockpitBoardRow(unicodeTask, unicodeProject)
	unicodeModel := cockpitModel(
		[]boardRow{unicodeRow},
		nil,
		nil,
		map[string]bool{unicodeTask.ID: true},
		OpenLoops{TodayTotal: 1, HasTasks: true},
		boardTabToday,
	)
	unicodeModel.statusMessage = "Saved 修复"

	doneProject := cockpitProject(t, "Done QA")
	doneTask := cockpitTask(t, doneProject, "Wrap up cockpit QA", models.StatusDone, models.SectionCompleted)
	doneRow := cockpitBoardRow(doneTask, doneProject)
	doneModel := cockpitModel(
		[]boardRow{doneRow},
		nil,
		nil,
		map[string]bool{doneTask.ID: true},
		OpenLoops{TodayTotal: 1, TodayDone: 1, HasTasks: true},
		boardTabToday,
	)

	emptyModel := cockpitModel(nil, nil, nil, nil, OpenLoops{Inbox: 2, HasTasks: true}, boardTabToday)

	cases := []struct {
		name   string
		model  *FocusBoardModel
		width  int
		height int
		want   []string
	}{
		{
			name:   "unicode metadata with status",
			model:  unicodeModel,
			width:  48,
			height: 18,
			want:   []string{"Fix", "Saved 修复", "x done"},
		},
		{
			name:   "all done",
			model:  doneModel,
			width:  48,
			height: 14,
			want:   []string{"All done!", "p plan more", "1/1 done"},
		},
		{
			name:   "empty inbox only",
			model:  emptyModel,
			width:  48,
			height: 16,
			want:   []string{"Triage inbox", "ctrl+i triage", "a add"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.model.SetSize(tc.width, tc.height)
			rendered := tc.model.View().Content
			plain := ansi.Strip(rendered)

			assertRenderedLinesWithinVisualWidth(t, "today compact "+tc.name, rendered, tc.width)
			assertNoFooterContentOverlap(t, "today compact "+tc.name, rendered, tc.height)
			for _, want := range tc.want {
				if !strings.Contains(plain, want) {
					t.Fatalf("compact render missing %q:\n%s", want, plain)
				}
			}
		})
	}
}

// TestTodayCockpitStripRendersAboveTaskList locks the positional contract:
// when both the Next Action strip and a planned-today row are visible, the
// strip line must appear before the task row. A regression that moves the
// strip below or alongside the rows breaks the cockpit's "recommendation
// first" reading order.
func TestTodayCockpitStripRendersAboveTaskList(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)

	plain := ansi.Strip(m.View().Content)

	stripIdx := strings.Index(plain, "Next: "+title)
	if stripIdx < 0 {
		t.Fatalf("strip line missing from render: %s", plain)
	}
	// The task row contains the title and the row prefix marker. We locate
	// the row by searching after the strip occurrence so the title's first
	// appearance (inside the strip) is not confused for the row.
	rowIdx := strings.Index(plain[stripIdx+len("Next: "+title):], title)
	if rowIdx < 0 {
		t.Fatalf("task row missing from render after strip: %s", plain)
	}
	rowAbs := stripIdx + len("Next: "+title) + rowIdx
	if rowAbs <= stripIdx {
		t.Fatalf("expected strip (idx %d) above task row (idx %d), got reversed order", stripIdx, rowAbs)
	}
	// Sanity: at least one newline between them so the strip doesn't share a
	// row with the task list.
	if !strings.Contains(plain[stripIdx:rowAbs], "\n") {
		t.Fatalf("expected newline between strip and task row, got: %q", plain[stripIdx:rowAbs])
	}
}

// TestTodayCockpitSelectedRowDistinctFromStrip locks the visual-distinction
// contract: the strip's title is bold-on-default while the selected task row
// is rendered on a colored background. The ANSI escape signature of those
// two styles must not match — otherwise the user can't tell at a glance
// which line is "the recommendation" vs "the focused list row".
func TestTodayCockpitSelectedRowDistinctFromStrip(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)
	m.cursor = 0
	m.applyFilter()

	rendered := m.View().Content
	if rendered == "" {
		t.Fatalf("empty render")
	}

	// Selected row uses SelectedStyle which sets both background and
	// foreground — i.e., the line emits an ANSI sequence containing a
	// background code (\x1b[…;48;…m or 48;2;r;g;bm).
	bgEscape := regexp.MustCompile(`\x1b\[[^m]*48;`)
	if !bgEscape.MatchString(rendered) {
		t.Fatalf("expected selected row to emit a background-color ANSI escape, got none")
	}

	// Find the strip line and verify it does NOT carry a background-color
	// escape on its own. The strip uses bold foreground only.
	stripLineRegexp := regexp.MustCompile(`(?m)^.*Next: Ship overdue invoice.*$`)
	stripLine := stripLineRegexp.FindString(rendered)
	if stripLine == "" {
		t.Fatalf("strip line not found in render: %s", ansi.Strip(rendered))
	}
	if bgEscape.MatchString(stripLine) {
		t.Fatalf("strip line should not carry a background-color escape (would collide with selected-row styling): %q", stripLine)
	}

	// And the strip line must carry bold so it still draws the eye. ANSI
	// bold attribute is `1` between the CSI and the `m` terminator.
	boldEscape := regexp.MustCompile(`\x1b\[[^m]*\b1\b[^m]*m`)
	if !boldEscape.MatchString(stripLine) {
		t.Fatalf("strip line should be bold-styled, got %q", stripLine)
	}
}

// TestTodayCockpitStripSuppressedOnNextTab verifies the strip renders only on
// the Today tab. Switching to Next (or Closed) must drop the strip so the
// cockpit-only affordance does not leak into list-mode tabs.
func TestTodayCockpitStripSuppressedOnNextTab(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)
	// Provide a Next-tab row so the model has content to render on the
	// switched tab.
	nextProject := cockpitProject(t, "Other")
	nextTask := cockpitTask(t, nextProject, "Other next-tab task", models.StatusTodo, models.SectionNext)
	m.nextRows = []boardRow{cockpitBoardRow(nextTask, nextProject)}
	m.allRows = append(m.allRows, m.nextRows...)
	m.tabs.Active = boardTabNext
	m.applyFilter()

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Next: "+title) {
		t.Fatalf("strip leaked into Next tab: %s", plain)
	}
	if !strings.Contains(plain, "Other next-tab task") {
		t.Fatalf("Next tab row missing: %s", plain)
	}
}

// TestTodayCockpitStripSuppressedOnClosedTab is the Closed-tab counterpart of
// the Next-tab suppression check above. Closed list-mode renders only history
// rows and the strip must stay hidden.
func TestTodayCockpitStripSuppressedOnClosedTab(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)
	closedProject := cockpitProject(t, "Closed Project")
	closedTask := cockpitTask(t, closedProject, "Old closed task", models.StatusDone, models.SectionCompleted)
	m.closedRows = []boardRow{cockpitBoardRow(closedTask, closedProject)}
	m.tabs.Active = boardTabClosed
	m.applyFilter()

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Next: "+title) {
		t.Fatalf("strip leaked into Closed tab: %s", plain)
	}
	if !strings.Contains(plain, "Old closed task") {
		t.Fatalf("Closed tab row missing: %s", plain)
	}
	if !strings.Contains(plain, ",o reopen") || !strings.Contains(plain, "enter/o details") || !strings.Contains(plain, "tD delete") {
		t.Fatalf("Closed tab footer missing updated task actions: %s", plain)
	}
}

// TestTodayCockpitStripSuppressedInTunnelMode covers the tunnel-mode branch.
// Tunnel mode replaces the title with the focused task itself; the Next
// Action strip would be redundant and is explicitly guarded out in
// focus_board_render.go.
func TestTodayCockpitStripSuppressedInTunnelMode(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)
	m.focusMode = focusModeTunnel

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Next: "+title) {
		t.Fatalf("strip should be suppressed in tunnel mode, got: %s", plain)
	}
	if !strings.Contains(plain, "Currently: "+title) {
		t.Fatalf("tunnel header missing the focused task: %s", plain)
	}
}

// TestTodayCockpitStripSuppressedBelowMinWidth verifies that a narrow Today
// surface drops the strip rather than corrupting the layout. The threshold
// matches NextActionStripMinWidth so the rest of the cockpit (rows, footer)
// keeps its budget. We pick a width one below the threshold; the strip
// renderer returns "" for that width and the model's stripHeight stays 0.
// The fixture uses a short title that survives the narrow-row truncation
// budget so the assertion is about the strip vs the surrounding chrome,
// not about the row's title-truncation behavior.
func TestTodayCockpitStripSuppressedBelowMinWidth(t *testing.T) {
	const title = "Ship"
	m := cockpitWorkOnTaskFixture(t, title)
	m.SetSize(NextActionStripMinWidth-1, 30)

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Next: "+title) {
		t.Fatalf("strip should be suppressed below NextActionStripMinWidth, got: %s", plain)
	}
	// The Today subtitle and task row should still be present — the strip
	// suppression must not break the rest of the surface. We assert on the
	// "Today" header and on the planned-row prefix `1.` because both survive
	// the narrow column budget while the full row content does not.
	if !strings.Contains(plain, "Today") {
		t.Fatalf("Today header should still render with strip suppressed: %s", plain)
	}
	if !strings.Contains(plain, "1.") {
		t.Fatalf("planned-row prefix should still render with strip suppressed: %s", plain)
	}
}

// TestTodayCockpitStripVisibleInAddMode locks the layout invariant that
// activating quick-add does not hide the strip. The strip is positioned
// above the search and add inputs so add-mode coexists with the
// recommendation rather than blanking it.
func TestTodayCockpitStripVisibleInAddMode(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)
	m.addMode = true
	m.addInput.SetValue("New planned task")
	m.addInput.Focus()

	plain := ansi.Strip(m.View().Content)
	stripIdx := strings.Index(plain, "Next: "+title)
	if stripIdx < 0 {
		t.Fatalf("strip should remain visible in add mode, got: %s", plain)
	}
	// The add-input preview line ("> New planned task") must appear after
	// the strip — strip stays at the top of the surface.
	addIdx := strings.Index(plain, "> New planned task")
	if addIdx < 0 {
		t.Fatalf("add-input line missing from render: %s", plain)
	}
	if addIdx <= stripIdx {
		t.Fatalf("strip (idx %d) must precede add input (idx %d) in render", stripIdx, addIdx)
	}
}

// TestTodayCockpitStripVisibleInEditMode covers the quick-edit path. The
// in-row edit input replaces a single row but the strip — which lives
// above the row list — must continue to render so the user keeps the
// "what to do next" cue while renaming a planned task.
func TestTodayCockpitStripVisibleInEditMode(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)
	m.editMode = true
	m.editIdx = 0
	m.editInput.SetValue("Renamed task")
	m.editInput.Focus()

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Next: "+title) {
		t.Fatalf("strip should remain visible in edit mode, got: %s", plain)
	}
	if !strings.Contains(plain, "> Renamed task") {
		t.Fatalf("edit input line missing from render: %s", plain)
	}
	// The original row title must NOT appear in the list area (the edit
	// input replaced it). It only appears inside the strip line at this
	// point — verify that single occurrence.
	if got := strings.Count(plain, title); got != 1 {
		t.Fatalf("expected exactly one occurrence of %q (inside the strip only) while in edit mode, got %d in: %s", title, got, plain)
	}
}

// TestTodayCockpitStripVisibleAlongsideQueueIndicator confirms that the
// daily-plan queue indicator (the "w" prefix on planned rows) still renders
// in the row body when the strip is present. This guards the "queue toggles
// working" requirement from the session brief — the strip must not steal
// the row's queue-indicator real estate.
func TestTodayCockpitStripVisibleAlongsideQueueIndicator(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Next: "+title) {
		t.Fatalf("strip should be visible: %s", plain)
	}
	// The planned-row prefix is "1." (since plannedIdx starts at 1). It
	// must appear in the row area, which lives below the strip line.
	stripIdx := strings.Index(plain, "Next: "+title)
	tail := plain[stripIdx:]
	if !strings.Contains(tail, "1.") {
		t.Fatalf("planned-row prefix `1.` should render below the strip, got: %s", tail)
	}
}

// TestTodayCockpitStripStaysOnEmptyState locks the empty-state invariant: a
// Today tab with no planned rows still shows the strip (set to the first-run
// or "all caught up" variant), so onboarding cues stay visible above the
// empty-state copy. This matches the contract baseline_cockpit_test.go's
// TestBaselineCockpitEmptyAppRendersAcrossSizes already exercises across
// widths; this test pins the relative ordering of strip vs empty-state copy.
func TestTodayCockpitStripStaysOnEmptyState(t *testing.T) {
	m := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabToday)
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	stripIdx := strings.Index(plain, "Create your first project")
	if stripIdx < 0 {
		t.Fatalf("first-run strip copy missing: %s", plain)
	}
	// Session 6 replaced the passive empty-state body ("No tasks planned for
	// today") with action-oriented copy. The first-run variant restates the
	// strip CTA as "Capture your first task to get started." — the strip
	// still leads the reading order and must precede this body.
	emptyIdx := strings.Index(plain, "Capture your first task to get started.")
	if emptyIdx < 0 {
		t.Fatalf("empty-state copy missing: %s", plain)
	}
	if stripIdx >= emptyIdx {
		t.Fatalf("expected strip (idx %d) above empty-state copy (idx %d)", stripIdx, emptyIdx)
	}
}

// TestTodayCockpitWPlanShortcutHintStillPresent guards the footer-hint copy
// for the steady-state Today tab. Even with the strip occupying the top of
// the surface, the footer must continue to advertise `x done`, `p priority`,
// and `a add` so muscle memory keeps working. The strip's own hint cluster
// is separate from this footer.
func TestTodayCockpitActionShortcutHintsStillPresent(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)

	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"x done", "p priority", "a add"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("footer hint %q missing alongside strip: %s", want, plain)
		}
	}
}
