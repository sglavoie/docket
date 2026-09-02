package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// operationSelectionProvider exposes stable selected identities without
// leaking renderer selection maps into execution.
type operationSelectionProvider interface {
	SelectedTaskIdentities() []TaskIdentity
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
func (m *AppModel) operationCommandContext() CommandContext {
	view := m.currentView()
	ctx := PresentationCommandContext(view)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
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
		ctx.Entity = EntityContext{
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
	ctx.Selection.EntityIdentities = taskEntityIdentities(ctx.Selection.Tasks)
	ctx.CurrentEntity = commandContextCurrentEntity(ctx.Entity)
	ctx.ScopeProjectIDs = m.operationScopeProjectIDs(project)
	return ctx
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
	return normalizedProjectIDs(ids)
}

func (m *OutlinerModel) operationCommandContext() CommandContext {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	task, project := m.getTaskRef()
	header := m.cursorProjectHeaderProject()
	if checklist, onChecklistRow := m.cursorChecklistItem(); onChecklistRow {
		ctx.Entity = checklistOperationEntity(checklist)
	} else if task != nil && project != nil {
		ctx.Entity = EntityContext{
			ProjectID: project.ID,
			TaskID:    task.ID,
			Archived:  project.Archived,
		}
	} else if header != nil {
		ctx.Entity.ProjectID = header.ID
		ctx.Entity.Archived = header.Archived
	} else if m.project != nil {
		ctx.Entity.ProjectID = m.project.ID
		ctx.Entity.Archived = m.project.Archived
	}
	// The row kind is an independent fact, not another arm of the chain above: a
	// section header in a single-project Outliner still needs that project's
	// Archived guard, which only the fallback fills.
	ctx.Entity.HasSectionHeaderCursor = m.cursorNamesSectionHeader()
	ctx.Entity.HasProjectHeaderCursor = header != nil
	_, ctx.Entity.HasNoteCursor = m.cursorNoteIdentity()
	ctx.Selection.Tasks = m.SelectedTaskIdentities()
	ctx.Selection.Visual = m.IsMultiSelectActive()
	counts := m.selectionCounts()
	ctx.Selection.VisibleCount = counts.Visible
	ctx.Selection.HiddenCount = counts.Hidden
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

// checklistOperationEntity names the checklist item under the cursor while
// keeping the owner task and project filled, so the archived-project guard and
// the checklist operations' own target resolution both keep working.
func checklistOperationEntity(cursor checklistCursor) EntityContext {
	entity := taskOperationEntity(cursor.Task, cursor.Project)
	entity.HasChecklistCursor = true
	entity.ChecklistIndex = cursor.Index
	return entity
}

func taskOperationEntity(task *models.Task, project *models.Project) EntityContext {
	if task == nil || project == nil {
		return EntityContext{}
	}
	return EntityContext{
		ProjectID: project.ID,
		TaskID:    task.ID,
		Archived:  project.Archived,
	}
}
