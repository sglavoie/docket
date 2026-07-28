package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

const (
	operationHarnessExportProject   OperationID = "tui.harness.export-project"
	operationHarnessMetrics         OperationID = "tui.harness.metrics"
	operationHarnessRoadmap         OperationID = "tui.harness.roadmap"
	operationHarnessNewProject      OperationID = "tui.harness.new-project"
	operationHarnessReset           OperationID = "tui.harness.reset"
	operationHarnessOutliner        OperationID = "tui.harness.outliner"
	operationHarnessReference       OperationID = "tui.harness.reference"
	operationHarnessSettings        OperationID = "tui.harness.settings"
	operationHarnessNewTemplate     OperationID = "tui.harness.new-template"
	operationHarnessNewWorkflowRule OperationID = "tui.harness.new-workflow-rule"
	operationHarnessWorkflowRules   OperationID = "tui.harness.workflow-rules"
	operationHarnessTaskTemplates   OperationID = "tui.harness.task-templates"
	operationHarnessScoringProfiles OperationID = "tui.harness.scoring-profiles"
	operationHarnessFinder          OperationID = "tui.harness.finder"
	operationHarnessScope           OperationID = "tui.harness.scope"
	operationHarnessInbox           OperationID = "tui.harness.inbox"
	operationHarnessProjectInbox    OperationID = "tui.harness.project-inbox"
	operationHarnessDailyReview     OperationID = "tui.harness.daily-review"
	operationHarnessMessageLog      OperationID = "tui.harness.message-log"
	operationHarnessRenameProject   OperationID = "tui.harness.rename-project"
	operationHarnessTaskDetail      OperationID = "tui.harness.task-detail"
	operationHarnessTaskAttachments OperationID = "tui.harness.task-attachments"
	operationHarnessTaskComments    OperationID = "tui.harness.task-comments"
	operationHarnessTaskDecisions   OperationID = "tui.harness.task-decisions"
	operationHarnessTaskActivity    OperationID = "tui.harness.task-activity"
	operationHarnessProjectNotes    OperationID = "tui.harness.project-notes"
	operationHarnessNotesSearch     OperationID = "tui.harness.notes-search"
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
// links never appear in Finder, help, footers, or available-command output.
func (m *AppModel) registerHarnessOperations() error {
	specs := []harnessOperationSpec{
		{operationHarnessExportProject, "export project", OperationCategoryProject, func() tea.Msg { return ExportProjectMsg{} }},
		{operationHarnessMetrics, "metrics dashboard", OperationCategoryReview, func() tea.Msg { return ShowMetricsDashboardMsg{} }},
		{operationHarnessRoadmap, "roadmap", OperationCategoryReview, func() tea.Msg { return ShowRoadmapMsg{} }},
		{operationHarnessNewProject, "new project", OperationCategoryProject, func() tea.Msg { return CreateProjectMsg{} }},
		{operationHarnessReset, "outliner root", OperationCategoryNavigation, func() tea.Msg { return JumpDashboardMsg{} }},
		{operationHarnessOutliner, "outliner", OperationCategoryNavigation, func() tea.Msg { return ShowGlobalOutlinerMsg{} }},
		{operationHarnessReference, "reference", OperationCategorySystem, func() tea.Msg { return ShowTutorMsg{} }},
		{operationHarnessSettings, "settings", OperationCategorySystem, func() tea.Msg { return SettingsMsg{} }},
		{operationHarnessNewTemplate, "create task template", OperationCategoryTask, func() tea.Msg { return ShowTaskTemplateCreatorMsg{} }},
		{operationHarnessNewWorkflowRule, "new workflow rule", OperationCategorySystem, func() tea.Msg { return ShowWorkflowRuleCreatorMsg{} }},
		{operationHarnessWorkflowRules, "workflow rules", OperationCategorySystem, func() tea.Msg { return ShowWorkflowRulesMsg{} }},
		{operationHarnessTaskTemplates, "task templates", OperationCategoryTask, func() tea.Msg { return ShowTaskTemplatesMsg{} }},
		{operationHarnessScoringProfiles, "scoring profiles", OperationCategorySystem, func() tea.Msg { return ShowScoreProfileManagerMsg{} }},
		{operationHarnessFinder, "finder", OperationCategoryNavigation, func() tea.Msg { return ShowFinderMsg{} }},
		{operationHarnessScope, "choose scope", OperationCategoryScope, func() tea.Msg { return ShowContextHubMsg{} }},
		{operationHarnessInbox, "global inbox mode", OperationCategoryMode, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeInbox}}
		}},
		{operationHarnessProjectInbox, "project inbox mode", OperationCategoryMode, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeInbox}}
		}},
		{operationHarnessDailyReview, "daily review", OperationCategoryMode, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeDailyReview}}
		}},
		{operationHarnessMessageLog, "message log", OperationCategoryNotification, func() tea.Msg { return ShowMessageLogMsg{} }},
		{operationHarnessRenameProject, "rename project", OperationCategoryProject, func() tea.Msg {
			_, project := m.outlinerTarget()
			if project == nil {
				return nil
			}
			return EditProjectMsg{Project: project}
		}},
		{operationHarnessTaskDetail, "view task details", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return ShowTaskDetailMsg{Task: task, Project: project}
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
		{operationHarnessTaskDecisions, "manage decisions", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return ShowTaskDecisionsMsg{Task: task, Project: project}
		}},
		{operationHarnessTaskActivity, "task activity", OperationCategoryTask, func() tea.Msg {
			task, project := m.outlinerTarget()
			if task == nil || project == nil {
				return nil
			}
			return OpenTaskActivityMsg{Task: task, Project: project}
		}},
		{operationHarnessProjectNotes, "show notes", OperationCategoryNote, func() tea.Msg {
			return m.globalOperationMessage(operationToolNotes, m.operationCommandContext())
		}},
		{operationHarnessNotesSearch, "search notes", OperationCategoryNote, func() tea.Msg {
			return syntheticKey('/')
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
