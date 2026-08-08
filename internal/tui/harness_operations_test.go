package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

const CapabilityHarness Capability = "surface:test-harness"

const (
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
		{operationHarnessNewProject, "new project", OperationCategoryProject, func() tea.Msg { return CreateProjectMsg{} }},
		{operationHarnessOutliner, "outliner", OperationCategoryNavigation, func() tea.Msg { return m.harnessOpenOutliner(OutlinerOpenOptions{}) }},
		{operationHarnessOutlinerRoot, "outliner root", OperationCategoryNavigation, func() tea.Msg { return m.harnessOpenOutliner(OutlinerOpenOptions{}) }},
		{operationHarnessReference, "reference", OperationCategorySystem, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: m.currentView(),
				ViewName:   "Outliner",
				Items:      m.coreInputHelpItems(),
			}
		}},
		{operationHarnessScope, "choose scope", OperationCategoryScope, func() tea.Msg { return ShowProjectHubMsg{} }},
		{operationHarnessInbox, "global inbox mode", OperationCategoryMode, func() tea.Msg {
			return m.harnessOpenOutliner(OutlinerOpenOptions{SetMode: true, Mode: ModeInbox})
		}},
		{operationHarnessProjectInbox, "project inbox mode", OperationCategoryMode, func() tea.Msg {
			return m.harnessOpenOutliner(OutlinerOpenOptions{SetMode: true, Mode: ModeInbox})
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

func (m *AppModel) harnessOpenOutliner(options OutlinerOpenOptions) tea.Msg {
	presentationCmd := m.prepareCrossProjectOutliner()
	m.crossOutliner.ApplyOpenOptions(options)
	route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err != nil {
		return ErrorMsg{Err: err}
	}
	return OperationCommandResultMsg{Cmd: tea.Batch(m.activateTaskWorkspaceRoute(route), presentationCmd)}
}
