package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The Outliner builds its own CommandContext rather than borrowing the shell's,
// because a mounted surface reaching dispatchOperationKey holds no shell
// reference. Both halves fill the same struct through the same
// checklistOperationEntity and taskOperationEntity constructors in
// operation_target_context.go, so the two contexts cannot disagree about what a
// row is.

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

func (m *OutlinerModel) operationSelectionEntities(identities []taskref.TaskIdentity) []EntityContext {
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

func (m *OutlinerModel) taskAndProjectForIdentity(identity taskref.TaskIdentity) (*models.Task, *models.Project) {
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
