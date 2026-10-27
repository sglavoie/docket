package pulse

import (
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// TestWorkspaceAgendaListsWhatThePulseCounts: the agenda names the pulse's
// Today and Overdue tasks and nothing else, overdue first by oldest deadline,
// then Today by priority, and lists a task both overdue and planned once.
func TestWorkspaceAgendaListsWhatThePulseCounts(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	older := now.Add(-48 * time.Hour)
	recent := now.Add(-2 * time.Hour)
	later := now.Add(4 * time.Hour)
	wake := now.Add(time.Hour)
	done := &models.Task{ID: "done", ShortID: 1, Title: "Done", WorkflowState: models.StateDone, DueDate: &older}
	lateRecent := &models.Task{ID: "late-recent", ShortID: 2, Title: "Late recent", WorkflowState: models.StateDoing, DueDate: &recent}
	lateOlder := &models.Task{ID: "late-older", ShortID: 3, Title: "Late older", WorkflowState: models.StateBacklog, DueDate: &older, Priority: models.PriorityLow}
	planLow := &models.Task{ID: "plan-low", ShortID: 4, Title: "Plan low", WorkflowState: models.StateBacklog, Priority: models.PriorityLow}
	planHigh := &models.Task{ID: "plan-high", ShortID: 5, Title: "Plan high", WorkflowState: models.StateBacklog, Priority: models.PriorityHigh, DueDate: &later}
	planUnset := &models.Task{ID: "plan-unset", ShortID: 6, Title: "Plan unset", WorkflowState: models.StateDoing}
	deferred := &models.Task{ID: "deferred", ShortID: 7, Title: "Deferred", WorkflowState: models.StateBacklog, DeferredUntil: &wake}
	unplanned := &models.Task{ID: "unplanned", ShortID: 8, Title: "Unplanned", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "p", Slug: "p", Name: "Work", Tasks: []*models.Task{
		done, lateRecent, lateOlder, planLow, planHigh, planUnset, deferred, unplanned,
	}}
	project.BuildTaskTree()
	today := map[taskref.TaskIdentity]bool{}
	for _, task := range []*models.Task{lateRecent, planLow, planHigh, planUnset, deferred, done} {
		today[taskref.For(task, project)] = true
	}

	agenda := ComputeWorkspaceAgenda([]*models.Project{project}, today, now)
	var got []string
	for _, item := range agenda.Items {
		kind := "today"
		if item.Kind == AgendaOverdue {
			kind = "overdue"
		}
		got = append(got, kind+":"+item.TaskID)
	}
	want := []string{"overdue:late-older", "overdue:late-recent", "today:plan-high", "today:plan-low", "today:plan-unset"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("agenda = %v, want %v", got, want)
	}
	if agenda.Total != len(want) {
		t.Fatalf("total = %d, want %d", agenda.Total, len(want))
	}
	if first := agenda.Items[0]; first.ProjectName != "Work" || first.Title != "Late older" || first.Due == nil {
		t.Fatalf("first item = %+v", first)
	}

	counted := ComputeWorkspacePulse([]*models.Project{project}, today, now)
	if counted.Overdue != 2 || counted.Today != 4 {
		t.Fatalf("pulse = %+v; the agenda's fixture drifted from what the pulse counts", counted)
	}

	lateOlder.Hidden = true
	if hidden := ComputeWorkspaceAgenda([]*models.Project{project}, today, now); hidden.Total != len(want)-1 {
		t.Fatalf("hidden work listed: %+v", hidden.Items)
	}
}

// TestWorkspaceAgendaBoundsItsListButCountsEverything: a host is sent at most
// AgendaLimit tasks and told how many there are.
func TestWorkspaceAgendaBoundsItsListButCountsEverything(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	project := &models.Project{ID: "p", Slug: "p"}
	today := map[taskref.TaskIdentity]bool{}
	for i := range AgendaLimit + 5 {
		task := &models.Task{ID: fmt.Sprintf("t%02d", i), ShortID: i + 1, Title: "Task", WorkflowState: models.StateBacklog}
		project.Tasks = append(project.Tasks, task)
		today[taskref.For(task, project)] = true
	}
	project.BuildTaskTree()

	agenda := ComputeWorkspaceAgenda([]*models.Project{project}, today, now)
	if len(agenda.Items) != AgendaLimit || agenda.Total != AgendaLimit+5 {
		t.Fatalf("agenda holds %d of %d, want %d of %d", len(agenda.Items), agenda.Total, AgendaLimit, AgendaLimit+5)
	}
	if agenda.Items[0].TaskID != "t00" {
		t.Fatalf("equal tasks lost their ShortID order: first = %s", agenda.Items[0].TaskID)
	}
	if !agenda.Equal(ComputeWorkspaceAgenda([]*models.Project{project}, today, now)) {
		t.Fatal("an unchanged agenda compared unequal")
	}
	project.Tasks[0].Title = "Renamed"
	if agenda.Equal(ComputeWorkspaceAgenda([]*models.Project{project}, today, now)) {
		t.Fatal("a renamed task compared equal")
	}
}
