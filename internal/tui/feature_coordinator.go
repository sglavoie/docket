package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
)

// This file introduces the feature-coordinator seam for the primary TUI views.
//
// Historically each primary view (project selection, project tree,
// outliner, quick capture, the system Inbox, and the task forms) mutated its own
// copy of the domain model and then called a Persister.Save* helper directly.
// That coupled presentation to persistence: every view re-implemented the
// load/mutate/save dance, and there was no single place a mutation's durable
// outcome flowed back through.
//
// A feature coordinator collapses that into one seam per feature. A coordinator
// holds a narrow application-mutation surface (FeatureMutator), satisfied by
// *app.Service, and exposes exactly the operations its view performs. A mutation is dispatched as
// a tea.Cmd that runs the application use case (committing one atomic ChangeSet)
// and resolves to a TransactionResultMsg carrying the presentation-neutral
// outcome. The view re-renders from that message instead of persisting a model
// copy itself. This is the seam the serial feature-package migrations build the
// rest of their view construction and mutation routing onto.

// FeatureMutator is the narrow application-mutation surface the primary feature
// coordinators depend on. *app.Service satisfies it. Coordinators take the
// interface, not the concrete service, so a scenario test can drive them against
// an in-memory application service with the same use-case semantics.
type FeatureMutator interface {
	// Capture creates a new task in a project (Quick Capture, the Inbox capture
	// flow, and the create task form).
	Capture(req app.CaptureRequest) (app.TaskResult, error)
	// CaptureMany creates routed tasks across one or more projects as one
	// transaction (multiline capture).
	CaptureMany(reqs []app.CaptureRequest) ([]app.TaskResult, error)
	// AddTaskWithChildren creates a parent task plus subtasks in one ChangeSet
	// (the create task form).
	AddTaskWithChildren(req app.AddTaskRequest) (app.FormCreateResult, error)
	// Edit applies a full or partial edit to a task (the edit task form).
	Edit(req app.EditRequest) (app.TaskResult, error)
	// ApplyOutlinePlan commits one edited outline buffer as one transaction (the
	// external outline editor).
	ApplyOutlinePlan(req app.OutlinePlanRequest) (app.OutlinePlanResult, error)
	// ProjectAtRevision returns a project and the revision it was loaded at from
	// one snapshot, so the outline editor dumps the same tree its buffer claims.
	ProjectAtRevision(projectID string) (*models.Project, string, error)
}

// CompletionActivityReader is the read-only application query used by the
// Today title. It stays separate from FeatureMutator so mutation-only test
// doubles and feature adapters do not acquire an unrelated read obligation.
type CompletionActivityReader interface {
	CompletionActivity() (app.CompletionActivity, error)
}

// Compile-time assurance that the application service satisfies the mutation
// surface, so a production coordinator can be built from one *app.Service.
var (
	_ FeatureMutator           = (*app.Service)(nil)
	_ CompletionActivityReader = (*app.Service)(nil)
)

var errCompletionActivityReaderUnavailable = errors.New("completion activity reader is unavailable")

// TransactionResultMsg is emitted when a coordinator mutation commits. It
// carries the presentation-neutral outcome (the committed revision, the affected
// task's qualified reference and final workflow state) so a primary view renders
// from the transaction result rather than re-persisting its own model. Routing
// save completion through this message is what removes the load/mutate/save
// duplication from the migrated views.
type TransactionResultMsg struct {
	// Feature names the coordinator that produced the mutation (capture, inbox,
	// project-tree, outliner, task-form).
	// Action is the human-facing label for the mutation (e.g. "Capture",
	// "Complete", "Set Priority").
	Action string
	// Result is the application layer's outcome for the affected task.
	Result app.TaskResult
	// PlanningChanged reports that the same ChangeSet carried a checked planning
	// artifact. The shell advances its planning snapshot before refreshing the Outliner.
	PlanningChanged bool
	// outlinerRelocation is an internal navigation capture attached only to a
	// successful operation that may move the followed task between rows.
	outlinerRelocation *outlinerRelocationCapture
}

// SuccessMessage renders the status line shown when the transaction commits.
func (m TransactionResultMsg) SuccessMessage() string {
	title := m.Result.Title
	if title == "" {
		return m.Action
	}
	return m.Action + ": " + title
}

// TransactionErrorMsg is emitted when a coordinator mutation fails before or at
// commit. Because the application layer aborts a rejected or stale commit before
// persisting anything, the view keeps its pre-mutation model — the mutation left
// no partial write to reconcile.
type TransactionErrorMsg struct {
	Err error
}

// FeatureCoordinator is the shared base for the primary feature coordinators. It
// binds a feature name to the application-mutation surface each coordinator's
// commands dispatch through.
type FeatureCoordinator struct {
	name    string
	mutator FeatureMutator
}

// newFeatureCoordinator binds a named coordinator to the application surface.
func newFeatureCoordinator(name string, mutator FeatureMutator) FeatureCoordinator {
	return FeatureCoordinator{name: name, mutator: mutator}
}

// Name returns the coordinator's feature name.
func (c FeatureCoordinator) Name() string { return c.name }

// --- Primary feature coordinators ------------------------------------------

// CaptureCoordinator backs Quick Capture and the system Inbox capture flow.
type CaptureCoordinator struct{ FeatureCoordinator }

// Capture creates a task in the target project and routes the outcome through a
// transaction-result message.
func (c CaptureCoordinator) Capture(req app.CaptureRequest) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.Capture(req)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return TransactionResultMsg{
			Action: "Capture", Result: res,
			PlanningChanged: req.Destination.IsPlanning(),
		}
	}
}

// CaptureCreate runs the capture use case for a stack capture view (Quick
// Capture) and emits a TaskCreatedMsg carrying the committed application result
// plus the open-after flags, so the shell reloads the committed project,
// re-resolves the task for cursor navigation and open-after, and shows the
// creation status. A rejected capture surfaces an
// ErrorMsg, leaving the capture view open with the pre-mutation input.
func (c CaptureCoordinator) CaptureCreate(req app.CaptureRequest, openContent bool) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.Capture(req)
		if err != nil {
			return status.ErrorMsg{Err: err}
		}
		result := res
		return TaskCreatedMsg{
			Result: &result, OpenContent: openContent,
			PlanningChanged: req.Destination.IsPlanning(),
		}
	}
}

// CaptureBulk runs the multi-task capture use case for Bulk Capture. On success it
// emits a TaskCreatedMsg (which pops+refreshes) when closeOnSuccess is set — the
// whole batch was valid — or a SaveCompleteMsg (refresh, keep the view) when some
// input lines failed to parse and remain for the user to fix. A commit failure
// surfaces an ErrorMsg.
func (c CaptureCoordinator) CaptureBulk(reqs []app.CaptureRequest, closeOnSuccess bool) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		results, err := mutator.CaptureMany(reqs)
		if err != nil {
			return status.ErrorMsg{Err: err}
		}
		if closeOnSuccess {
			msg := TaskCreatedMsg{
				CreatedCount:    len(results),
				PlanningChanged: captureRequestsChangePlanning(reqs),
			}
			if len(results) > 0 {
				result := results[0]
				msg.Result = &result
			}
			return msg
		}
		return status.SaveCompleteMsg{}
	}
}

// TaskFormCoordinator backs the expanded create form.
type TaskFormCoordinator struct{ FeatureCoordinator }

// CreateForm runs the task-form create use case (parent + optional subtasks +
// any template artifact) and emits a TaskCreatedMsg carrying the committed
// parent result, the open-after target, and the open flags. openSubtaskIdx
// selects which created subtask (index into the committed subtask results) to
// navigate to after the reload; -1 targets the parent. A rejected create
// surfaces an ErrorMsg, leaving the form open with the pre-mutation draft.
func (c TaskFormCoordinator) CreateForm(req app.AddTaskRequest, openSubtaskIdx int, openContent bool) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.AddTaskWithChildren(req)
		if err != nil {
			return status.ErrorMsg{Err: err}
		}
		result := res.Task
		openTaskID := ""
		if openSubtaskIdx >= 0 && openSubtaskIdx < len(res.Subtasks) {
			openTaskID = res.Subtasks[openSubtaskIdx].TaskID
		}
		return TaskCreatedMsg{
			Result: &result, OpenTaskID: openTaskID, OpenContent: openContent,
			PlanningChanged: req.Destination.IsPlanning(),
		}
	}
}

func captureRequestsChangePlanning(reqs []app.CaptureRequest) bool {
	for _, req := range reqs {
		if req.Destination.IsPlanning() {
			return true
		}
	}
	return false
}

// OutlineCoordinator backs the external outline editor.
//
// Its two methods are synchronous rather than tea.Cmd-returning because the
// commit runs inside the tea.ExecProcess callback that closes the editor: it has
// to happen on the buffer that just came back, and the adapter needs the typed
// outcome inline to decide between a success status and a kept-buffer refusal.
type OutlineCoordinator struct{ FeatureCoordinator }

// Project loads the project to dump and the revision it was loaded at.
func (c OutlineCoordinator) Project(projectID string) (*models.Project, string, error) {
	return c.mutator.ProjectAtRevision(projectID)
}

// Apply commits one edited outline buffer as one transaction.
func (c OutlineCoordinator) Apply(req app.OutlinePlanRequest) (app.OutlinePlanResult, error) {
	return c.mutator.ApplyOutlinePlan(req)
}

// ActivityCoordinator carries the completion-history query through the same
// application boundary as primary feature mutations.
type ActivityCoordinator struct {
	reader CompletionActivityReader
}

func (c ActivityCoordinator) CompletionActivity() (app.CompletionActivity, error) {
	if c.reader == nil {
		return app.CompletionActivity{}, errCompletionActivityReaderUnavailable
	}
	return c.reader.CompletionActivity()
}

// FeatureCoordinators aggregates the primary-view coordinators the AppModel
// hands to feature views as they migrate onto the application layer.
type FeatureCoordinators struct {
	Capture  CaptureCoordinator
	TaskForm TaskFormCoordinator
	Outline  OutlineCoordinator
	Activity ActivityCoordinator
}

// newFeatureCoordinators builds the primary coordinators over a shared
// application mutation surface.
func newFeatureCoordinators(mutator FeatureMutator) FeatureCoordinators {
	var activity ActivityCoordinator
	if reader, ok := mutator.(CompletionActivityReader); ok {
		activity.reader = reader
	}
	return FeatureCoordinators{
		Capture:  CaptureCoordinator{newFeatureCoordinator("capture", mutator)},
		TaskForm: TaskFormCoordinator{newFeatureCoordinator("task-form", mutator)},
		Outline:  OutlineCoordinator{newFeatureCoordinator("outline", mutator)},
		Activity: activity,
	}
}
