package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) handleQuickEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.quickEdit.SetContext(m.quickEdit.GetProject(), m.quickEditProjects())
	transition, result, cmd := m.quickEdit.Handle(msg)
	if transition != grammar.QuickEditCommit || result == nil {
		return m, cmd
	}
	project := m.quickEdit.GetProject()
	task := m.quickEdit.GetTask()
	if project == nil || task == nil || project.GetTaskByID(task.ID) != task {
		m.quickEdit.RejectCommit("task edit context is unavailable")
		return m, nil
	}
	source := taskref.For(task, project)
	relocation := m.captureOutlinerRelocation([]taskref.TaskIdentity{source})
	m.quickEdit.BeginCommit()
	return m, m.taskItem.EditTaskLine(m.taskLineEditRequest(source, result), relocation)
}

func (m *OutlinerModel) quickEditProjects() []*models.Project {
	if m.project != nil {
		return []*models.Project{m.project}
	}
	return m.loadedProjects
}

func taskLineEditRequest(source taskref.TaskIdentity, result *grammar.QuickEditResult) app.TaskLineEditRequest {
	req := app.TaskLineEditRequest{
		Source:               source.Locator(),
		Title:                result.Title,
		DueSet:               result.DueDateSet,
		DeferSet:             result.DeferSet,
		TagsToAdd:            append([]string(nil), result.TagsToAdd...),
		TagsToRemove:         append([]string(nil), result.TagsToRemove...),
		DestinationProjectID: source.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
	}
	if result.PrioritySet {
		priority := result.Priority
		req.Priority = &priority
	}
	if result.ContextSet {
		context := result.ContextValue
		if result.ContextClear {
			context = ""
		}
		req.Context = &context
	}
	if result.DueDateSet && !result.DueDateClear {
		req.Due = result.DueDate
	}
	if result.DeferSet && !result.DeferClear {
		req.Defer = result.DeferUntil
	}
	if result.DestinationSet {
		destination := result.Destination
		req.Destination = &destination
	}
	if result.PinnedSet {
		pinned := result.Pinned
		req.Pinned = &pinned
	}
	if result.ReviewSet {
		review := result.Review
		req.Review = &review
	}
	if result.TargetProject != nil {
		req.DestinationProjectID = result.TargetProject.ID
	}
	switch result.ParentMode {
	case grammar.QuickEditParentPreserve:
		// Preserve is already the request default.
	case grammar.QuickEditParentRoot:
		req.Parent.Mode = app.TaskLineParentRoot
	case grammar.QuickEditParentSet:
		req.Parent.Mode = app.TaskLineParentSet
		if result.Parent != nil {
			req.Parent.Parent = app.TaskLocator{
				ProjectID: req.DestinationProjectID,
				TaskID:    result.Parent.ID,
			}
		}
	}
	// A dependency locator is fully qualified at parse time and is independent of
	// where the edited task lands, so a cross-project move carries it unchanged.
	req.Dependencies = result.DependencyPatch()
	// Children resolve in the effective project, which is the destination
	// whenever the edit also crosses a boundary.
	for _, child := range result.Children {
		req.Children = append(req.Children, app.TaskLocator{
			ProjectID: req.DestinationProjectID,
			TaskID:    child.ID,
		})
	}
	return req
}

// taskLineEditRequest binds every live inline edit to an explicit planning
// builder instead of inheriting whichever default a prior service call left
// installed. Cross-project edits carry a complete planning-move delta, so they
// use the structural builder that reconciles against the newest on-disk state.
func (m *OutlinerModel) taskLineEditRequest(source taskref.TaskIdentity, result *grammar.QuickEditResult) app.TaskLineEditRequest {
	req := taskLineEditRequest(source, result)
	req.PlanningArtifacts = shellPlanningReconcileArtifactBuilder(m.runtime, m.configManager)
	if req.DestinationProjectID != source.ProjectID {
		req.PlanningArtifacts = shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager)
	}
	return req
}

// escSteps defines the Esc dismissal cascade for Outliner, narrowest to widest:
// Peek → active range → frozen selection → search query → active lens →
// Focus stack → cross-project Focus stack → section zoom → project scope.
func (m *OutlinerModel) escSteps() []listsurface.EscStep {
	return []listsurface.EscStep{
		{Active: func() bool { return m.peekMode }, Clear: func() {
			m.peekMode = false
		}},
		{Active: func() bool { return m.IsRangeSelectionActive() }, Clear: func() {
			m.FreezeRangeSelection()
			m.updateVisualSelectionStatus()
		}},
		{Active: func() bool { return m.MultiSelectMode }, Clear: func() {
			m.toggleMultiSelectMode()
		}},
		{Active: func() bool { return m.hasSearchProjection() }, Clear: func() {
			m.ClearSearch()
		}},
		// A lens is one atomic context over the mode, facets, sort, query, and
		// population, so it unwinds before the narrowings taken inside it: leaving
		// it restores the focus, zoom, and scope the user had anyway, and stepping
		// through those first would unwind state the lens is about to replace.
		{Active: func() bool { return m.hasActiveLens() }, Clear: func() {
			m.restoreLensBaseline()
		}},
		{Active: func() bool { return len(m.focusStack) > 0 }, Clear: func() {
			m.popFocusLevel()
		}},
		{Active: func() bool { return len(m.crossFocusStack) > 0 }, Clear: func() {
			m.popFocusLevel()
		}},
		// A zoom narrows one mode, so it unwinds after task focus and before the
		// project scope that owns the mode.
		{Active: func() bool { return m.zoom.active() }, Clear: func() {
			m.exitSectionZoom()
		}},
		// Task focus and section zoom both unwind before project scope: Esc pops
		// one scope frame, restoring the previous scope's mode, filter, cursor,
		// scroll, and zoom.
		{Active: func() bool { return m.isProjectScoped() }, Clear: func() {
			m.popProjectScope()
		}},
	}
}

func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() ||
		m.SearchShouldCaptureEsc() ||
		m.MultiSelectMode ||
		m.quickEdit.IsActive() ||
		m.queryReadout.expanded ||
		// An open menu is the innermost thing Esc can put down, so it has to claim
		// the key before the shell's back cascade spends it unwinding a scope the
		// user cannot even see the rows of.
		m.menuBar.open ||
		listsurface.AnyEscStepActive(m.escSteps())
}
func (m *OutlinerModel) HasTextInput() bool {
	return m.DefaultHasTextInput() || m.quickEdit.IsActive()
}
