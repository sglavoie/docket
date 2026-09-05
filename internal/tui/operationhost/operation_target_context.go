package operationhost

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
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
	SelectionCounts() listsurface.SelectionCounts
}

// operationCommandContext captures the active cursor, multi-selection, project
// scope, and entity state before an overlay (such as the palette) shadows the
// primary view.
func (c *Controller) CommandContext(shell Shell) operation.CommandContext {
	view := shell.CurrentView()
	ctx := route.PresentationCommandContext(view)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	if manage, ok := shell.ActiveView().(*projecthub.ManageHubModel); ok {
		ctx = manage.CommandContext()
	}

	task, project := shell.CursorTaskAndProject()
	header := shell.CursorProjectHeaderProject()
	checklist, onChecklistRow := shell.CursorChecklistItem()
	switch {
	case onChecklistRow:
		ctx.Entity = outliner.ChecklistOperationEntity(checklist)
	case task != nil && project != nil:
		ctx.Entity = operation.EntityContext{
			ProjectID: project.ID,
			TaskID:    task.ID,
			Archived:  project.Archived,
		}
	case header != nil:
		ctx.Entity.ProjectID = header.ID
		ctx.Entity.Archived = header.Archived
	case shell.ActiveOutliner() == nil && shell.CurrentProject() != nil:
		ctx.Entity.ProjectID = shell.CurrentProject().ID
		ctx.Entity.Archived = shell.CurrentProject().Archived
	}
	// This is the context the shell dispatches keys against, so the row kind has
	// to be filled here too or Enter never reaches the zoom the footer offers.
	ctx.Entity.HasSectionHeaderCursor = shell.CursorNamesSectionHeader()
	ctx.Entity.HasProjectHeaderCursor = header != nil
	_, ctx.Entity.HasNoteCursor = shell.CursorNoteIdentity()

	if selected, ok := shell.ActiveView().(operationSelectionProvider); ok {
		ctx.Selection.Tasks = selected.SelectedTaskIdentities()
	}
	if visual, ok := shell.ActiveView().(operationVisualSelectionProvider); ok {
		ctx.Selection.Visual = visual.IsMultiSelectActive()
	}
	if classified, ok := shell.ActiveView().(operationSelectionCountsProvider); ok {
		counts := classified.SelectionCounts()
		ctx.Selection.VisibleCount = counts.Visible
		ctx.Selection.HiddenCount = counts.Hidden
	}
	ctx.Selection.Entities = selectionEntities(shell, ctx.Selection.Tasks)
	ctx.Selection.EntityIdentities = operation.TaskEntityIdentities(ctx.Selection.Tasks)
	ctx.CurrentEntity = operation.CommandContextCurrentEntity(ctx.Entity)
	ctx.ScopeProjectIDs = scopeProjectIDs(shell, project)
	return ctx
}

func selectionEntities(shell Shell, identities []taskref.TaskIdentity) []operation.EntityContext {
	normalized := operation.NormalizedTaskIdentities(identities)
	entities := make([]operation.EntityContext, 0, len(normalized))
	for _, identity := range normalized {
		task, project := taskAndProjectForIdentity(shell, identity)
		if task == nil || project == nil {
			continue
		}
		entities = append(entities, outliner.TaskOperationEntity(task, project))
	}
	return entities
}

func scopeProjectIDs(shell Shell, cursorProject *models.Project) []string {
	if out := shell.ActiveOutliner(); out != nil {
		projects := out.ScopeProjects()
		ids := make([]string, 0, len(projects))
		for _, project := range projects {
			if project != nil && project.ID != "" {
				ids = append(ids, project.ID)
			}
		}
		return operation.NormalizedProjectIDs(ids)
	}
	if manage, ok := shell.ActiveView().(*projecthub.ManageHubModel); ok {
		return manage.ScopeProjectIDs()
	}
	if cursorProject != nil {
		return []string{cursorProject.ID}
	}
	if shell.CurrentProject() != nil {
		return []string{shell.CurrentProject().ID}
	}
	projects := shell.TaskWorkspaceProjects()
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID != "" {
			ids = append(ids, project.ID)
		}
	}
	return operation.NormalizedProjectIDs(ids)
}

// taskAndProjectForIdentity resolves a stable identity against the live
// catalog. It is the one place the context and the reference builder turn a
// remembered identity back into the pair a handler acts on, so a task that
// moved or was deleted between the press and the execution answers nil rather
// than a stale pointer.
func taskAndProjectForIdentity(
	shell Shell,
	identity taskref.TaskIdentity,
) (*models.Task, *models.Project) {
	if !identity.Valid() {
		return nil, nil
	}
	project := shell.ProjectByID(identity.ProjectID)
	if project == nil {
		return nil, nil
	}
	return project.GetTaskByID(identity.TaskID), project
}
