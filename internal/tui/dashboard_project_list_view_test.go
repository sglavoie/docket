package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestProjectStatusWidthBlockedMatchesIndicator ensures the precomputed
// status-indicator column width matches the actual rendered width when the
// project has blocked tasks. A mismatch causes column misalignment in the
// dashboard project list.
func TestProjectStatusWidthBlockedMatchesIndicator(t *testing.T) {
	icons.SetUseEmoji(true)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	p := &models.Project{
		ID:   "p1",
		Name: "Project",
		Tasks: []*models.Task{
			{ID: "t1", Title: "blocked task", WorkflowState: models.StateWaiting},
		},
	}

	m := &DashboardModel{}
	got := m.projectStatusWidth(p, 3)
	rendered := m.projectRowStatusIndicator(p, 3)
	want := lipgloss.Width(rendered)

	if got != want {
		t.Fatalf("projectStatusWidth = %d, rendered width = %d (indicator %q)", got, want, rendered)
	}
}

// TestProjectRowStatusIndicatorSeparatesIconsAndCounts verifies each compact
// dashboard status badge retains a readable space between its icon and count
// in both Unicode and emoji modes, while keeping width prediction in sync.
func TestProjectRowStatusIndicatorSeparatesIconsAndCounts(t *testing.T) {
	now := time.Now()
	project := &models.Project{
		ID:   "p1",
		Name: "Project",
		Path: t.TempDir(),
		Tasks: []*models.Task{
			{ID: "inbox", Title: "Inbox", WorkflowState: models.StateInbox},
			{ID: "overdue", Title: "Overdue", WorkflowState: models.StateReady, DueDate: dashboardTimePtr(now.AddDate(0, 0, -1))},
			{ID: "due-soon", Title: "Due soon", WorkflowState: models.StateReady, DueDate: dashboardTimePtr(now)},
			{ID: "blocked", Title: "Blocked", WorkflowState: models.StateWaiting},
			{ID: "complete", Title: "Complete", WorkflowState: models.StateDone},
		},
	}
	if err := storage.SaveBlockers(project, []*models.Blocker{models.NewBlocker("Waiting on review")}); err != nil {
		t.Fatalf("SaveBlockers: %v", err)
	}

	for _, useEmoji := range []bool{false, true} {
		t.Run(map[bool]string{false: "unicode", true: "emoji"}[useEmoji], func(t *testing.T) {
			icons.SetUseEmoji(useEmoji)
			t.Cleanup(func() { icons.SetUseEmoji(false) })

			m := &DashboardModel{}
			rendered := m.projectRowStatusIndicator(project, 3)
			plain := ansi.Strip(rendered)
			for _, icon := range []string{
				icons.Get("inbox"),
				icons.Get("overdue"),
				icons.Get("due_today"),
				icons.Get("blocked_dot"),
				icons.Get("blockers"),
				IconChecklist(),
			} {
				want := fmt.Sprintf("%s 1", icon)
				if !strings.Contains(plain, want) {
					t.Fatalf("indicator %q does not contain spaced badge %q", plain, want)
				}
			}

			if got, want := m.projectStatusWidth(project, 3), lipgloss.Width(rendered); got != want {
				t.Fatalf("projectStatusWidth = %d, rendered width = %d (indicator %q)", got, want, plain)
			}
		})
	}
}

func dashboardTimePtr(t time.Time) *time.Time { return &t }

func TestDashboardProjectIconColumnAlignsBlankIconRows(t *testing.T) {
	projects := []*models.Project{
		{ID: "with-icon", Name: "With Icon", StatusIcon: "planning"},
		{ID: "no-icon", Name: "No Icon"},
	}
	m := &DashboardModel{}

	for _, useEmoji := range []bool{false, true} {
		t.Run(map[bool]string{false: "unicode", true: "emoji"}[useEmoji], func(t *testing.T) {
			icons.SetUseEmoji(useEmoji)
			t.Cleanup(func() { icons.SetUseEmoji(false) })

			maxNameWidth, _, _, iconColWidth := m.projectListColumnWidths(projects, false, 3)
			withIcon := ansi.Strip(m.renderProjectRowName(projects[0], NormalStyle, maxNameWidth, iconColWidth))
			noIcon := ansi.Strip(m.renderProjectRowName(projects[1], NormalStyle, maxNameWidth, iconColWidth))

			withIconStart := titleStartColumn(t, withIcon, projects[0].Name)
			noIconStart := titleStartColumn(t, noIcon, projects[1].Name)
			if withIconStart != noIconStart {
				t.Fatalf("project name columns differ: with icon starts at %d (%q), no icon starts at %d (%q)",
					withIconStart, withIcon, noIconStart, noIcon)
			}
		})
	}
}

// TestDashboardProjectIconColumnGutterWidth ensures a project status icon is
// followed by exactly dashboardProjectIconGutterWidth visual cells before the
// project title, in both unicode (⋙ for project.wip) and emoji (🚧) modes, and
// that no-icon rows stay aligned with icon rows.
func TestDashboardProjectIconColumnGutterWidth(t *testing.T) {
	projects := []*models.Project{
		{ID: "wip", Name: "trail", StatusIcon: "wip"},
		{ID: "no-icon", Name: "No Icon"},
	}
	m := &DashboardModel{}

	for _, useEmoji := range []bool{false, true} {
		t.Run(map[bool]string{false: "unicode", true: "emoji"}[useEmoji], func(t *testing.T) {
			icons.SetUseEmoji(useEmoji)
			t.Cleanup(func() { icons.SetUseEmoji(false) })

			icon := projects[0].StatusIconEmoji()
			if icon == "" {
				t.Fatalf("expected project.wip icon to be non-empty")
			}

			maxNameWidth, _, _, iconColWidth := m.projectListColumnWidths(projects, false, 3)
			withIcon := ansi.Strip(m.renderProjectRowName(projects[0], NormalStyle, maxNameWidth, iconColWidth))
			noIcon := ansi.Strip(m.renderProjectRowName(projects[1], NormalStyle, maxNameWidth, iconColWidth))

			titleStart := titleStartColumn(t, withIcon, projects[0].Name)
			gutter := titleStart - lipgloss.Width(icon)
			if gutter != dashboardProjectIconGutterWidth {
				t.Fatalf("icon %q gutter = %d, want %d (row %q)", icon, gutter, dashboardProjectIconGutterWidth, withIcon)
			}

			if noIconStart := titleStartColumn(t, noIcon, projects[1].Name); noIconStart != titleStart {
				t.Fatalf("no-icon row misaligned: title starts at %d, icon row at %d", noIconStart, titleStart)
			}
		})
	}
}

func titleStartColumn(t *testing.T, line, title string) int {
	t.Helper()
	idx := strings.Index(line, title)
	if idx < 0 {
		t.Fatalf("line %q does not contain title %q", line, title)
	}
	return lipgloss.Width(line[:idx])
}
