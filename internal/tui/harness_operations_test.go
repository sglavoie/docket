package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

const CapabilityHarness Capability = "surface:test-harness"

const (
	operationHarnessExportProject   OperationID = "tui.harness.export-project"
	operationHarnessNewProject      OperationID = "tui.harness.new-project"
	operationHarnessOutliner        OperationID = "tui.harness.outliner"
	operationHarnessOutlinerRoot    OperationID = "tui.harness.outliner-root"
	operationHarnessReference       OperationID = "tui.harness.reference"
	operationHarnessScope           OperationID = "tui.harness.scope"
	operationHarnessInbox           OperationID = "tui.harness.inbox"
	operationHarnessProjectInbox    OperationID = "tui.harness.project-inbox"
	operationHarnessRenameProject   OperationID = "tui.harness.rename-project"
	operationHarnessTaskAttachments OperationID = "tui.harness.task-attachments"
	operationHarnessTaskComments    OperationID = "tui.harness.task-comments"
	operationHarnessProjectNotes    OperationID = "tui.harness.project-notes"
	operationHarnessConflict        OperationID = "tui.harness.conflict"
)

type harnessOperationSpec struct {
	id       OperationID
	label    string
	category OperationCategory
	message  func() tea.Msg
}

// registerHarnessOperations keeps layout and semantic scenario setup on the
// same OperationRegistry/OperationExecutor foundation as production actions.
// CapabilityHarness is added only by UITestDriver.runCommand, so these deep
// links never appear in help, footers, or available-command output.
func (m *AppModel) registerHarnessOperations() error {
	specs := []harnessOperationSpec{
		{operationHarnessExportProject, "export project", OperationCategoryProject, func() tea.Msg { return ExportProjectMsg{} }},
		{operationHarnessNewProject, "new project", OperationCategoryProject, func() tea.Msg { return CreateProjectMsg{} }},
		{operationHarnessOutliner, "outliner", OperationCategoryNavigation, func() tea.Msg { return ShowGlobalOutlinerMsg{} }},
		{operationHarnessOutlinerRoot, "outliner root", OperationCategoryNavigation, func() tea.Msg { return ShowGlobalOutlinerMsg{} }},
		{operationHarnessReference, "reference", OperationCategorySystem, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: m.currentView(),
				ViewName:   "Outliner",
				Items:      m.coreInputHelpItems(),
			}
		}},
		{operationHarnessScope, "choose scope", OperationCategoryScope, func() tea.Msg { return ShowProjectHubMsg{} }},
		{operationHarnessInbox, "global inbox mode", OperationCategoryMode, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeInbox}}
		}},
		{operationHarnessProjectInbox, "project inbox mode", OperationCategoryMode, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeInbox}}
		}},
		{operationHarnessRenameProject, "rename project", OperationCategoryProject, func() tea.Msg {
			_, project := m.outlinerTarget()
			if project == nil {
				return nil
			}
			return EditProjectMsg{Project: project}
		}},
		{operationHarnessTaskAttachments, "task attachments", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return ShowTaskAttachmentsMsg{Task: task, Project: project}
		}},
		{operationHarnessTaskComments, "task comments", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return ShowTaskCommentsMsg{Task: task, Project: project}
		}},
		{operationHarnessProjectNotes, "show notes", OperationCategoryNote, func() tea.Msg {
			_, project := m.outlinerTarget()
			if project != nil {
				return ShowNotesManagerMsg{Scope: ProjectNotesScope(project.ID)}
			}
			return ShowNotesManagerMsg{Scope: AllNotesScope()}
		}},
		{operationHarnessConflict, "resolve conflict", OperationCategorySystem, func() tea.Msg {
			retryRequired := true
			return ShowConflictResolutionMsg{Bundle: &storage.ConflictBundle{
				Record: storage.ConflictRecord{
					ProjectName:            "Rich Project",
					Action:                 "task edit",
					RequiresOperationRetry: &retryRequired,
					Conflicts: []storage.MergeConflict{{
						Kind: storage.MergeConflictFieldEdit, TaskTitle: "Build login form",
						Field: "title", Base: "Build login", Local: "Build secure login", Current: "Build sign-in",
					}},
				},
			}}
		}},
	}

	for _, spec := range specs {
		spec := spec
		descriptor := OperationDescriptor{
			ID:                   spec.id,
			Label:                spec.label,
			Category:             spec.category,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityHarness),
			Risk:                 OperationRiskReadOnly,
		}
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilityHarness),
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
			Handler: func(OperationInvocation) OperationHandlerResult {
				return OperationHandlerResult{Message: spec.message()}
			},
		}); err != nil {
			return fmt.Errorf("%s: %w", spec.id, err)
		}
	}
	return nil
}
