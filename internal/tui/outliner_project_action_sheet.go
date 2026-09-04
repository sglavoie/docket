package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// The Outliner's project sheet groups into two categories only. Project holds
// the reversible edits and Danger the two acts that end the project's presence
// in the workspace, so the destructive pair is never mixed into the list a user
// arrows through looking for Edit.
const (
	outlinerProjectActionCategoryProject listsurface.ListCategoryID = "outliner-project-actions:project"
	outlinerProjectActionCategoryDanger  listsurface.ListCategoryID = "outliner-project-actions:danger"
)

var outlinerProjectActionCategories = []listsurface.ListCategory{
	{ID: outlinerProjectActionCategoryProject, Label: "Project"},
	{ID: outlinerProjectActionCategoryDanger, Label: "Danger"},
}

// outlinerProjectOperationManifest is the Outliner's project action list and the
// source the project sheet groups and orders by. It is deliberately separate
// from the Project Hub's ProjectOperationManifest: one-path is a per-surface
// rule, so the same operation is legitimately a direct key on one surface and a
// menu row on the other. Edit project is the clearest case — `e` on the Hub,
// menu row here, because the Outliner's `e` already edits a task.
//
// The DirectOnly entries stay listed here so ValidateOperationOnePath can see
// them, but the sheet skips them: each already has a key on this surface. Pin
// joined them when the project header gained `*`, the same rule that took
// Comments and Attachments out of the task menu when they gained `;` and `&`.
//
// Three project operations are absent on purpose. Open project already has
// Enter through OperationCoreActivateGroup and Open project notes already has
// Ctrl+N through OperationCoreNotes; listing either would be the second path the
// rule exists to prevent. Unarchive project is absent for a different reason: an
// Outliner project header is drawn from TaskWorkspaceProjects(), which is
// non-archived by definition, so the row could never be offered. Archiving is
// therefore one-way from here — the Hub is where an archived project comes back.
func outlinerProjectOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: operation.OperationProjectSetTags, Label: "Set project tags", Aliases: []string{"project tags"}, Provider: operation.ValueProviderProjectTags, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationProjectActions, Label: "Project actions", Aliases: []string{"project action sheet"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operation.OperationScopeRenameProject, Label: "Edit project", Aliases: []string{"project settings"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation},
		{ID: operation.OperationScopePinProject, Label: "Toggle project pin", Aliases: []string{"pin project", "unpin project"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationScopeArchiveProject, Label: "Archive project", Category: outlinerProjectActionCategoryDanger, Risk: operation.OperationRiskDestructive},
		{ID: operation.OperationProjectDelete, Label: "Delete project", Aliases: []string{"remove project", "destroy project"}, Category: outlinerProjectActionCategoryDanger, Risk: operation.OperationRiskDestructive},
	}
}

// outlinerProjectOperationManifestEntryFor is the closed allowlist behind the
// project sheet, the counterpart of taskOperationManifestEntryFor. The two stay
// separate so a project operation can never appear in the task Space menu and a
// task operation can never appear in this one.
func outlinerProjectOperationManifestEntryFor(id operation.OperationID) (operation.OperationManifestEntry, bool) {
	for _, entry := range outlinerProjectOperationManifest() {
		if entry.ID == id {
			return entry, true
		}
	}
	return operation.OperationManifestEntry{}, false
}

// projectActionSheetOperationDescriptor states the sheet's applicability apart
// from registerProjectOperations because that family shares one applicability —
// "the context names a project" — which a section header in a single-project
// scope also satisfies. This one asks the stricter question the row kind
// answers, so the key opens a project sheet only where a project header actually
// is.
func projectActionSheetOperationDescriptor() operation.OperationDescriptor {
	return operation.OperationDescriptor{
		ID:                   operation.OperationProjectActions,
		Label:                "Project Actions",
		Description:          "Open the project action menu",
		Aliases:              []string{"project action sheet", "project menu"},
		Category:             operation.OperationCategoryProject,
		TargetSource:         operation.OperationTargetCurrentEntity,
		TargetCardinality:    operation.TargetCardinalityOne,
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Risk:                 operation.OperationRiskReadOnly,
		Applicability: func(ctx operation.CommandContext) bool {
			return operation.CoreOutlinerApplicability(ctx) &&
				!ctx.Selection.Visual &&
				!ctx.Entity.HasChecklistCursor &&
				ctx.Entity.HasProjectHeaderCursor
		},
	}
}

// registerProjectActionSheetOperation binds Space on a project header.
func (m *AppModel) registerProjectActionSheetOperation() error {
	if m.operations.registry == nil || m.operations.executor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	return m.registerOperation(projectActionSheetOperationDescriptor(), operation.OperationBinding{
		SingleHandler: func(
			invocation operation.OperationInvocation,
			_ operation.OperationTarget,
		) operation.OperationHandlerResult {
			if err := m.openProjectActionSheet(invocation.Context); err != nil {
				m.setInfoStatus(err.Error())
				return operation.OperationHandlerResult{Message: commandMessage(status.DefaultStatusTimeout())}
			}
			return operation.OperationHandlerResult{}
		},
	})
}

// NewProjectActionSheetModel builds the Outliner project sheet on the shared
// contextual engine. The CommandContext carries the frozen project identity, so
// a cursor move beneath the modal cannot retarget the eventual operation.
func NewProjectActionSheetModel(
	registry *operation.OperationRegistry,
	executor *operation.OperationExecutor,
	context operation.CommandContext,
	projectName string,
) (*listsurface.ContextActionSheetModel, error) {
	return listsurface.NewContextActionSheetModel(
		registry,
		executor,
		context,
		ProjectActionSheetOperationIDs(registry, context),
		listsurface.ContextActionSheetConfig{
			Host:            listsurface.ContextActionHostOutliner,
			Completion:      listsurface.ContextActionCompleteCloseSheet,
			Title:           "Project Actions",
			Subtitle:        strings.TrimSpace(projectName),
			ItemIDPrefix:    "project-action:",
			Categories:      outlinerProjectActionCategories,
			CategoryFor:     outlinerProjectActionCategory,
			SecondaryFor:    operationActionSecondaryText,
			ImmediateFilter: true,
			Targets:         1,
			EmptyState: listsurface.ListEmptyState{
				Title:  "No project actions available.",
				Detail: "Move the cursor to a project header.",
			},
			NoMatchState: listsurface.ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
}

// ProjectActionSheetOperationIDs returns the deterministic project-operation
// projection the sheet and its tests both read. Applicability is evaluated
// against the frozen project, so a row the project's own state forbids — pinning
// or archiving one that is already archived — drops out with no branching here:
// ProjectOperationRequirements already decides it.
func ProjectActionSheetOperationIDs(
	registry *operation.OperationRegistry,
	context operation.CommandContext,
) []operation.OperationID {
	if registry == nil {
		return []operation.OperationID{}
	}
	ids := make([]operation.OperationID, 0, len(outlinerProjectOperationManifest()))
	for _, entry := range outlinerProjectOperationManifest() {
		if entry.DirectOnly {
			continue
		}
		descriptor, ok := registry.Descriptor(entry.ID)
		if !ok || !descriptor.Applicable(context) {
			continue
		}
		if _, err := (operation.OperationTargetResolver{}).Resolve(descriptor, context); err != nil {
			continue
		}
		ids = append(ids, entry.ID)
	}
	return ids
}

func outlinerProjectActionCategory(descriptor operation.OperationDescriptor) listsurface.ListCategoryID {
	if entry, manifested := outlinerProjectOperationManifestEntryFor(descriptor.ID); manifested {
		return entry.Category
	}
	if descriptor.Risk == operation.OperationRiskDestructive {
		return outlinerProjectActionCategoryDanger
	}
	return outlinerProjectActionCategoryProject
}

// openProjectActionSheet raises the sheet for the project header under the
// cursor. The refusal names the row kind the key needs rather than the
// operation, so a user who pressed Space one row too high knows where to move.
func (m *AppModel) openProjectActionSheet(context operation.CommandContext) error {
	project := m.cursorProjectHeaderProject()
	if project == nil {
		return fmt.Errorf("move the cursor to a project header to open project actions")
	}
	sheet, err := NewProjectActionSheetModel(
		m.GetOperationRegistry(),
		m.operations.executor,
		context,
		project.Name,
	)
	if err != nil {
		return err
	}
	m.operations.actionSheet = sheet
	return nil
}
