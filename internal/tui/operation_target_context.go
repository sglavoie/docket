package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// operationSelectionProvider exposes stable selected identities without
// leaking renderer selection maps into execution.
type operationSelectionProvider interface {
	SelectedTaskIdentities() []TaskIdentity
}

type operationVisualSelectionProvider interface {
	IsMultiSelectActive() bool
}

// operationCommandContext captures the active cursor, multi-selection, project
// scope, and entity state before an overlay (such as the palette) shadows the
// primary view.
func (m *AppModel) operationCommandContext() CommandContext {
	view := m.currentView()
	ctx := PresentationCommandContext(view)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	ctx.FocusedProjectID = m.operationFocusedProjectID()
	if review, ok := m.activeView().(*ReviewHubModel); ok {
		ctx = review.CommandContext()
	}
	if manage, ok := m.activeView().(*ManageHubModel); ok {
		ctx = manage.CommandContext()
	}

	task, project := m.getCurrentCursorTaskAndProject()
	if task != nil && project != nil {
		ctx.Entity = EntityContext{
			ProjectID: project.ID,
			TaskID:    task.ID,
			State:     string(task.State()),
			Mutable:   !task.IsTerminalState(),
			Archived:  project.Archived,
		}
	} else if m.currentProject != nil {
		ctx.Entity.ProjectID = m.currentProject.ID
		ctx.Entity.Archived = m.currentProject.Archived
		ctx.Entity.Mutable = !m.currentProject.Archived
	}

	if selected, ok := m.activeView().(operationSelectionProvider); ok {
		ctx.Selection.Tasks = selected.SelectedTaskIdentities()
	}
	if visual, ok := m.activeView().(operationVisualSelectionProvider); ok {
		ctx.Selection.Visual = visual.IsMultiSelectActive()
	}
	ctx.Selection.Entities = m.operationSelectionEntities(ctx.Selection.Tasks)
	ctx.Selection.EntityIdentities = taskEntityIdentities(ctx.Selection.Tasks)
	ctx.CurrentEntity = commandContextCurrentEntity(ctx.Entity)
	ctx.ScopeProjectIDs = m.operationScopeProjectIDs(project)
	return ctx
}

// operationFocusedProjectID reports the project the session is focused on. It
// never falls back to the cursor: base and multi-project scopes stay app-scoped
// even when the cursor happens to sit on a project's task.
func (m *AppModel) operationFocusedProjectID() string {
	if out := m.activeOutliner(); out != nil {
		if out.project != nil {
			return out.project.ID
		}
		return out.scopedProjectID
	}
	if m.currentProject != nil {
		return m.currentProject.ID
	}
	return ""
}

func (m *AppModel) operationSelectionEntities(identities []TaskIdentity) []EntityContext {
	normalized := normalizedTaskIdentities(identities)
	entities := make([]EntityContext, 0, len(normalized))
	for _, identity := range normalized {
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			continue
		}
		entities = append(entities, taskOperationEntity(task, project))
	}
	return entities
}

func (m *AppModel) operationScopeProjectIDs(cursorProject *models.Project) []string {
	if out := m.activeOutliner(); out != nil {
		projects := out.scopeProjects()
		ids := make([]string, 0, len(projects))
		for _, project := range projects {
			if project != nil && project.ID != "" {
				ids = append(ids, project.ID)
			}
		}
		return normalizedProjectIDs(ids)
	}
	if review, ok := m.activeView().(*ReviewHubModel); ok {
		return review.ScopeProjectIDs()
	}
	if manage, ok := m.activeView().(*ManageHubModel); ok {
		return manage.ScopeProjectIDs()
	}
	if cursorProject != nil {
		return []string{cursorProject.ID}
	}
	if m.currentProject != nil {
		return []string{m.currentProject.ID}
	}
	projects := m.activeProjects()
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID != "" {
			ids = append(ids, project.ID)
		}
	}
	return normalizedProjectIDs(ids)
}

func (m *OutlinerModel) operationCommandContext() CommandContext {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	task, project := m.getTaskRef()
	if task != nil && project != nil {
		ctx.Entity = EntityContext{
			ProjectID: project.ID,
			TaskID:    task.ID,
			State:     string(task.State()),
			Mutable:   !task.IsTerminalState(),
			Archived:  project.Archived,
		}
	} else if m.project != nil {
		ctx.Entity.ProjectID = m.project.ID
		ctx.Entity.Archived = m.project.Archived
		ctx.Entity.Mutable = !m.project.Archived
	}
	ctx.Selection.Tasks = m.SelectedTaskIdentities()
	ctx.Selection.Visual = m.IsMultiSelectActive()
	ctx.Selection.Entities = m.operationSelectionEntities(ctx.Selection.Tasks)
	ctx.Selection.EntityIdentities = taskEntityIdentities(ctx.Selection.Tasks)
	ctx.CurrentEntity = commandContextCurrentEntity(ctx.Entity)
	projects := m.scopeProjects()
	ctx.ScopeProjectIDs = make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID != "" {
			ctx.ScopeProjectIDs = append(ctx.ScopeProjectIDs, project.ID)
		}
	}
	ctx.ScopeProjectIDs = normalizedProjectIDs(ctx.ScopeProjectIDs)
	if m.project != nil {
		ctx.FocusedProjectID = m.project.ID
	} else {
		ctx.FocusedProjectID = m.scopedProjectID
	}
	return ctx
}

func (m *OutlinerModel) operationSelectionEntities(identities []TaskIdentity) []EntityContext {
	normalized := normalizedTaskIdentities(identities)
	entities := make([]EntityContext, 0, len(normalized))
	for _, identity := range normalized {
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			continue
		}
		entities = append(entities, taskOperationEntity(task, project))
	}
	return entities
}

func (m *OutlinerModel) taskAndProjectForIdentity(identity TaskIdentity) (*models.Task, *models.Project) {
	if !identity.Valid() {
		return nil, nil
	}
	if m.project != nil {
		if m.project.ID != identity.ProjectID {
			return nil, nil
		}
		return m.project.GetTaskByID(identity.TaskID), m.project
	}
	project := m.crossResolveProject(identity.ProjectID)
	if project == nil {
		return nil, nil
	}
	return project.GetTaskByID(identity.TaskID), project
}

func taskOperationEntity(task *models.Task, project *models.Project) EntityContext {
	if task == nil || project == nil {
		return EntityContext{}
	}
	return EntityContext{
		ProjectID: project.ID,
		TaskID:    task.ID,
		State:     string(task.State()),
		Mutable:   !task.IsTerminalState(),
		Archived:  project.Archived,
	}
}
