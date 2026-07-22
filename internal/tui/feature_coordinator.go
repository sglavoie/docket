package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// This file introduces the feature-coordinator seam for the primary TUI views.
//
// Historically each primary view (Dashboard, project selection, project tree,
// outliner, quick capture, the system Inbox, and the task forms) mutated its own
// copy of the domain model and then called a Persister.Save* helper directly.
// That coupled presentation to persistence: every view re-implemented the
// load/mutate/save dance, and there was no single place a mutation's durable
// outcome flowed back through.
//
// A feature coordinator collapses that into one seam per feature. A coordinator
// holds a narrow application-mutation surface (FeatureMutator) and a narrow
// read-model surface (FeatureQuery) — both satisfied by *app.Service — and
// exposes exactly the operations its view performs. A mutation is dispatched as
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
	// CaptureMany creates several tasks in one project as one ChangeSet (Bulk
	// Capture).
	CaptureMany(reqs []app.CaptureRequest) ([]app.TaskResult, error)
	// CreateLinkedTask creates a task related to an existing source task (the
	// Linked Task Creator).
	CreateLinkedTask(req app.LinkedTaskRequest) (app.LinkedTaskResult, error)
	// AddTaskWithChildren creates a parent task plus subtasks in one ChangeSet
	// (the create task form).
	AddTaskWithChildren(req app.AddTaskRequest) (app.FormCreateResult, error)
	// Edit applies a full or partial edit to a task (the edit task form).
	Edit(req app.EditRequest) (app.TaskResult, error)
	// Transition moves a task to a workflow state (Inbox triage, tree/outliner
	// state changes).
	Transition(req app.StateRequest) (app.TaskResult, error)
	// Complete terminates a task as done, running the recurrence hooks.
	Complete(taskID string) (app.TaskResult, error)
	// SetPriority sets a task's 1..5 priority.
	SetPriority(taskID string, priority int) (app.TaskResult, error)
}

// FeatureQuery is the narrow read-model surface the query-only primary
// coordinators (Dashboard, project selection) depend on. *app.Service satisfies
// it. It never commits: every method loads a revision-consistent snapshot and
// returns presentation-neutral summaries.
type FeatureQuery interface {
	// ListProjects returns a summary of every project in the workspace.
	ListProjects() ([]app.ProjectSummary, error)
	// Project returns the summary for a single project.
	Project(projectID string) (app.ProjectSummary, error)
}

// Compile-time assurance that the application service satisfies both feature
// surfaces, so a production coordinator can be built from one *app.Service.
var (
	_ FeatureMutator = (*app.Service)(nil)
	_ FeatureQuery   = (*app.Service)(nil)
)

// TransactionResultMsg is emitted when a coordinator mutation commits. It
// carries the presentation-neutral outcome (the committed revision, the affected
// task's qualified reference and final workflow state) so a primary view renders
// from the transaction result rather than re-persisting its own model. Routing
// save completion through this message is what removes the load/mutate/save
// duplication from the migrated views.
type TransactionResultMsg struct {
	// Feature names the coordinator that produced the mutation (dashboard,
	// capture, inbox, project-tree, outliner, task-form).
	Feature string
	// Action is the human-facing label for the mutation (e.g. "Capture",
	// "Complete", "Set Priority").
	Action string
	// Result is the application layer's outcome for the affected task.
	Result app.TaskResult
	// PlanningChanged reports that the same ChangeSet carried a checked planning
	// artifact. The shell advances its planning snapshot before refreshing Planner.
	PlanningChanged bool
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
	Feature string
	Action  string
	Err     error
	// DeleteRequestToken correlates a rejected canonical delete with the Inbox /
	// Weekly Review post-commit bookkeeping it armed, so only the exact request's
	// failure drops that in-flight state. Zero for every non-delete rejection (and
	// for deletes that armed no bookkeeping); the correlation guards ignore zero.
	DeleteRequestToken uint64
}

// FeatureCoordinator is the shared base for the primary feature coordinators. It
// binds a feature name to the application-mutation and read-model surfaces and
// provides the single command runner every mutation flows through.
type FeatureCoordinator struct {
	name    string
	mutator FeatureMutator
	query   FeatureQuery
}

// newFeatureCoordinator binds a named coordinator to the shared application
// surfaces.
func newFeatureCoordinator(name string, mutator FeatureMutator, query FeatureQuery) FeatureCoordinator {
	return FeatureCoordinator{name: name, mutator: mutator, query: query}
}

// Name returns the coordinator's feature name.
func (c FeatureCoordinator) Name() string { return c.name }

// run wraps an application mutation as an async tea.Cmd that resolves to a
// transaction-result message. It is the single seam through which a primary
// view's mutation reaches persistence: the view dispatches the command and
// re-renders from the returned TransactionResultMsg, never mutating and saving a
// model copy itself. A use-case error resolves to a TransactionErrorMsg, which
// leaves the pre-mutation model untouched.
func (c FeatureCoordinator) run(action string, mutate func(FeatureMutator) (app.TaskResult, error)) tea.Cmd {
	name := c.name
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutate(mutator)
		if err != nil {
			return TransactionErrorMsg{Feature: name, Action: action, Err: err}
		}
		return TransactionResultMsg{Feature: name, Action: action, Result: res}
	}
}

// --- Primary feature coordinators ------------------------------------------

// CaptureCoordinator backs Quick Capture and the system Inbox capture flow.
type CaptureCoordinator struct{ FeatureCoordinator }

// Capture creates a task in the target project and routes the outcome through a
// transaction-result message.
func (c CaptureCoordinator) Capture(req app.CaptureRequest) tea.Cmd {
	return c.run("Capture", func(m FeatureMutator) (app.TaskResult, error) {
		return m.Capture(req)
	})
}

// CaptureCreate runs the capture use case for a stack capture view (Quick
// Capture) and emits a TaskCreatedMsg carrying the committed application result
// plus the open-after flags, so the shell reloads the committed project,
// re-resolves the task for cursor navigation and open-after, and shows the
// "Created in <bucket>" / onboarding status. A rejected capture surfaces an
// ErrorMsg, leaving the capture view open with the pre-mutation input.
func (c CaptureCoordinator) CaptureCreate(req app.CaptureRequest, openContent, openDetail bool) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.Capture(req)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		result := res
		return TaskCreatedMsg{Result: &result, OpenContent: openContent, OpenDetail: openDetail}
	}
}

// CaptureBoard runs the capture use case for the Focus Board quick-add and emits
// a transaction-result message whose status renders "Added #N: <title>", so the
// shell reloads the committed project and refreshes the board without popping it.
func (c CaptureCoordinator) CaptureBoard(req app.CaptureRequest) tea.Cmd {
	name := c.name
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.Capture(req)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return TransactionResultMsg{
			Feature: name, Action: fmt.Sprintf("Added #%d", res.ShortID), Result: res,
			PlanningChanged: req.PlanningArtifacts != nil,
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
			return ErrorMsg{Err: err}
		}
		if closeOnSuccess {
			msg := TaskCreatedMsg{CreatedCount: len(results)}
			if len(results) > 0 {
				result := results[0]
				msg.Result = &result
			}
			return msg
		}
		return SaveCompleteMsg{}
	}
}

type linkedTaskBatchMutator interface {
	CreateLinkedTasks([]app.LinkedTaskRequest) ([]app.LinkedTaskResult, error)
}

// CreateLinkedBulk commits every compact linked-task line and its relationship
// through one application-layer ChangeSet.
func (c CaptureCoordinator) CreateLinkedBulk(reqs []app.LinkedTaskRequest, linkType LinkType) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		batch, ok := mutator.(linkedTaskBatchMutator)
		if !ok {
			return ErrorMsg{Err: fmt.Errorf("linked-task batch mutation is unavailable")}
		}
		results, err := batch.CreateLinkedTasks(reqs)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return LinkedTaskBatchCreatedMsg{Results: results, LinkType: linkType}
	}
}

// CreateLinked runs the linked-task capture use case and emits a
// LinkedTaskCreatedMsg carrying the committed result, so the shell reloads the
// committed project, re-resolves both tasks, jumps/opens as configured, and shows
// the "Created <link> #N" status. A rejected creation surfaces an ErrorMsg.
func (c CaptureCoordinator) CreateLinked(req app.LinkedTaskRequest, linkType LinkType, openContent, openDetail bool) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.CreateLinkedTask(req)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		result := res
		return LinkedTaskCreatedMsg{Result: &result, LinkType: linkType, OpenContent: openContent, OpenDetail: openDetail}
	}
}

// InboxCoordinator backs the system Inbox and the inbox-processor triage flow.
type InboxCoordinator struct{ FeatureCoordinator }

// Triage moves an inbox task to the chosen workflow state.
func (c InboxCoordinator) Triage(taskID string, target models.WorkflowState) tea.Cmd {
	return c.run("Triage", func(m FeatureMutator) (app.TaskResult, error) {
		return m.Transition(app.StateRequest{TaskID: taskID, Target: target})
	})
}

// Complete terminates an inbox task as done.
func (c InboxCoordinator) Complete(taskID string) tea.Cmd {
	return c.run("Complete", func(m FeatureMutator) (app.TaskResult, error) {
		return m.Complete(taskID)
	})
}

// ProjectTreeCoordinator backs the project tree hierarchy editor.
type ProjectTreeCoordinator struct{ FeatureCoordinator }

// SetPriority sets a tree task's priority.
func (c ProjectTreeCoordinator) SetPriority(taskID string, priority int) tea.Cmd {
	return c.run("Set Priority", func(m FeatureMutator) (app.TaskResult, error) {
		return m.SetPriority(taskID, priority)
	})
}

// Transition moves a tree task to a workflow state.
func (c ProjectTreeCoordinator) Transition(taskID string, target models.WorkflowState) tea.Cmd {
	return c.run("Change State", func(m FeatureMutator) (app.TaskResult, error) {
		return m.Transition(app.StateRequest{TaskID: taskID, Target: target})
	})
}

// Complete terminates a tree task as done.
func (c ProjectTreeCoordinator) Complete(taskID string) tea.Cmd {
	return c.run("Complete", func(m FeatureMutator) (app.TaskResult, error) {
		return m.Complete(taskID)
	})
}

// OutlinerCoordinator backs the outliner hierarchy editor.
type OutlinerCoordinator struct{ FeatureCoordinator }

// SetPriority sets an outliner task's priority.
func (c OutlinerCoordinator) SetPriority(taskID string, priority int) tea.Cmd {
	return c.run("Set Priority", func(m FeatureMutator) (app.TaskResult, error) {
		return m.SetPriority(taskID, priority)
	})
}

// Complete terminates an outliner task as done.
func (c OutlinerCoordinator) Complete(taskID string) tea.Cmd {
	return c.run("Complete", func(m FeatureMutator) (app.TaskResult, error) {
		return m.Complete(taskID)
	})
}

// TaskFormCoordinator backs the create/edit task forms.
type TaskFormCoordinator struct{ FeatureCoordinator }

// CreateForm runs the task-form create use case (parent + optional subtasks +
// any template artifact) and emits a TaskCreatedMsg carrying the committed
// parent result, the open-after target, and the open flags. openSubtaskIdx
// selects which created subtask (index into the committed subtask results) to
// navigate to after the reload; -1 targets the parent. A rejected create
// surfaces an ErrorMsg, leaving the form open with the pre-mutation draft.
func (c TaskFormCoordinator) CreateForm(req app.AddTaskRequest, openSubtaskIdx int, openContent, openDetail bool) tea.Cmd {
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.AddTaskWithChildren(req)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		result := res.Task
		openTaskID := ""
		if openSubtaskIdx >= 0 && openSubtaskIdx < len(res.Subtasks) {
			openTaskID = res.Subtasks[openSubtaskIdx].TaskID
		}
		return TaskCreatedMsg{Result: &result, OpenTaskID: openTaskID, OpenContent: openContent, OpenDetail: openDetail}
	}
}

// EditForm runs the task-form edit use case for a qualified task (full field
// replacement) and emits a TaskUpdatedMsg carrying the committed result, so the
// shell reloads the project and re-resolves the live task pointer. A rejected
// edit surfaces an ErrorMsg, leaving the form open with the pre-mutation draft.
func (c TaskFormCoordinator) EditForm(task TaskIdentity, req app.EditRequest) tea.Cmd {
	req.TaskID = ""
	req.Locator = task.Locator()
	mutator := c.mutator
	return func() tea.Msg {
		res, err := mutator.Edit(req)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		result := res
		return TaskUpdatedMsg{Result: &result}
	}
}

// DashboardCoordinator backs the Dashboard's project overview. It is query-only:
// the Dashboard reads project summaries and never mutates through this seam.
type DashboardCoordinator struct{ FeatureCoordinator }

// Projects returns every project summary for the dashboard overview.
func (c DashboardCoordinator) Projects() ([]app.ProjectSummary, error) {
	return c.query.ListProjects()
}

// ProjectSelectionCoordinator backs project selection surfaces. Query-only.
type ProjectSelectionCoordinator struct{ FeatureCoordinator }

// Projects returns every project summary for selection.
func (c ProjectSelectionCoordinator) Projects() ([]app.ProjectSummary, error) {
	return c.query.ListProjects()
}

// Project returns a single project summary for selection.
func (c ProjectSelectionCoordinator) Project(projectID string) (app.ProjectSummary, error) {
	return c.query.Project(projectID)
}

// FeatureCoordinators aggregates the primary-view coordinators the AppModel
// hands to feature views as they migrate onto the application layer.
type FeatureCoordinators struct {
	Dashboard        DashboardCoordinator
	ProjectSelection ProjectSelectionCoordinator
	ProjectTree      ProjectTreeCoordinator
	Outliner         OutlinerCoordinator
	Capture          CaptureCoordinator
	Inbox            InboxCoordinator
	TaskForm         TaskFormCoordinator
}

// newFeatureCoordinators builds the primary coordinators over shared application
// surfaces. In production both arguments are the same *app.Service; a scenario
// test may inject distinct in-memory implementations to isolate behavior.
func newFeatureCoordinators(mutator FeatureMutator, query FeatureQuery) FeatureCoordinators {
	return FeatureCoordinators{
		Dashboard:        DashboardCoordinator{newFeatureCoordinator("dashboard", mutator, query)},
		ProjectSelection: ProjectSelectionCoordinator{newFeatureCoordinator("project-selection", mutator, query)},
		ProjectTree:      ProjectTreeCoordinator{newFeatureCoordinator("project-tree", mutator, query)},
		Outliner:         OutlinerCoordinator{newFeatureCoordinator("outliner", mutator, query)},
		Capture:          CaptureCoordinator{newFeatureCoordinator("capture", mutator, query)},
		Inbox:            InboxCoordinator{newFeatureCoordinator("inbox", mutator, query)},
		TaskForm:         TaskFormCoordinator{newFeatureCoordinator("task-form", mutator, query)},
	}
}
