package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Project operation IDs. The tui.scope.* IDs predate this catalog and keep
// their identity because they already name the same action from the Scope Hub;
// tui.project.* is reserved for the maintenance actions added alongside them.
const (
	operationScopeRenameProject   OperationID = "tui.scope.rename-project"
	operationScopeStatusProject   OperationID = "tui.scope.status-project"
	operationScopePinProject      OperationID = "tui.scope.pin-project"
	operationScopeArchiveProject  OperationID = "tui.scope.archive-project"
	operationScopeRestoreProject  OperationID = "tui.scope.restore-project"
	operationScopeOpenProjectNote OperationID = "tui.scope.open-project-notes"

	operationProjectOpen             OperationID = "tui.project.open"
	operationProjectMetrics          OperationID = "tui.project.metrics"
	operationProjectExport           OperationID = "tui.project.export"
	operationProjectOpenFolder       OperationID = "tui.project.open-folder"
	operationProjectArchiveCompleted OperationID = "tui.project.archive-completed"
)

// projectOperationDescriptors is the authoritative catalog of project-targeted
// operations. Every entry resolves one frozen project identity, so the same
// descriptor serves the Scope Hub, Finder, and any other host that names a
// project row.
func projectOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		category OperationCategory,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          category,
			Placement:         PlacementFinderOperation,
			TargetSource:      OperationTargetCurrentEntity,
			TargetCardinality: TargetCardinalityOne,
			Applicability:     projectOperationApplicable,
			Risk:              risk,
		}
	}
	return []OperationDescriptor{
		descriptor(operationProjectOpen, "Open project", "Focus the Outliner on this project", OperationCategoryProject, OperationRiskReadOnly, "focus project"),
		descriptor(operationScopeRenameProject, "Edit project", "Open the full project editor", OperationCategoryProject, OperationRiskMutation, "rename project", "project settings"),
		descriptor(operationScopeStatusProject, "Set project status", "Choose the project status icon", OperationCategoryProject, OperationRiskMutation, "status icon"),
		descriptor(operationScopePinProject, "Toggle project pin", "Pin or unpin the project", OperationCategoryProject, OperationRiskMutation, "pin project", "unpin project"),
		descriptor(operationScopeArchiveProject, "Archive project", "Archive the project", OperationCategoryProject, OperationRiskDestructive),
		descriptor(operationScopeRestoreProject, "Unarchive project", "Return the project to active", OperationCategoryProject, OperationRiskMutation, "restore archived project"),
		descriptor(operationScopeOpenProjectNote, "Open project notes", "Open notes owned by this project", OperationCategoryNote, OperationRiskReadOnly),
		descriptor(operationProjectMetrics, "Project metrics", "Completion and throughput analytics for this project", OperationCategoryReview, OperationRiskReadOnly, "project analytics"),
		descriptor(operationProjectExport, "Export project", "Write this project to an export file", OperationCategoryProject, OperationRiskReadOnly),
		descriptor(operationProjectOpenFolder, "Open project folder", "Reveal this project's directory", OperationCategoryProject, OperationRiskReadOnly, "reveal project"),
		descriptor(operationProjectArchiveCompleted, "Archive completed tasks", "Move this project's completed tasks into its archive", OperationCategoryTask, OperationRiskDestructive, "archive done tasks"),
	}
}

// projectOperationApplicable admits a project operation only when the context
// names exactly one project. currentEntityIdentity degrades to the first scoped
// project when nothing is focused, so a multi-project scope is refused rather
// than allowed to borrow an unrelated project.
func projectOperationApplicable(ctx CommandContext) bool {
	if currentEntityIdentity(ctx).Kind != EntityKindProject {
		return false
	}
	if normalizeEntityIdentity(ctx.CurrentEntity).Kind == EntityKindProject || ctx.Entity.ProjectID != "" {
		return true
	}
	return len(normalizedProjectIDs(ctx.ScopeProjectIDs)) == 1
}

// projectOperationApplicability composes the entity-shape check with the same
// lifecycle and protection requirements execution enforces, resolved from the
// freshly loaded project rather than from the metadata a row was built with. A
// host therefore advertises only actions that can succeed, and a request forced
// against a project whose state changed is refused before the executor's
// destructive confirmation instead of after it. An unresolved project stays
// applicable so the handler reports the single authoritative "no longer
// available" error.
func (m *AppModel) projectOperationApplicability(
	id OperationID,
	label string,
) OperationApplicability {
	requirement, gated := projectOperationRequirements[id]
	return func(ctx CommandContext) bool {
		if !projectOperationApplicable(ctx) {
			return false
		}
		if !gated {
			return true
		}
		project := m.projectByID(currentEntityIdentity(ctx).ProjectID)
		if project == nil {
			return true
		}
		return m.projectRequirementUnmet(requirement, label, project) == nil
	}
}

// registerProjectOperations binds every project operation to one handler.
// Archiving a project delegates confirmation because the shell's project
// archive prompt is also where the sandbox and system Inbox restrictions live;
// archiving completed tasks had no prompt at all and therefore uses the
// executor's canonical destructive confirmation.
func (m *AppModel) registerProjectOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range projectOperationDescriptors() {
		descriptor.Applicability = m.projectOperationApplicability(descriptor.ID, descriptor.Label)
		if err := m.registerOperation(descriptor, OperationBinding{
			SingleHandler:         m.executeProjectOperation,
			DelegateValueProvider: true,
			DelegateTransaction:   true,
			DelegateConfirmation:  descriptor.ID == operationScopeArchiveProject,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeProjectOperation(
	invocation OperationInvocation,
	target OperationTarget,
) OperationHandlerResult {
	if target.Entity.Kind != EntityKindProject || target.ProjectID == "" {
		return OperationHandlerResult{
			Err: fmt.Errorf("%s needs a project target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	project := m.projectByID(target.ProjectID)
	if project == nil {
		return OperationHandlerResult{
			Err: fmt.Errorf("project %s is no longer available", target.ProjectID),
		}
	}
	if err := m.projectOperationUnavailable(invocation.Descriptor, project); err != nil {
		return OperationHandlerResult{Err: err}
	}
	message := m.projectOperationMessage(invocation.Descriptor.ID, project)
	if message == nil {
		return OperationHandlerResult{
			Err: fmt.Errorf("project operation %s has no action", invocation.Descriptor.ID),
		}
	}
	return OperationHandlerResult{Message: message}
}

// projectOperationRequirement declares the project state one operation needs.
type projectOperationRequirement struct {
	requireArchived   bool
	requireActive     bool
	rejectSandbox     bool
	rejectSystemInbox bool
}

var projectOperationRequirements = map[OperationID]projectOperationRequirement{
	operationProjectOpen:             {requireActive: true},
	operationScopeRenameProject:      {rejectSystemInbox: true},
	operationScopePinProject:         {requireActive: true},
	operationScopeArchiveProject:     {requireActive: true, rejectSandbox: true, rejectSystemInbox: true},
	operationScopeRestoreProject:     {requireArchived: true},
	operationProjectArchiveCompleted: {requireActive: true},
}

// projectOperationUnavailable checks the freshly reloaded project rather than
// the state a row was built from, so a project archived or removed since the
// action sheet opened fails loudly instead of acting on stale metadata.
func (m *AppModel) projectOperationUnavailable(
	descriptor OperationDescriptor,
	project *models.Project,
) error {
	return m.projectRequirementUnmet(
		projectOperationRequirements[descriptor.ID],
		descriptor.Label,
		project,
	)
}

// projectRequirementUnmet names why one project fails a requirement. It is the
// single decision both advertisement and execution read, so a sheet can never
// offer an action the handler would refuse.
func (m *AppModel) projectRequirementUnmet(
	requirement projectOperationRequirement,
	label string,
	project *models.Project,
) error {
	switch {
	case requirement.requireArchived && !project.Archived:
		return fmt.Errorf("%s applies only to archived projects", label)
	case requirement.requireActive && project.Archived:
		return fmt.Errorf("%s is unavailable while %s is archived", label, project.Name)
	case requirement.rejectSandbox && project.Sandbox:
		return fmt.Errorf("%s is unavailable for the sandbox project", label)
	case requirement.rejectSystemInbox && m.runtime.IsSystemInboxProject(project.ID):
		return fmt.Errorf("%s is unavailable for the system Inbox project", label)
	}
	return nil
}

//nolint:exhaustive // Only project operations are registered with this handler.
func (m *AppModel) projectOperationMessage(id OperationID, project *models.Project) tea.Msg {
	switch id {
	case operationProjectOpen:
		return ApplyProjectScopeMsg{ProjectID: project.ID}
	case operationScopeRenameProject:
		return EditProjectMsg{Project: project}
	case operationScopeStatusProject:
		return ShowStatusIconPickerMsg{Project: project}
	case operationScopePinProject:
		return ProjectPinRequestMsg{Project: project, Pinned: !project.Pinned}
	case operationScopeArchiveProject:
		return ProjectArchiveRequestMsg{Project: project, Archive: true}
	case operationScopeRestoreProject:
		return ProjectArchiveRequestMsg{Project: project, Archive: false}
	case operationScopeOpenProjectNote:
		return ShowNotesMsg{Project: project}
	case operationProjectMetrics:
		return ShowMetricsDashboardMsg{Project: project}
	case operationProjectExport:
		return ExportProjectMsg{Project: project}
	case operationProjectOpenFolder:
		return OpenDataFolderMsg{Project: project}
	case operationProjectArchiveCompleted:
		return ArchiveCompletedTasksMsg{Project: project}
	default:
		return nil
	}
}
