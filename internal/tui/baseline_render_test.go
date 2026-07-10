package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// baselineSize captures one cell of the (view × size) baseline matrix.
type baselineSize struct {
	label  string
	width  int
	height int
}

// baselineSizes are the three representative desktop terminal sizes a power
// user is likely to encounter. Every baseline fixture iterates this set so
// regressions in any of these sizes fail the harness. Sessions 2–6 inherit
// this matrix; widen it by adding a size here, not by introducing per-view
// constants.
var baselineSizes = []baselineSize{
	{"80x24", 80, 24},
	{"100x30", 100, 30},
	{"140x40", 140, 40},
}

// narrowBaseline is the opt-in narrow case. Views that have a sane narrow
// rendering today (Quick Capture, the Focus Board empty state, the
// Dashboard project list) include it; the inbox processors do not — their
// narrow rendering is a known gap recorded in the session-1 handoff and
// will tighten in sessions 3–4.
var narrowBaseline = baselineSize{"narrow 60x20", 60, 20}

// assertBaselineLineWidth fails the test if any rendered line, after ANSI
// stripping, exceeds width. This is the visual-width guard reused by every
// baseline fixture; it mirrors assertRenderedLinesWithinVisualWidth in
// narrow_layout_renderers_test.go so the harness can grow without coupling
// to a single private helper.
func assertBaselineLineWidth(t *testing.T, label, rendered string, width int) {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		w := lipgloss.Width(ansi.Strip(line))
		if w > width {
			t.Fatalf("%s line exceeds width %d: %q (width=%d)\n\n%s",
				label, width, ansi.Strip(line), w, ansi.Strip(rendered))
		}
	}
}

// assertPrimaryActionsVisible fails the test if any of the primary action
// labels are missing from the rendered output. Primary actions are the keys
// a power user reaches for first (Enter/Esc in Quick Capture; n/D/T/u in
// the inbox footer; tab/Esc in Today). The check is substring-based on the
// ANSI-stripped text so it tolerates lipgloss styling and reordering.
func assertPrimaryActionsVisible(t *testing.T, label, rendered string, actions ...string) {
	t.Helper()
	plain := ansi.Strip(rendered)
	for _, a := range actions {
		if !strings.Contains(plain, a) {
			t.Fatalf("%s: primary action %q missing from rendered output\n\n%s",
				label, a, plain)
		}
	}
}

// assertNoFooterContentOverlap fails the test when the rendered output is
// taller than the height budget passed to SetSize. Views that respect the
// budget either pad to fit (inbox) or truncate (focus board); a render that
// exceeds the budget would cause the footer to overlap content rows visually.
func assertNoFooterContentOverlap(t *testing.T, label, rendered string, height int) {
	t.Helper()
	got := lipgloss.Height(rendered)
	if got > height {
		t.Fatalf("%s rendered height %d exceeds budget %d (footer/content overlap risk)\n\n%s",
			label, got, height, ansi.Strip(rendered))
	}
}

// newBaselineInboxProject builds an in-memory project with a handful of
// processable inbox tasks used by the local inbox baseline fixture. Tasks
// stay in memory; the inbox processor does not touch disk during render.
func newBaselineInboxProject(name string) *models.Project {
	project := testutil.NewTestProject(name)
	project.EnsureDefaultSections()
	titles := []string{
		"Triage backlog email thread",
		"Decide on sprint goal for next iteration",
		"Skim research paper on triage UX",
	}
	for i, title := range titles {
		task, _ := models.NewTask(uuid.New().String(), i+1, models.TaskTypeTask, title)
		task.Section = models.SectionInbox
		project.Tasks = append(project.Tasks, task)
	}
	return project
}

// newBaselineAlertProjects builds in-memory projects designed to fill the
// dashboard alerts info card with a representative mix: triage backlog,
// overdue, due-today, and an inbox hotspot project (>=5 inbox tasks).
func newBaselineAlertProjects() []*models.Project {
	projects := make([]*models.Project, 0, 2)
	now := time.Now()

	hotspot := testutil.NewTestProject("Triage Hotspot")
	hotspot.EnsureDefaultSections()
	for i := 0; i < 6; i++ {
		task, _ := models.NewTask(uuid.New().String(), i+1, models.TaskTypeTask, fmt.Sprintf("Inbox item %d", i))
		task.Section = models.SectionInbox
		hotspot.Tasks = append(hotspot.Tasks, task)
	}
	projects = append(projects, hotspot)

	planning := testutil.NewTestProject("Planning")
	planning.EnsureDefaultSections()
	overdue, _ := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Overdue deliverable")
	overdueDue := now.Add(-48 * time.Hour)
	overdue.DueDate = &overdueDue
	overdue.Section = models.SectionNext
	dueToday, _ := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Due today checkpoint")
	dueTodayDue := now
	dueToday.DueDate = &dueTodayDue
	dueToday.Section = models.SectionNext
	planning.Tasks = append(planning.Tasks, overdue, dueToday)
	projects = append(projects, planning)

	return projects
}

// TestBaselineQuickCaptureRendersAcrossSizes locks down the composed Quick
// Capture surface across every baseline size plus the narrow minimum. Quick
// Capture is a normal (non-full-height) stack view, so the footer is owned by
// AppModel.View() rather than the child model. Composing through the app (not
// rendering the child directly) exercises the real single-footer contract:
// this baseline asserts the capture title/body renders, the primary footer
// actions are present, exactly one app-shell footer separator is drawn
// (footer/content separation), and the total height stays within the terminal
// budget. The width-only child render this replaced could not catch a footer
// that overflowed or duplicated its separator.
func TestBaselineQuickCaptureRendersAcrossSizes(t *testing.T) {
	sizes := append([]baselineSize{}, baselineSizes...)
	sizes = append(sizes, narrowBaseline)

	for _, sz := range sizes {
		t.Run(sz.label, func(t *testing.T) {
			app, project := newTestAppModelWithProject()
			// Drive sizing through the window-size path so the registered view is
			// budgeted to terminal height minus the footer, matching real usage.
			app.handleWindowSize(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})

			if handled, _ := app.handleViewMessages(ShowQuickCaptureMsg{Project: project}); !handled {
				t.Fatal("ShowQuickCaptureMsg should be handled")
			}

			rendered := app.View().Content
			label := "quick capture " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Quick Capture",
				"Enter create",
				"? syntax",
				"Esc cancel",
			)
			// Footer/content separation: exactly one app-shell separator.
			if got := countFullWidthDividers(rendered); got != 1 {
				t.Fatalf("%s: expected exactly one app-shell footer separator, got %d\n%s",
					label, got, ansi.Strip(rendered))
			}
			// Total height stays within the terminal budget.
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// TestBaselineInboxProcessorRendersAcrossSizes locks down the local inbox
// processor: header, task display, operations panel, and the six-key triage
// footer must all fit each baseline size. Session 3 shortened the title to
// "Inbox" and compacted the subtitle to "1/N · X done · Y skipped" so the
// header fits 80 cols with room to spare; the panel also surfaces the
// w: Today action introduced this session. The narrow 60x20 case remains
// height-blocked (task card + 4-group panel + footer exceed 20 lines).
func TestBaselineInboxProcessorRendersAcrossSizes(t *testing.T) {
	project := newBaselineInboxProject("Inbox Baseline")
	cfg := testutil.NewTestConfig()

	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			m := NewInboxProcessorModel(project, cfg, nil)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "inbox processor " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"n", "Skip",
				"D", "Done",
				"T", "Tags",
				"u", "Undo",
				"esc", "Back",
				"w", "Today",
				"Where should this task go next?",
			)
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// TestBaselineGlobalInboxProcessorRendersAcrossSizes locks down the global
// inbox processor: title, cross-project task display, ops panel, and footer
// must fit each baseline size. Session 3 compacted the header to
// "Global Inbox  1/N across M · X done · Y skipped" so the 80x24 cell now
// renders cleanly; the per-size floor previously set to 100 is removed.
// The w: Today action and the four-group panel must also be visible across
// all sizes so the local/global processors stay in lockstep.
func TestBaselineGlobalInboxProcessorRendersAcrossSizes(t *testing.T) {
	cfg := testutil.NewTestConfig()

	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			m := NewGlobalInboxProcessorModel(cfg, nil)

			// Inject in-memory inbox tasks after construction so we exercise
			// the populated render path without writing fixtures to disk. The
			// model expects backend.taskCount() > 0 to render the populated
			// chrome.
			project := newBaselineInboxProject("Global Inbox Baseline")
			m.projectCache = map[string]*models.Project{project.ID: project}
			m.inboxTasks = nil
			for _, task := range project.Tasks {
				m.inboxTasks = append(m.inboxTasks, GlobalInboxTask{
					Task:    task,
					Project: project,
				})
			}
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "global inbox processor " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"n", "Skip",
				"D", "Done",
				"esc", "Back",
				"w", "Today",
				"Where should this task go next?",
			)
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// TestBaselineDashboardTriageAlertsRendersAcrossSizes locks down the
// dashboard's triage-alert surface: the expanded alerts info card must
// render its "to triage" and "overdue" rollup lines without overflowing
// the card width or exceeding the terminal height. Session 5 extends this
// fixture to cover the end-state momentum copy.
func TestBaselineDashboardTriageAlertsRendersAcrossSizes(t *testing.T) {
	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			cfg := testutil.NewTestConfig()
			cfg.DataDir = t.TempDir()

			m := NewDashboardModel(cfg, nil)
			// loadProjects ran with an empty cfg.Projects, so m.projects is
			// nil. Assign fresh in-memory projects (with their tasks intact)
			// so the alerts card sees real triage backlog and overdue items.
			m.projects = newBaselineAlertProjects()
			// Compact mode collapses alerts to a single "Alerts · clear" line
			// regardless of content. Expand so the rollup lines render.
			m.dashboardCompactCards = false
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "dashboard alerts " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"to triage",
				"overdue",
			)
		})
	}
}

// TestBaselineFocusBoardTodayRendersAcrossSizes locks down the Today view's
// chrome (title, tab bar, empty-state hint) and footer at each baseline size
// plus the narrow case. With no daily plan or queue, the focus board takes
// the empty-state path, which is exactly the surface session 5 will hand
// the user momentum copy on.
func TestBaselineFocusBoardTodayRendersAcrossSizes(t *testing.T) {
	cfg := testutil.NewTestConfig()
	sizes := append([]baselineSize{}, baselineSizes...)
	sizes = append(sizes, narrowBaseline)

	for _, sz := range sizes {
		t.Run(sz.label, func(t *testing.T) {
			dailyPlan := []FocusTask{}
			m := NewFocusBoardModel(cfg, nil,
				&dailyPlan,
				map[string]bool{}, // queueIDs
				map[string]bool{}, // dailyPlanIDs
				map[string]bool{}, // carriedOverIDs
			)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "focus board " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"Esc",
				"close",
			)
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}
