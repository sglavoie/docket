package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// The coordinator package keeps its own copy of this double for its unit tests.
// A test helper cannot cross a package boundary, and these shell surfaces drive
// a coordinator built over a stub rather than the storage stack, so the double
// is declared on both sides rather than one side reaching for the other.

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
func (s *stubMutator) CaptureManyCreatingProjects(_ app.ProjectCreation, reqs []app.CaptureRequest) ([]app.TaskResult, error) {
	return s.CaptureMany(reqs)
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
