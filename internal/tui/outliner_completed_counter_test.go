package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func completedCounterProject(id string, states ...models.WorkflowState) *models.Project {
	project := &models.Project{ID: id, Name: id}
	for i, state := range states {
		project.Tasks = append(project.Tasks, &models.Task{
			ID:            id + "-" + string(rune('a'+i)),
			WorkflowState: state,
		})
	}
	return project
}

func TestOutlinerCompletedTaskCountUsesGlobalAndFocusedProjectScopes(t *testing.T) {
	alpha := completedCounterProject(
		"alpha",
		models.StateDone,
		models.StateCanceled,
		models.StateInProgress,
	)
	beta := completedCounterProject("beta", models.StateDone, models.StateReady)
	outliner := &OutlinerModel{
		loadedProjects: []*models.Project{alpha, beta},
		activeContext:  RegularContextScope("work"),
	}

	if got := outliner.completedTaskCount(); got != 3 {
		t.Fatalf("base Context completed count = %d, want global count 3", got)
	}

	outliner.scopedProjectID = alpha.ID
	if got := outliner.completedTaskCount(); got != 2 {
		t.Fatalf("focused alpha completed count = %d, want 2", got)
	}

	outliner.scopedProjectID = beta.ID
	if got := outliner.completedTaskCount(); got != 1 {
		t.Fatalf("focused beta completed count = %d, want 1", got)
	}
}

func TestOutlinerCompletedTaskCounterHidesZeroAndWarnsAboveTwenty(t *testing.T) {
	if got := renderCompletedTaskCounter(0); got != "" {
		t.Fatalf("zero completed counter = %q, want hidden", got)
	}
	if got, want := renderCompletedTaskCounter(20), MutedStyle.Render("completed 20"); got != want {
		t.Fatalf("counter at threshold = %q, want muted %q", got, want)
	}
	if got, want := renderCompletedTaskCounter(21), PreviewWarningStyle.Render("completed 21"); got != want {
		t.Fatalf("counter above threshold = %q, want warning %q", got, want)
	}
}

func TestOutlinerCompletedTaskCounterFollowsOpenLoopsAndDropsFirst(t *testing.T) {
	outliner := &OutlinerModel{
		loadedProjects: []*models.Project{
			completedCounterProject("alpha", models.StateDone),
		},
		openLoops: OpenLoops{Inbox: 3, Overdue: 1},
	}
	line := "Outliner"
	openLoops := outliner.renderOutlinerOpenLoopsCounter()
	completed := renderCompletedTaskCounter(1)
	fullCounter := openLoops + MutedStyle.Render(" · ") + completed

	fullWidth := lipgloss.Width(line) + 1 + lipgloss.Width(fullCounter)
	full := ansi.Strip(outliner.appendOpenLoopsCounter(line, fullWidth))
	if !strings.HasSuffix(full, "inbox 3 · overdue 1 · completed 1") {
		t.Fatalf("full counter = %q, want completed after open loops", full)
	}

	openLoopsOnlyWidth := lipgloss.Width(line) + 1 + lipgloss.Width(openLoops)
	narrow := ansi.Strip(outliner.appendOpenLoopsCounter(line, openLoopsOnlyWidth))
	if !strings.HasSuffix(narrow, "inbox 3 · overdue 1") {
		t.Fatalf("narrow counter = %q, want open loops retained", narrow)
	}
	if strings.Contains(narrow, "completed") {
		t.Fatalf("narrow counter = %q, want completed segment dropped first", narrow)
	}
}
