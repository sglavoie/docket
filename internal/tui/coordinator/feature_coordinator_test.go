package coordinator

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// stubMutator is a hand-rolled FeatureMutator recording the last call and
// returning a canned result/error, so the base run() plumbing can be tested
// without the storage stack.
type stubMutator struct {
	result app.TaskResult
	err    error
	calls  int
	// project is what ProjectAtRevision hands back, paired with result.Revision.
	project *models.Project
	// outline records the last outline plan the coordinator submitted.
	outline app.OutlinePlanRequest
}

func (s *stubMutator) Capture(app.CaptureRequest) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) CaptureMany([]app.CaptureRequest) ([]app.TaskResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return []app.TaskResult{s.result}, nil
}
func (s *stubMutator) AddTaskWithChildren(app.AddTaskRequest) (app.FormCreateResult, error) {
	s.calls++
	return app.FormCreateResult{Task: s.result}, s.err
}
func (s *stubMutator) Edit(app.EditRequest) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) Transition(app.StateRequest) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) Complete(string) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) SetPriority(string, int) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) ApplyOutlinePlan(req app.OutlinePlanRequest) (app.OutlinePlanResult, error) {
	s.calls++
	s.outline = req
	if s.err != nil {
		return app.OutlinePlanResult{}, s.err
	}
	return app.OutlinePlanResult{Updated: []app.TaskResult{s.result}}, nil
}
func (s *stubMutator) ProjectAtRevision(string) (*models.Project, string, error) {
	s.calls++
	return s.project, s.result.Revision, s.err
}

func TestFeatureCoordinatorRunEmitsResult(t *testing.T) {
	stub := &stubMutator{result: app.TaskResult{Title: "Done thing", Revision: "rev-1"}}
	features := NewFeatureCoordinators(stub)

	msg := features.Capture.Capture(app.CaptureRequest{ProjectID: "project", Title: "Done thing"})()
	result, ok := msg.(TransactionResultMsg)
	if !ok {
		t.Fatalf("expected TransactionResultMsg, got %T", msg)
	}
	if result.Action != "Capture" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
	if result.Result.Revision != "rev-1" {
		t.Fatalf("result revision = %q", result.Result.Revision)
	}
	if stub.calls != 1 {
		t.Fatalf("mutator called %d times, want 1", stub.calls)
	}
}

func TestCaptureCoordinatorsReportPlanningChanges(t *testing.T) {
	for _, destination := range []app.CaptureDestination{
		app.CaptureDestinationBacklog,
		app.CaptureDestinationToday,
		app.CaptureDestinationNext,
	} {
		destination := destination
		t.Run(string(destination), func(t *testing.T) {
			stub := &stubMutator{result: app.TaskResult{ProjectID: "project", TaskID: "task"}}
			features := NewFeatureCoordinators(stub)
			want := destination.IsPlanning()

			transaction, ok := features.Capture.Capture(app.CaptureRequest{
				ProjectID: "project", Title: "Task", Destination: destination,
			})().(TransactionResultMsg)
			if !ok || transaction.PlanningChanged != want {
				t.Fatalf("Capture message = %#v, want PlanningChanged=%v", transaction, want)
			}

			created, ok := features.Capture.CaptureCreate(app.CaptureRequest{
				ProjectID: "project", Title: "Task", Destination: destination,
			}, false)().(TaskCreatedMsg)
			if !ok || created.PlanningChanged != want {
				t.Fatalf("CaptureCreate message = %#v, want PlanningChanged=%v", created, want)
			}

			formCreated, ok := features.TaskForm.CreateForm(app.AddTaskRequest{
				ProjectID: "project", Title: "Task", Destination: destination,
			}, -1, false)().(TaskCreatedMsg)
			if !ok || formCreated.PlanningChanged != want {
				t.Fatalf("CreateForm message = %#v, want PlanningChanged=%v", formCreated, want)
			}
		})
	}

	stub := &stubMutator{result: app.TaskResult{ProjectID: "project", TaskID: "task"}}
	features := NewFeatureCoordinators(stub)
	bulk, ok := features.Capture.CaptureBulk([]app.CaptureRequest{
		{ProjectID: "project", Title: "Backlog", Destination: app.CaptureDestinationBacklog},
		{ProjectID: "project", Title: "Next", Destination: app.CaptureDestinationNext},
	}, true)().(TaskCreatedMsg)
	if !ok || !bulk.PlanningChanged {
		t.Fatalf("mixed CaptureBulk message = %#v, want planning refresh", bulk)
	}
}

func TestFeatureCoordinatorRunEmitsError(t *testing.T) {
	sentinel := errors.New("boom")
	stub := &stubMutator{err: sentinel}
	features := NewFeatureCoordinators(stub)

	msg := features.Capture.Capture(app.CaptureRequest{ProjectID: "project", Title: "Rejected"})()
	failure, ok := msg.(TransactionErrorMsg)
	if !ok {
		t.Fatalf("expected TransactionErrorMsg, got %T", msg)
	}
	if !errors.Is(failure.Err, sentinel) {
		t.Fatalf("error = %v, want sentinel", failure.Err)
	}
}

// fixedCompletionActivityClock pins the service's notion of "today" so a
// same-day completion is a fact of the fixture rather than of the wall clock.
type fixedCompletionActivityClock struct{ now time.Time }

func (c fixedCompletionActivityClock) Now() time.Time { return c.now }

// installActivityRepository swaps the process-wide repository for an in-memory
// mock holding the given projects, restoring the previous one on cleanup, so
// the service's snapshot load never touches a real workspace.
func installActivityRepository(t *testing.T, projects ...*models.Project) {
	t.Helper()
	repository := testutil.NewMockRepository()
	for _, project := range projects {
		repository.Projects[project.ID] = project
	}
	previous := storage.DefaultRepository
	storage.DefaultRepository = repository
	t.Cleanup(func() { storage.DefaultRepository = previous })
}

// activityRepo is the minimum app.Repository the completion-activity query
// needs: one snapshot over the installed mock repository. Nothing here commits,
// so Commit only has to satisfy the interface.
type activityRepo struct{ projects []*models.Project }

func (r *activityRepo) Snapshot() (app.Snapshot, error) {
	revisions := make(map[string]string, len(r.projects))
	documents := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, project := range r.projects {
		if err := storage.LoadProject(project); err != nil {
			return nil, err
		}
		doc, err := storage.LoadProjectDocument(project)
		if err != nil {
			return nil, err
		}
		revisions[project.ID] = doc.Revision
		documents[project.ID] = doc
	}
	return app.NewSnapshot(r.projects, revisions, documents), nil
}

func (r *activityRepo) Commit(...storage.ChangeSet) ([]*storage.CommitResult, error) {
	return nil, errors.New("activityRepo does not commit")
}

// The activity reader is acquired by a runtime type assertion over the mutation
// surface, so the construction seam is where a lost reader would go unnoticed.
func TestNewFeatureCoordinatorsBindsActivityReaderOrReportsItMissing(t *testing.T) {
	now := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	done := &models.Task{ID: "done-today", Title: "done-today", WorkflowState: models.StateDone, UpdatedAt: now}
	done.CompletedAt = &now
	project := &models.Project{ID: "activity-wiring", Name: "activity-wiring", Slug: "activity-wiring", Tasks: []*models.Task{done}}
	project.BuildTaskTree()
	installActivityRepository(t, project)
	service := app.NewServiceWithArtifacts(&activityRepo{projects: []*models.Project{project}}, fixedCompletionActivityClock{now: now}, nil, nil)

	activity, err := NewFeatureCoordinators(service).Activity.CompletionActivity()
	if err != nil {
		t.Fatalf("activity query over *app.Service: %v", err)
	}
	if activity.CompletedToday() != 1 || !activity.HasHistory() {
		t.Fatalf("activity = %+v, want the service-computed same-day completion", activity)
	}

	_, err = NewFeatureCoordinators(&stubMutator{}).Activity.CompletionActivity()
	if !errors.Is(err, errCompletionActivityReaderUnavailable) {
		t.Fatalf("mutator-only coordinator error = %v, want %v", err, errCompletionActivityReaderUnavailable)
	}
}

func TestActivityCoordinatorRejectsNilReader(t *testing.T) {
	if _, err := (ActivityCoordinator{}).CompletionActivity(); !errors.Is(err, errCompletionActivityReaderUnavailable) {
		t.Fatalf("nil reader error = %v, want %v", err, errCompletionActivityReaderUnavailable)
	}
}

func TestTransactionResultSuccessMessage(t *testing.T) {
	cases := []struct {
		name string
		msg  TransactionResultMsg
		want string
	}{
		{"with title", TransactionResultMsg{Action: "Capture", Result: app.TaskResult{Title: "Ship it"}}, "Capture: Ship it"},
		{"without title", TransactionResultMsg{Action: "Complete"}, "Complete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.msg.SuccessMessage(); got != tc.want {
				t.Fatalf("SuccessMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
