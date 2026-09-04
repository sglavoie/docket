package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func commandMessage(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return operation.OperationCommandResultMsg{Cmd: cmd}
}

// coreNotesScope keeps Ctrl+N on All Notes everywhere except a project header
// row, which names one project explicitly rather than incidentally — the same
// exception coreCaptureMessage makes, read through the same predicate, so no
// task cursor can narrow the manager. The header is re-resolved against the
// shell's note-owner population, since an archived project the archived facet
// loaded is never an owner and would only bounce back to All Notes.
func (m *AppModel) coreNotesScope() route.NotesScope {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return route.AllNotesScope()
	}
	header := out.cursorProjectHeaderProject()
	if header == nil || m.activeProjectByID(header.ID) == nil {
		return route.AllNotesScope()
	}
	return route.ProjectNotesScope(header.ID)
}

// coreCaptureMessage routes base capture to the workspace-owned System Inbox
// and project-focus capture to that focused project. A task Focus wins first
// and captures a direct child of the focus root, which is an explicit
// projection anchor pushed by Enter rather than the cursor. At the nesting cap
// it degrades to plain capture into the focus root's project. Cursor project
// identity is deliberately ignored at base scope, with one exception: a project
// header row names a project explicitly rather than incidentally, so capture
// there lands in it. cursorProjectHeaderProject returns nil on every task row,
// so the invariant holds verbatim everywhere else.
func (m *AppModel) coreCaptureMessage() tea.Msg {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return m.captureComposerMessage(nil, nil)
	}
	var project *models.Project
	if identity, focused := out.focusRootIdentity(); focused {
		if task, focusProject := m.taskAndProjectForIdentity(identity); task != nil && focusProject != nil {
			if taskAcceptsDeeperChild(task) {
				return ShowTaskComposerMsg{Preset: m.focusChildComposerPreset(focusProject, task)}
			}
			project = focusProject
		}
	}
	if project == nil {
		switch {
		case out.project != nil:
			project = out.project
		case out.isProjectScoped():
			project = out.scopedProject()
		}
	}
	if project == nil {
		project = out.cursorProjectHeaderProject()
	}
	if project == nil {
		// Resolve through RuntimeState's canonical workspace-owned Unfiled
		// reference, then prefer the shell's loaded project pointer when it is
		// available so capture never borrows the base cursor's project.
		var unfiled *models.Project
		if m.runtime != nil {
			unfiled = m.runtime.SystemUnfiledProject()
			if unfiled != nil {
				if loaded := m.projectByID(unfiled.ID); loaded != nil {
					unfiled = loaded
				}
			} else if id := m.runtime.Catalog().SystemUnfiledProjectID; id != "" {
				unfiled = m.projectByID(id)
			}
		}
		return m.captureComposerMessage(unfiled, m.availableProjects())
	}
	return m.captureComposerMessage(project, m.availableProjects())
}

func operationTargetLocators(targets operation.ResolvedOperationTargets) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			locators = append(locators, target.Task.Locator())
		}
	}
	return locators
}

func planningMemberLocators(members map[taskref.TaskIdentity]bool) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(members))
	for identity, member := range members {
		if member {
			locators = append(locators, identity.Locator())
		}
	}
	return locators
}

func (m *AppModel) corePlanningOperation(
	targets operation.ResolvedOperationTargets,
	action app.PlanningAction,
	capture *outlinerRelocationCapture,
) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	summary := fmt.Sprintf("Updated %d task(s)", len(locators))
	switch action {
	case app.PlanningAddToday:
		summary = fmt.Sprintf("Added %d task(s) to Today", len(locators))
	case app.PlanningRemoveToday:
		summary = fmt.Sprintf("Removed %d task(s) from Today", len(locators))
	case app.PlanningAddNext:
		summary = fmt.Sprintf("Moved %d task(s) to Next", len(locators))
	case app.PlanningRemoveNext:
		summary = fmt.Sprintf("Removed %d task(s) from Next", len(locators))
	case app.PlanningBacklog:
		summary = fmt.Sprintf("Moved %d task(s) to Backlog", len(locators))
	}
	return withOutlinerRelocationCapture(
		m.secondaryFeatures.Bulk.ApplyPlanningLocators(locators, action, summary),
		capture,
	)
}

func (m *AppModel) coreMoveToBacklogSingle(capture *outlinerRelocationCapture) tea.Cmd {
	task, project := m.outlinerTarget()
	if task == nil || project == nil {
		return nil
	}
	fresh, _, transactionID, err := m.applyTaskPlanningAction(task.ID, project.ID, app.PlanningBacklog)
	if err != nil {
		m.err = err
		return status.ErrorCmd("%s", err)
	}
	successMessage := fmt.Sprintf("Moved to Backlog: %s", taskTitleOrFallback(fresh, task))
	return m.finishSynchronousOutlinerRelocation(m.queueToggleResultCmd(), capture, successMessage, transactionID)
}

func (m *AppModel) coreToggleToday(targets operation.ResolvedOperationTargets, capture *outlinerRelocationCapture) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	action, err := app.ResolvePlanningToggleAction(
		app.PlanningHorizonToday,
		locators,
		planningMemberLocators(m.todayIDs),
	)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return m.corePlanningOperation(targets, action, capture)
}

func (m *AppModel) coreToggleNext(targets operation.ResolvedOperationTargets, capture *outlinerRelocationCapture) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	action, err := app.ResolvePlanningToggleAction(
		app.PlanningHorizonNext,
		locators,
		planningMemberLocators(m.nextOrderIDs),
	)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return m.corePlanningOperation(targets, action, capture)
}
