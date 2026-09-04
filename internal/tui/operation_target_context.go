package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// operationSelectionProvider exposes stable selected identities without
// leaking renderer selection maps into execution.
type operationSelectionProvider interface {
	SelectedTaskIdentities() []taskref.TaskIdentity
}

type operationVisualSelectionProvider interface {
	IsMultiSelectActive() bool
}

type operationSelectionCountsProvider interface {
	selectionCounts() SelectionCounts
}

// operationCommandContext captures the active cursor, multi-selection, project
// scope, and entity state before an overlay (such as the palette) shadows the
// primary view.
func (m *AppModel) operationCommandContext() operation.CommandContext {
	view := m.currentView()
	ctx := route.PresentationCommandContext(view)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	if manage, ok := m.activeView().(*ManageHubModel); ok {
		ctx = manage.CommandContext()
	}

	task, project := m.getCurrentCursorTaskAndProject()
	header := m.cursorProjectHeaderProject()
	checklist, onChecklistRow := m.cursorChecklistItem()
	switch {
	case onChecklistRow:
		ctx.Entity = checklistOperationEntity(checklist)
	case task != nil && project != nil:
		ctx.Entity = operation.EntityContext{
			ProjectID: project.ID,
			TaskID:    task.ID,
			Archived:  project.Archived,
		}
	case header != nil:
		ctx.Entity.ProjectID = header.ID
		ctx.Entity.Archived = header.Archived
	case m.activeOutliner() == nil && m.currentProject != nil:
		ctx.Entity.ProjectID = m.currentProject.ID
		ctx.Entity.Archived = m.currentProject.Archived
	}
	// This is the context the shell dispatches keys against, so the row kind has
	// to be filled here too or Enter never reaches the zoom the footer offers.
	ctx.Entity.HasSectionHeaderCursor = m.cursorNamesSectionHeader()
	ctx.Entity.HasProjectHeaderCursor = header != nil
	_, ctx.Entity.HasNoteCursor = m.cursorNoteIdentity()

	if selected, ok := m.activeView().(operationSelectionProvider); ok {
		ctx.Selection.Tasks = selected.SelectedTaskIdentities()
	}
	if visual, ok := m.activeView().(operationVisualSelectionProvider); ok {
		ctx.Selection.Visual = visual.IsMultiSelectActive()
	}
	if classified, ok := m.activeView().(operationSelectionCountsProvider); ok {
		counts := classified.selectionCounts()
		ctx.Selection.VisibleCount = counts.Visible
		ctx.Selection.HiddenCount = counts.Hidden
	}
	ctx.Selection.Entities = m.operationSelectionEntities(ctx.Selection.Tasks)
	ctx.Selection.EntityIdentities = operation.TaskEntityIdentities(ctx.Selection.Tasks)
	ctx.CurrentEntity = operation.CommandContextCurrentEntity(ctx.Entity)
	ctx.ScopeProjectIDs = m.operationScopeProjectIDs(project)
	return ctx
}

func (m *AppModel) operationSelectionEntities(identities []taskref.TaskIdentity) []operation.EntityContext {
	normalized := operation.NormalizedTaskIdentities(identities)
	entities := make([]operation.EntityContext, 0, len(normalized))
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
		return operation.NormalizedProjectIDs(ids)
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
	projects := m.taskWorkspaceProjects()
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID != "" {
			ids = append(ids, project.ID)
		}
	}
	return operation.NormalizedProjectIDs(ids)
}

// checklistOperationEntity names the checklist item under the cursor while
// keeping the owner task and project filled, so the archived-project guard and
// the checklist operations' own target resolution both keep working.
func checklistOperationEntity(cursor checklistCursor) operation.EntityContext {
	entity := taskOperationEntity(cursor.Task, cursor.Project)
	entity.HasChecklistCursor = true
	entity.ChecklistIndex = cursor.Index
	return entity
}

func taskOperationEntity(task *models.Task, project *models.Project) operation.EntityContext {
	if task == nil || project == nil {
		return operation.EntityContext{}
	}
	return operation.EntityContext{
		ProjectID: project.ID,
		TaskID:    task.ID,
		Archived:  project.Archived,
	}
}
