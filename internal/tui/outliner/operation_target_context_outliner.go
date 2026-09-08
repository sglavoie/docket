package outliner

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The Outliner builds its own CommandContext rather than borrowing the shell's,
// because a mounted surface reaching dispatchOperationKey holds no shell
// reference. Both halves fill the same struct through the same
// checklistOperationEntity and taskOperationEntity constructors declared below,
// so the two contexts cannot disagree about what a row is. Those two live here
// rather than beside the shell's context because both are written in the
// Outliner's own row vocabulary: a checklist cursor is an Outliner row.

func (m *OutlinerModel) OperationCommandContext() operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	task, project := m.GetTaskRef()
	header := m.CursorProjectHeaderProject()
	if checklist, onChecklistRow := m.CursorChecklistItem(); onChecklistRow {
		ctx.Entity = ChecklistOperationEntity(checklist)
	} else if task != nil && project != nil {
		ctx.Entity = operation.EntityContext{
			ProjectID: project.ID,
			TaskID:    task.ID,
			Archived:  project.Archived,
		}
	} else if header != nil {
		ctx.Entity.ProjectID = header.ID
		ctx.Entity.Archived = header.Archived
	} else if m.Project != nil {
		ctx.Entity.ProjectID = m.Project.ID
		ctx.Entity.Archived = m.Project.Archived
	}
	// The row kind is an independent fact, not another arm of the chain above: a
	// section header in a single-project Outliner still needs that project's
	// Archived guard, which only the fallback fills.
	ctx.Entity.HasSectionHeaderCursor = m.CursorNamesSectionHeader()
	ctx.Entity.HasProjectHeaderCursor = header != nil
	_, ctx.Entity.HasNoteCursor = m.CursorNoteIdentity()
	ctx.Peek = m.PeekCommandContext()
	ctx.Selection.Tasks = m.SelectedTaskIdentities()
	ctx.Selection.Visual = m.IsMultiSelectActive()
	counts := m.SelectionCounts()
	ctx.Selection.VisibleCount = counts.Visible
	ctx.Selection.HiddenCount = counts.Hidden
	ctx.Selection.Entities = m.operationSelectionEntities(ctx.Selection.Tasks)
	ctx.Selection.EntityIdentities = operation.TaskEntityIdentities(ctx.Selection.Tasks)
	ctx.CurrentEntity = operation.CommandContextCurrentEntity(ctx.Entity)
	projects := m.ScopeProjects()
	ctx.ScopeProjectIDs = make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID != "" {
			ctx.ScopeProjectIDs = append(ctx.ScopeProjectIDs, project.ID)
		}
	}
	ctx.ScopeProjectIDs = operation.NormalizedProjectIDs(ctx.ScopeProjectIDs)
	return ctx
}

func (m *OutlinerModel) operationSelectionEntities(identities []taskref.TaskIdentity) []operation.EntityContext {
	normalized := operation.NormalizedTaskIdentities(identities)
	entities := make([]operation.EntityContext, 0, len(normalized))
	for _, identity := range normalized {
		task, project := m.TaskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			continue
		}
		entities = append(entities, TaskOperationEntity(task, project))
	}
	return entities
}

func (m *OutlinerModel) TaskAndProjectForIdentity(identity taskref.TaskIdentity) (*models.Task, *models.Project) {
	if !identity.Valid() {
		return nil, nil
	}
	if m.Project != nil {
		if m.Project.ID != identity.ProjectID {
			return nil, nil
		}
		return m.Project.GetTaskByID(identity.TaskID), m.Project
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
func ChecklistOperationEntity(cursor ChecklistCursor) operation.EntityContext {
	entity := TaskOperationEntity(cursor.Task, cursor.Project)
	entity.HasChecklistCursor = true
	entity.ChecklistIndex = cursor.Index
	return entity
}

func TaskOperationEntity(task *models.Task, project *models.Project) operation.EntityContext {
	if task == nil || project == nil {
		return operation.EntityContext{}
	}
	return operation.EntityContext{
		ProjectID: project.ID,
		TaskID:    task.ID,
		Archived:  project.Archived,
	}
}
