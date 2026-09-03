package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/status"
)

// operationProjectActions is the Outliner's Space on a project group header. It
// lives in the tui.project.* namespace because it names a project, and apart
// from the Hub's own sheet operation because the two surfaces offer different
// rows: the Hub reaches its project keys directly and menus only the lifecycle
// pair, while here the whole administration set is the menu.
const operationProjectActions OperationID = "tui.project.actions"

// The Outliner's project sheet groups into two categories only. Project holds
// the reversible edits and Danger the two acts that end the project's presence
// in the workspace, so the destructive pair is never mixed into the list a user
// arrows through looking for Edit.
const (
	outlinerProjectActionCategoryProject ListCategoryID = "outliner-project-actions:project"
	outlinerProjectActionCategoryDanger  ListCategoryID = "outliner-project-actions:danger"
)

var outlinerProjectActionCategories = []ListCategory{
	{ID: outlinerProjectActionCategoryProject, Label: "Project"},
	{ID: outlinerProjectActionCategoryDanger, Label: "Danger"},
}

// outlinerProjectOperationManifest is the Outliner's project action list and the
// source the project sheet groups and orders by. It is deliberately separate
// from the Project Hub's projectOperationManifest: one-path is a per-surface
// rule, so the same operation is legitimately a direct key on one surface and a
// menu row on the other. Edit project is the clearest case — `e` on the Hub,
// menu row here, because the Outliner's `e` already edits a task.
//
// The DirectOnly entries stay listed here so validateOperationOnePath can see
// them, but the sheet skips them: each already has a key on this surface. Pin
// joined them when the project header gained `*`, the same rule that took
// Comments and Attachments out of the task menu when they gained `;` and `&`.
//
// Three project operations are absent on purpose. Open project already has
// Enter through operationCoreActivateGroup and Open project notes already has
// Ctrl+N through operationCoreNotes; listing either would be the second path the
// rule exists to prevent. Unarchive project is absent for a different reason: an
// Outliner project header is drawn from TaskWorkspaceProjects(), which is
// non-archived by definition, so the row could never be offered. Archiving is
// therefore one-way from here — the Hub is where an archived project comes back.
func outlinerProjectOperationManifest() []operationManifestEntry {
	return []operationManifestEntry{
		{ID: operationProjectSetTags, Label: "Set project tags", Aliases: []string{"project tags"}, Provider: ValueProviderProjectTags, Category: outlinerProjectActionCategoryProject, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationProjectActions, Label: "Project actions", Aliases: []string{"project action sheet"}, Category: outlinerProjectActionCategoryProject, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationScopeRenameProject, Label: "Edit project", Aliases: []string{"project settings"}, Category: outlinerProjectActionCategoryProject, Risk: OperationRiskMutation},
		{ID: operationScopePinProject, Label: "Toggle project pin", Aliases: []string{"pin project", "unpin project"}, Category: outlinerProjectActionCategoryProject, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationScopeArchiveProject, Label: "Archive project", Category: outlinerProjectActionCategoryDanger, Risk: OperationRiskDestructive},
		{ID: operationProjectDelete, Label: "Delete project", Aliases: []string{"remove project", "destroy project"}, Category: outlinerProjectActionCategoryDanger, Risk: OperationRiskDestructive},
	}
}

// outlinerProjectOperationManifestEntryFor is the closed allowlist behind the
// project sheet, the counterpart of taskOperationManifestEntryFor. The two stay
// separate so a project operation can never appear in the task Space menu and a
// task operation can never appear in this one.
func outlinerProjectOperationManifestEntryFor(id OperationID) (operationManifestEntry, bool) {
	for _, entry := range outlinerProjectOperationManifest() {
		if entry.ID == id {
			return entry, true
		}
	}
	return operationManifestEntry{}, false
}

// projectActionSheetOperationDescriptor states the sheet's applicability apart
// from registerProjectOperations because that family shares one applicability —
// "the context names a project" — which a section header in a single-project
// scope also satisfies. This one asks the stricter question the row kind
// answers, so the key opens a project sheet only where a project header actually
// is.
func projectActionSheetOperationDescriptor() OperationDescriptor {
	return OperationDescriptor{
		ID:                   operationProjectActions,
		Label:                "Project Actions",
		Description:          "Open the project action menu",
		Aliases:              []string{"project action sheet", "project menu"},
		Category:             OperationCategoryProject,
		TargetSource:         OperationTargetCurrentEntity,
		TargetCardinality:    TargetCardinalityOne,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Risk:                 OperationRiskReadOnly,
		Applicability: func(ctx CommandContext) bool {
			return coreOutlinerApplicability(ctx) &&
				!ctx.Selection.Visual &&
				!ctx.Entity.HasChecklistCursor &&
				ctx.Entity.HasProjectHeaderCursor
		},
	}
}

// registerProjectActionSheetOperation binds Space on a project header.
func (m *AppModel) registerProjectActionSheetOperation() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	return m.registerOperation(projectActionSheetOperationDescriptor(), OperationBinding{
		SingleHandler: func(
			invocation OperationInvocation,
			_ OperationTarget,
		) OperationHandlerResult {
			if err := m.openProjectActionSheet(invocation.Context); err != nil {
				m.setInfoStatus(err.Error())
				return OperationHandlerResult{Message: commandMessage(status.DefaultStatusTimeout())}
			}
			return OperationHandlerResult{}
		},
	})
}

// NewProjectActionSheetModel builds the Outliner project sheet on the shared
// contextual engine. The CommandContext carries the frozen project identity, so
// a cursor move beneath the modal cannot retarget the eventual operation.
func NewProjectActionSheetModel(
	registry *OperationRegistry,
	executor *OperationExecutor,
	context CommandContext,
	projectName string,
) (*ContextActionSheetModel, error) {
	return NewContextActionSheetModel(
		registry,
		executor,
		context,
		ProjectActionSheetOperationIDs(registry, context),
		ContextActionSheetConfig{
			Host:            ContextActionHostOutliner,
			Completion:      ContextActionCompleteCloseSheet,
			Title:           "Project Actions",
			Subtitle:        strings.TrimSpace(projectName),
			ItemIDPrefix:    "project-action:",
			Categories:      outlinerProjectActionCategories,
			CategoryFor:     outlinerProjectActionCategory,
			SecondaryFor:    operationActionSecondaryText,
			ImmediateFilter: true,
			Targets:         1,
			EmptyState: ListEmptyState{
				Title:  "No project actions available.",
				Detail: "Move the cursor to a project header.",
			},
			NoMatchState: ListEmptyState{
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
// projectOperationRequirements already decides it.
func ProjectActionSheetOperationIDs(
	registry *OperationRegistry,
	context CommandContext,
) []OperationID {
	if registry == nil {
		return []OperationID{}
	}
	ids := make([]OperationID, 0, len(outlinerProjectOperationManifest()))
	for _, entry := range outlinerProjectOperationManifest() {
		if entry.DirectOnly {
			continue
		}
		descriptor, ok := registry.Descriptor(entry.ID)
		if !ok || !descriptor.Applicable(context) {
			continue
		}
		if _, err := (OperationTargetResolver{}).Resolve(descriptor, context); err != nil {
			continue
		}
		ids = append(ids, entry.ID)
	}
	return ids
}

func outlinerProjectActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if entry, manifested := outlinerProjectOperationManifestEntryFor(descriptor.ID); manifested {
		return entry.Category
	}
	if descriptor.Risk == OperationRiskDestructive {
		return outlinerProjectActionCategoryDanger
	}
	return outlinerProjectActionCategoryProject
}

// openProjectActionSheet raises the sheet for the project header under the
// cursor. The refusal names the row kind the key needs rather than the
// operation, so a user who pressed Space one row too high knows where to move.
func (m *AppModel) openProjectActionSheet(context CommandContext) error {
	project := m.cursorProjectHeaderProject()
	if project == nil {
		return fmt.Errorf("move the cursor to a project header to open project actions")
	}
	sheet, err := NewProjectActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		context,
		project.Name,
	)
	if err != nil {
		return err
	}
	m.contextActionSheet = sheet
	return nil
}
