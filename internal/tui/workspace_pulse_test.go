package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

type selectivePulseRepository struct {
	*testutil.MockRepository
	failProjectID string
}

func (r *selectivePulseRepository) Load(project *models.Project) error {
	if project != nil && project.ID == r.failProjectID {
		return errors.New("injected workspace pulse load failure")
	}
	return r.MockRepository.Load(project)
}

func installPulseRepository(t *testing.T, projects ...*models.Project) *testutil.MockRepository {
	t.Helper()
	repository := testutil.NewMockRepository()
	for _, project := range projects {
		repository.Projects[project.ID] = project
	}
	previous := storage.DefaultRepository
	storage.DefaultRepository = repository
	t.Cleanup(func() { storage.DefaultRepository = previous })
	return repository
}

func TestWorkspacePulseCountsEligibleFlatTasksAtInjectedTime(t *testing.T) {
	now := time.Date(2026, time.August, 6, 15, 30, 0, 0, time.FixedZone("test", -6*60*60))
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)
	parentID := "parent"

	parent := primaryRowsTask(parentID, models.StateInbox)
	child := primaryRowsTask("child", models.StateInbox)
	child.ParentID = &parentID
	today := primaryRowsTask("today", models.StateBacklog)
	overdue := primaryRowsTask("overdue", models.StateBacklog)
	overdue.DueDate = &yesterday
	sameDayDue := primaryRowsTask("same-day", models.StateBacklog)
	sameDayDue.DueDate = &now
	terminal := primaryRowsTask("terminal", models.StateDone)
	terminal.DueDate = &yesterday
	snoozed := primaryRowsTask("snoozed", models.StateInbox)
	snoozed.DueDate = &yesterday
	snoozed.SnoozedUntil = &tomorrow

	active := primaryRowsProject("active", parent, child, today, overdue, sameDayDue, terminal, snoozed)
	nilTasks := primaryRowsProject("nil-tasks", nil)
	parked := primaryRowsProject("parked", primaryRowsTask("parked-inbox", models.StateInbox))
	parked.Tags = []string{models.ParkedProjectTag}
	archived := primaryRowsProject("archived", primaryRowsTask("archived-inbox", models.StateInbox))
	archived.Archived = true
	runtime := testConfigWithProjects(active, nilTasks, parked, archived)
	installPulseRepository(t, active, nilTasks, parked, archived)

	todayIDs := map[TaskIdentity]bool{
		taskIdentityFor(today, active):                 true,
		taskIdentityFor(terminal, active):              true,
		taskIdentityFor(snoozed, active):               true,
		{ProjectID: active.ID, TaskID: "stale"}:        true,
		{ProjectID: parked.ID, TaskID: "parked-inbox"}: true,
	}
	pulse := ComputeWorkspacePulse(runtime, todayIDs, now)

	if pulse.Inbox != 2 {
		t.Errorf("Inbox = %d, want parent and nested child exactly once", pulse.Inbox)
	}
	if pulse.Today != 1 {
		t.Errorf("Today = %d, want only the eligible loaded planning identity", pulse.Today)
	}
	if pulse.Overdue != 1 {
		t.Errorf("Overdue = %d, want only the actionable overdue task", pulse.Overdue)
	}
}

func TestWorkspacePulsePreservesPartialLoadResults(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	loaded := primaryRowsProject("loaded", primaryRowsTask("loaded-inbox", models.StateInbox))
	failed := primaryRowsProject("failed", primaryRowsTask("failed-inbox", models.StateInbox))
	runtime := testConfigWithProjects(loaded, failed)
	base := installPulseRepository(t, loaded, failed)
	storage.DefaultRepository = &selectivePulseRepository{MockRepository: base, failProjectID: failed.ID}

	pulse := ComputeWorkspacePulse(runtime, nil, now)
	if pulse.Inbox != 1 {
		t.Fatalf("Inbox = %d, want the successfully loaded project to survive a peer load failure", pulse.Inbox)
	}
}

func TestWorkspacePulseNilRuntimeAndRefreshSeam(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	if pulse := ComputeWorkspacePulse(nil, nil, now); pulse != (WorkspacePulse{}) {
		t.Fatalf("nil runtime pulse = %+v, want zero", pulse)
	}

	project := primaryRowsProject("refresh", primaryRowsTask("inbox", models.StateInbox))
	runtime := testConfigWithProjects(project)
	installPulseRepository(t, project)
	shell := &AppModel{runtime: runtime}
	shell.refreshWorkspacePulseAt(now)
	if shell.workspacePulse.Inbox != 1 {
		t.Fatalf("refreshed Inbox = %d, want 1", shell.workspacePulse.Inbox)
	}
}

func TestWorkspacePulseFormattingAlwaysIncludesToday(t *testing.T) {
	if got := formatWorkspacePulseComponents(WorkspacePulse{}, true, true); got != "workspace: today 0" {
		t.Fatalf("zero pulse = %q, want %q", got, "workspace: today 0")
	}
	got := formatWorkspacePulseComponents(WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1}, true, true)
	for _, want := range []string{"inbox 3", "today 2", "overdue 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatted pulse %q is missing %q", got, want)
		}
	}
}
