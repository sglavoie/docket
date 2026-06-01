package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func assertPeekLinesWithinWidth(t *testing.T, lines []string, width int) {
	t.Helper()
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("line %d exceeds width %d: width=%d, content=%q (stripped=%q)", i, width, w, line, ansi.Strip(line))
		}
	}
}

func TestRenderPeekRightPane_EmptyTaskShowsPlaceholder(t *testing.T) {
	task := &models.Task{}
	got := renderPeekRightPane(task, 40)
	if len(got) != 1 {
		t.Fatalf("expected single placeholder line, got %d: %v", len(got), got)
	}
	if !strings.Contains(ansi.Strip(got[0]), "No metadata") {
		t.Fatalf("expected 'No metadata' placeholder; got: %q", ansi.Strip(got[0]))
	}
}

func TestRenderPeekRightPane_OnlyResolvedCommentsHidesSection(t *testing.T) {
	task := &models.Task{
		Comments: []*models.Comment{
			{Text: "resolved one", CreatedAt: time.Now(), Resolved: true},
			{Text: "resolved two", CreatedAt: time.Now(), Resolved: true},
		},
	}
	got := renderPeekRightPane(task, 40)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "Comments") {
		t.Fatalf("Comments section should be hidden when all are resolved; got:\n%s", joined)
	}
}

func TestRenderPeekRightPane_FiltersResolvedDecisions(t *testing.T) {
	task := &models.Task{
		Decisions: []*models.Decision{
			{Text: "live decision", Category: models.DecisionCategoryDesign},
			{Text: "old decision", Category: models.DecisionCategoryDesign, Resolved: true},
		},
	}
	got := renderPeekRightPane(task, 60)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "Decisions (1)") {
		t.Fatalf("expected header 'Decisions (1)'; got:\n%s", joined)
	}
	if !strings.Contains(joined, "live decision") {
		t.Fatalf("expected unresolved decision text; got:\n%s", joined)
	}
	if strings.Contains(joined, "old decision") {
		t.Fatalf("resolved decision should be hidden; got:\n%s", joined)
	}
}

func TestRenderPeekRightPane_ChecklistHeaderShowsProgress(t *testing.T) {
	task := &models.Task{
		Checklist: []models.ChecklistItem{
			{Text: "a", Done: true},
			{Text: "b", Done: true},
			{Text: "c"},
			{Text: "d"},
			{Text: "e"},
		},
	}
	got := renderPeekRightPane(task, 40)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "Checklist (2/5)") {
		t.Fatalf("expected 'Checklist (2/5)' header; got:\n%s", joined)
	}
	// Body contains item texts.
	for _, item := range []string{"a", "b", "c", "d", "e"} {
		if !strings.Contains(joined, item) {
			t.Fatalf("expected checklist item %q; got:\n%s", item, joined)
		}
	}
}

func TestRenderPeekRightPane_AttachmentCodeRefShowsLineNumber(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: "main.go", IsCodeRef: true, LineNumber: 42},
		},
	}
	got := renderPeekRightPane(task, 40)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "Attachments (1)") {
		t.Fatalf("expected 'Attachments (1)' header; got:\n%s", joined)
	}
	if !strings.Contains(joined, "main.go:42") {
		t.Fatalf("expected 'main.go:42' code ref; got:\n%s", joined)
	}
}

func TestRenderPeekRightPane_HidesEmptySectionsBetweenBlocks(t *testing.T) {
	task := &models.Task{
		Checklist: []models.ChecklistItem{{Text: "a"}},
		Comments: []*models.Comment{
			{Text: "live comment", CreatedAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
			{Text: "resolved comment", CreatedAt: time.Now(), Resolved: true},
		},
	}
	got := renderPeekRightPane(task, 40)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "Checklist") {
		t.Fatalf("expected Checklist section; got:\n%s", joined)
	}
	if !strings.Contains(joined, "Comments (1)") {
		t.Fatalf("expected 'Comments (1)'; got:\n%s", joined)
	}
	if strings.Contains(joined, "Decisions") {
		t.Fatalf("Decisions should be absent; got:\n%s", joined)
	}
	if strings.Contains(joined, "Attachments") {
		t.Fatalf("Attachments should be absent; got:\n%s", joined)
	}
}

func TestRenderPeekDescription_EmptyReturnsPlaceholder(t *testing.T) {
	task := &models.Task{Description: "   "}
	got := renderPeekDescription(task, 40)
	if len(got) != 1 {
		t.Fatalf("expected single placeholder line, got %d: %v", len(got), got)
	}
	if !strings.Contains(ansi.Strip(got[0]), "No description") {
		t.Fatalf("expected 'No description' placeholder; got: %q", ansi.Strip(got[0]))
	}
}

func TestPeekChecklist_StyledItemDoesNotOverflowWidth(t *testing.T) {
	task := &models.Task{
		Checklist: []models.ChecklistItem{
			{Text: "Have Claude/GPT do a proper review of what we had before and make sure new pipeline is reworked so it works without requiring new permissions"},
			{Text: "Test Trad-X properly in isolation to make sure it works fine, deploy that standalone", Done: true},
		},
	}
	const width = 30
	lines := peekChecklistBlock(task, width)
	if len(lines) == 0 {
		t.Fatal("expected lines, got none")
	}
	assertPeekLinesWithinWidth(t, lines, width)
}

func TestPeekComments_LongTextDoesNotOverflowWidth(t *testing.T) {
	task := &models.Task{
		Comments: []*models.Comment{
			{
				Text:      "Some strings can't be translated, they are defined server-side (e.g., tooltips for things like regions)\nand this explicit second line should stay inside the metadata pane too",
				CreatedAt: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	const width = 30
	lines := peekCommentsBlock(task, width)
	if len(lines) == 0 {
		t.Fatal("expected lines, got none")
	}
	assertPeekLinesWithinWidth(t, lines, width)
}

func TestPeekDecisions_LongTextAndRationaleDoNotOverflowWidth(t *testing.T) {
	task := &models.Task{
		Decisions: []*models.Decision{
			{
				Text:      "Use server-owned metadata for dynamic regional tooltip copy until the API exposes stable translation keys",
				Rationale: "Client-side translation would guess at values that are not available during initial render.\nKeep the rationale wrapped in the same pane.",
				Category:  models.DecisionCategoryArchitecture,
			},
		},
	}
	const width = 32
	lines := peekDecisionsBlock(task, width)
	if len(lines) == 0 {
		t.Fatal("expected lines, got none")
	}
	assertPeekLinesWithinWidth(t, lines, width)
}

func TestPeekAttachments_LongTextDoesNotOverflowWidth(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{
				Filename:     "translation-region-tooltip-behavior-investigation-notes.md",
				IsURL:        true,
				OriginalPath: "https://example.com/product/regions/tooltip/translation/server-owned-copy",
			},
		},
	}
	const width = 28
	lines := peekAttachmentsBlock(task, width)
	if len(lines) == 0 {
		t.Fatal("expected lines, got none")
	}
	assertPeekLinesWithinWidth(t, lines, width)
}

func TestRenderPeekRightPane_LongMetadataDoesNotOverflowWidth(t *testing.T) {
	task := &models.Task{
		Checklist: []models.ChecklistItem{
			{Text: "Confirm comments, decisions, checklist rows, and attachments all obey the same pane width"},
			{Text: "Completed checklist rows should remain muted while wrapping inside the pane", Done: true},
		},
		Decisions: []*models.Decision{
			{
				Text:      "Document that server-side metadata can provide strings the client cannot translate",
				Rationale: "The peek pane must preserve explicit newlines\nwithout letting continuation rows spill into the description pane.",
				Category:  models.DecisionCategoryDesign,
			},
		},
		Comments: []*models.Comment{
			{
				Text:      "Some strings can't be translated, they are defined server-side (e.g., tooltips for things like regions)",
				CreatedAt: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
			},
		},
		Attachments: []models.Attachment{
			{Filename: "internal/tui/app_view_peek.go", IsCodeRef: true, LineNumber: 268},
		},
	}
	const width = 34
	lines := renderPeekRightPane(task, width)
	assertPeekLinesWithinWidth(t, lines, width)
}

func TestRenderPeekOverlay_NarrowMetadataDoesNotRewrapContent(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	task.Description = strings.Repeat("Description text that should wrap only inside the left pane. ", 4)
	task.Checklist = []models.ChecklistItem{
		{Text: "Verify narrow overlay metadata wrapping for checklist rows"},
	}
	task.Decisions = []*models.Decision{
		{
			Text:      "Keep styled metadata rows inside the right pane",
			Rationale: "Outer box wrapping should not add extra physical rows.",
			Category:  models.DecisionCategoryUX,
		},
	}
	task.Comments = []*models.Comment{
		{
			Text:      "Some strings can't be translated, they are defined server-side (e.g., tooltips for things like regions)",
			CreatedAt: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		},
	}

	app.width = 60
	app.height = 18
	app.activatePeek(task, project)
	rendered := app.renderPeekOverlay()
	lines := strings.Split(rendered, "\n")

	if want := app.peekPaneHeight + 2; len(lines) != want {
		t.Fatalf("peek overlay rendered %d lines, want %d; content was rewrapped:\n%s", len(lines), want, ansi.Strip(rendered))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > app.width {
			t.Fatalf("overlay line %d exceeds app width %d: width=%d, content=%q", i, app.width, w, ansi.Strip(line))
		}
	}
}

func TestRenderPeekDescription_WrapsAndPreservesParagraphs(t *testing.T) {
	task := &models.Task{Description: "one two three four\n\nsecond paragraph here"}
	lines := renderPeekDescription(task, 10)
	if len(lines) == 0 {
		t.Fatalf("expected wrapped lines, got none")
	}
	for _, l := range lines {
		if len(l) > 10 {
			t.Fatalf("line exceeded width 10: %q (len=%d)", l, len(l))
		}
	}
}

// TestRepeatedSpacePress_KeepsPeekActive verifies that auto-repeat space key
// presses while peek is already active do not toggle peek off. Holding space
// produces a stream of KeyPressMsg{Space} events; only the final
// KeyReleaseMsg{Space} should dismiss peek.
func TestRepeatedSpacePress_KeepsPeekActive(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.peekTask = project.Tasks[0]
	app.peekProject = project
	app.peekActive = true

	msg := tea.KeyPressMsg{Code: tea.KeySpace}
	for i := range 5 {
		result, _ := app.Update(msg)
		app = result.(*AppModel)
		if !app.peekActive {
			t.Fatalf("peekActive became false after press #%d; should remain true while space is held", i+1)
		}
	}
}

// peekStubView is a minimal View implementation used to exercise the
// interface-based peek dispatch (`taskRefProvider`) and multi-select guard
// (`multiSelectStateful`) without standing up a real view's full state.
type peekStubView struct {
	task        *models.Task
	project     *models.Project
	multiSelect bool
}

func (s *peekStubView) Init() tea.Cmd                       { return nil }
func (s *peekStubView) Update(tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s *peekStubView) View() tea.View                      { return tea.NewView("") }
func (s *peekStubView) SetSize(int, int)                    {}
func (s *peekStubView) getTaskRef() (*models.Task, *models.Project) {
	return s.task, s.project
}
func (s *peekStubView) IsMultiSelectActive() bool { return s.multiSelect }

// peekEligibleViews lists every ViewType that Session 2 enabled for hold-to-peek.
// The interface dispatch in getCurrentCursorTaskAndProject is identical for all
// of them, so a single stub registered at each ViewType is sufficient to verify
// the dispatch contract per view.
var peekEligibleViews = []ViewType{
	ViewFocusView,
	ViewFocusBoard,
	ViewDailyReview,
	ViewWeeklyReview,
	ViewDueDateOverview,
	ViewPriorityView,
	ViewRecentActivity,
	ViewProjectFocusView,
	ViewKanban,
	ViewCrossProjectKanban,
}

// TestSpace_ActivatesPeek_AcrossViews verifies that pressing space in each
// S2-enabled view activates peek with the cursor task supplied by getTaskRef,
// and that releasing space dismisses peek. Uses a stub view to isolate the
// dispatch path from per-view setup complexity.
func TestSpace_ActivatesPeek_AcrossViews(t *testing.T) {
	for _, vt := range peekEligibleViews {
		t.Run(vt.String(), func(t *testing.T) {
			app, project := newTestAppModelWithProject()
			stub := &peekStubView{task: project.Tasks[0], project: project}
			app.views.Register(vt, stub)
			app.viewStack = []ViewType{vt}

			press := tea.KeyPressMsg{Code: tea.KeySpace}
			result, _ := app.Update(press)
			app = result.(*AppModel)

			if !app.peekActive {
				t.Fatalf("peek not activated after space press in view %s", vt)
			}
			if app.peekTask != project.Tasks[0] {
				t.Fatalf("peekTask mismatch in view %s", vt)
			}
			if app.peekProject != project {
				t.Fatalf("peekProject mismatch in view %s", vt)
			}

			release := tea.KeyReleaseMsg{Code: tea.KeySpace}
			result, _ = app.Update(release)
			app = result.(*AppModel)

			if app.peekActive {
				t.Fatalf("peek not dismissed after space release in view %s", vt)
			}
		})
	}
}

// TestSpace_DoesNotPeekDuringMultiSelect verifies that when a view reports
// active multi-select, the global space handler defers to the view (lets it
// handle toggle-select) instead of activating peek.
func TestSpace_DoesNotPeekDuringMultiSelect(t *testing.T) {
	app, project := newTestAppModelWithProject()
	stub := &peekStubView{
		task:        project.Tasks[0],
		project:     project,
		multiSelect: true,
	}
	app.views.Register(ViewFocusBoard, stub)
	app.viewStack = []ViewType{ViewFocusBoard}

	press := tea.KeyPressMsg{Code: tea.KeySpace}
	result, _ := app.Update(press)
	app = result.(*AppModel)

	if app.peekActive {
		t.Fatal("peek should not activate while multi-select is active")
	}
}

// TestPeek_DismissesEvenWithoutKittyRelease documents the spec: once peek is
// active, only esc or KeyReleaseMsg{Space} dismisses it. Unrelated messages
// (e.g. WindowSizeMsg) leave peek active.
func TestPeek_DismissesEvenWithoutKittyRelease(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.peekTask = project.Tasks[0]
	app.peekProject = project
	app.peekActive = true

	result, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = result.(*AppModel)

	if !app.peekActive {
		t.Fatal("peek should remain active after unrelated WindowSizeMsg")
	}

	result, _ = app.Update(tea.KeyReleaseMsg{Code: tea.KeySpace})
	app = result.(*AppModel)

	if app.peekActive {
		t.Fatal("peek should be dismissed after KeyReleaseMsg{Space}")
	}
}

// TestPeekEligibleViews_SatisfyTaskRefProvider compile-time-asserts that every
// real view model expected to support hold-to-peek implements the
// taskRefProvider interface dispatched in getCurrentCursorTaskAndProject.
// A missing method here means the view will silently fail to activate peek.
func TestPeekEligibleViews_SatisfyTaskRefProvider(t *testing.T) {
	type taskRefProvider interface {
		getTaskRef() (*models.Task, *models.Project)
	}
	views := []any{
		(*FocusViewModel)(nil),
		(*FocusBoardModel)(nil),
		(*AllTasksViewModel)(nil),
		(*DailyReviewModel)(nil),
		(*WeeklyReviewModel)(nil),
		(*DueDateOverviewModel)(nil),
		(*PriorityViewModel)(nil),
		(*RecentActivityModel)(nil),
		(*ProjectFocusViewModel)(nil),
		(*OutlinerModel)(nil),
		(*ProjectTreeModel)(nil),
		(*KanbanModel)(nil),
		(*InboxProcessorModel)(nil),
		(*GlobalInboxProcessorModel)(nil),
	}
	for _, v := range views {
		if _, ok := v.(taskRefProvider); !ok {
			t.Errorf("%T does not implement taskRefProvider — peek dispatch will skip it", v)
		}
	}
}

func TestClampPeekOffset(t *testing.T) {
	cases := []struct {
		name          string
		offset, total int
		visible, want int
	}{
		{"zero floor", -3, 100, 10, 0},
		{"within range", 5, 100, 10, 5},
		{"clamped to max", 200, 100, 10, 90},
		{"no scroll when fits", 5, 10, 10, 0},
		{"no scroll when empty", 5, 0, 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampPeekOffset(tc.offset, tc.total, tc.visible); got != tc.want {
				t.Errorf("clampPeekOffset(%d, %d, %d) = %d, want %d", tc.offset, tc.total, tc.visible, got, tc.want)
			}
		})
	}
}
