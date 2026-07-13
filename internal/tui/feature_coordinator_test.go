package tui

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// stubMutator is a hand-rolled FeatureMutator recording the last call and
// returning a canned result/error, so the base run() plumbing can be tested
// without the storage stack.
type stubMutator struct {
	result app.TaskResult
	err    error
	calls  int
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
func (s *stubMutator) CreateLinkedTask(app.LinkedTaskRequest) (app.LinkedTaskResult, error) {
	s.calls++
	return app.LinkedTaskResult{New: s.result}, s.err
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

type stubQuery struct{}

func (stubQuery) ListProjects() ([]app.ProjectSummary, error) { return nil, nil }
func (stubQuery) Project(string) (app.ProjectSummary, error)  { return app.ProjectSummary{}, nil }

func TestFeatureCoordinatorRunEmitsResult(t *testing.T) {
	stub := &stubMutator{result: app.TaskResult{Title: "Done thing", State: models.StateDone, Revision: "rev-1"}}
	features := newFeatureCoordinators(stub, stubQuery{})

	msg := features.Inbox.Complete("task-1")()
	result, ok := msg.(TransactionResultMsg)
	if !ok {
		t.Fatalf("expected TransactionResultMsg, got %T", msg)
	}
	if result.Feature != "inbox" || result.Action != "Complete" {
		t.Fatalf("unexpected envelope: feature=%q action=%q", result.Feature, result.Action)
	}
	if result.Result.Revision != "rev-1" {
		t.Fatalf("result revision = %q", result.Result.Revision)
	}
	if stub.calls != 1 {
		t.Fatalf("mutator called %d times, want 1", stub.calls)
	}
}

func TestFeatureCoordinatorRunEmitsError(t *testing.T) {
	sentinel := errors.New("boom")
	stub := &stubMutator{err: sentinel}
	features := newFeatureCoordinators(stub, stubQuery{})

	msg := features.ProjectTree.SetPriority("task-1", 2)()
	failure, ok := msg.(TransactionErrorMsg)
	if !ok {
		t.Fatalf("expected TransactionErrorMsg, got %T", msg)
	}
	if !errors.Is(failure.Err, sentinel) {
		t.Fatalf("error = %v, want sentinel", failure.Err)
	}
	if failure.Feature != "project-tree" || failure.Action != "Set Priority" {
		t.Fatalf("unexpected envelope: feature=%q action=%q", failure.Feature, failure.Action)
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
