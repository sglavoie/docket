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
	operationHarnessOutlineErrors   OperationID = "tui.harness.outline-errors"
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
		{operationHarnessOutlineErrors, "outline errors", OperationCategorySystem, func() tea.Msg {
			// The outline refusal surface is only reachable through an external
			// editor round trip, which no harness action can replay: editors are
			// disabled and the `command` action carries no buffer payload. Scenarios
			// therefore install it directly with a canned problem list.
			m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(
				"Rich Project",
				"/tmp/tt-outline-scenario.md",
				harnessOutlineProblems(),
			))
			return nil
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

// harnessOutlineProblems is one rejected buffer's worth of problems, holding
// both validation phases and a message long enough to need wrapping at narrow
// widths.
func harnessOutlineProblems() []lineError {
	return []lineError{
		{line: 4, message: "indent of 3 spaces is not a multiple of 2"},
		{line: 6, message: "marker [[tt:deadbeef]] does not name a task in this project"},
		{line: 9, message: "indent jumps from level 0 to level 2; indent one level at a time"},
		{line: 11, message: "task title cannot be empty"},
		{line: 12, message: "indentation sets parentage here; remove the >parent sigil"},
	}
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
