package taskform

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

func TestTaskFormCreatePersistsCheckInScheduleWithoutChangingDue(t *testing.T) {
	env := newScenarioEnv(t, "create-check-in")
	m := scenarioCreateForm(t, env)
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	m.titleInput.SetValue("Scheduled review")
	m.draft.checkIn = models.CheckInSchedule{Next: &day, RepeatDays: 14}

	msg := runCreate(t, m)
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil || task.NextCheckIn == nil {
		t.Fatalf("created task schedule = %#v", task)
	}
	if !task.NextCheckIn.Equal(day) || task.CheckInRepeatDays != 14 {
		t.Fatalf("created check-in = %v/%d, want %s/14", task.NextCheckIn, task.CheckInRepeatDays, day)
	}
	if task.DueDate != nil {
		t.Fatalf("check-in creation invented due date %s", task.DueDate)
	}
}

func TestTaskFormCheckInTodayDoesNotInferTodayDestination(t *testing.T) {
	stub := &stubMutator{}
	m := newTestTaskForm()
	m.SetTaskFormCoordinator(coordinator.NewFeatureCoordinators(stub).TaskForm)
	m.titleInput.SetValue("Review today without planning today")
	today := time.Date(2026, 9, 12, 0, 0, 0, 0, time.Local)
	m.draft.checkIn = models.CheckInSchedule{Next: &today, RepeatDays: 7}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("save refused: %v", m.err)
	}
	cmd()
	if stub.add.Destination != app.CaptureDestinationBacklog {
		t.Fatalf("destination = %q, want Backlog", stub.add.Destination)
	}
	if stub.add.DueDate != nil || stub.add.CheckIn.Next == nil || !stub.add.CheckIn.Next.Equal(today) {
		t.Fatalf("request due/check-in = %v / %+v", stub.add.DueDate, stub.add.CheckIn)
	}
}

func TestComposerExpansionCarriesAnExplicitCheckInSchedule(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind: ComposerProject, Project: project, AllProjects: []*models.Project{project},
	}, runtime)
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	want := day
	request := app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Expanded schedule",
		CheckIn:   models.CheckInSchedule{Next: &day, RepeatDays: 30},
	}

	composer.ExpandRequest(request)
	if composer.Expanded == nil || composer.Expanded.draft.checkIn.Next == nil {
		t.Fatalf("expanded schedule = %#v", composer.Expanded)
	}
	got := composer.Expanded.draft.checkIn
	if !got.Next.Equal(want) || got.RepeatDays != 30 {
		t.Fatalf("expanded check-in = %+v", got)
	}
	*request.CheckIn.Next = request.CheckIn.Next.AddDate(0, 0, 1)
	if !composer.Expanded.draft.checkIn.Next.Equal(want) {
		t.Fatal("expanded form aliases the capture request's check-in date")
	}
}
