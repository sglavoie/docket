package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Stable global destination IDs. Tool IDs name a live review, manage, or
// reference route; global IDs name a creator, folder action, or maintenance
// action that is not itself a browsable destination.
const (
	operationToolDailyReview     OperationID = "tui.tool.daily-review"
	operationToolWeeklyReview    OperationID = "tui.tool.weekly-review"
	operationToolActivity        OperationID = "tui.tool.activity"
	operationToolMetrics         OperationID = "tui.tool.metrics"
	operationToolRoadmap         OperationID = "tui.tool.roadmap"
	operationToolNotes           OperationID = "tui.tool.notes"
	operationToolBlockers        OperationID = "tui.tool.blockers"
	operationToolWorkflowRules   OperationID = "tui.tool.workflow-rules"
	operationToolScoringProfiles OperationID = "tui.tool.scoring-profiles"
	operationToolTaskTemplates   OperationID = "tui.tool.task-templates"
	operationToolTags            OperationID = "tui.tool.tags"
	operationToolNotifications   OperationID = "tui.tool.notifications"
	operationToolMessageLog      OperationID = "tui.tool.message-log"
	operationToolSettings        OperationID = "tui.tool.settings"
	operationToolContexts        OperationID = "tui.tool.contexts"
	operationToolReference       OperationID = "tui.tool.reference"
	operationToolTutor           OperationID = "tui.tool.tutor"
	operationToolAsk             OperationID = "tui.tool.ask"

	operationGlobalNewProject      OperationID = "tui.global.new-project"
	operationGlobalNewTaskTemplate OperationID = "tui.global.new-task-template"
	operationGlobalNewWorkflowRule OperationID = "tui.global.new-workflow-rule"
	operationGlobalOpenDataFolder  OperationID = "tui.global.open-data-folder"
	operationGlobalOpenLogFolder   OperationID = "tui.global.open-log-folder"
	operationGlobalSandboxReset    OperationID = "tui.global.sandbox-reset"
)

// globalToolDescriptors is the authoritative catalog of live global
// destinations. Every entry is placed in Finder's Tools category, so each tool
// appears there exactly once and nowhere else.
func globalToolDescriptors() []OperationDescriptor {
	tool := func(
		id OperationID,
		label string,
		description string,
		category OperationCategory,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          category,
			Placement:         PlacementFinderTool,
			TargetSource:      OperationTargetGlobal,
			TargetCardinality: TargetCardinalityNone,
			Risk:              OperationRiskReadOnly,
		}
	}
	return []OperationDescriptor{
		tool(operationToolDailyReview, "Daily Review", "Today's plan and triage loop", OperationCategoryReview, "today review"),
		tool(operationToolWeeklyReview, "Weekly Review", "Strategic project health review", OperationCategoryReview),
		tool(operationToolActivity, "Activity", "Recent task and project activity", OperationCategoryReview, "recent activity", "history"),
		tool(operationToolMetrics, "Metrics", "Completion, cycle-time, and throughput analytics", OperationCategoryReview, "analytics"),
		tool(operationToolRoadmap, "Roadmap", "Timelines, forecasts, and critical paths", OperationCategoryReview, "timeline"),
		tool(operationToolNotes, "Notes", "Project and global notes", OperationCategoryNote, "scratch notes"),
		tool(operationToolBlockers, "Blockers", "Blocked and blocking tasks", OperationCategoryTask, "dependencies"),
		tool(operationToolWorkflowRules, "Workflow Rules", "Manage workflow automation rules", OperationCategorySystem, "automation"),
		tool(operationToolScoringProfiles, "Scoring Profiles", "Manage task scoring profiles", OperationCategorySystem, "scoring"),
		tool(operationToolTaskTemplates, "Task Templates", "Manage reusable task templates", OperationCategorySystem, "templates"),
		tool(operationToolTags, "Tags", "Manage tags across projects", OperationCategoryTag),
		tool(operationToolNotifications, "Notifications", "Due, overdue, and workflow alerts", OperationCategoryNotification, "alerts"),
		tool(operationToolMessageLog, "Message Log", "Recent status and error messages", OperationCategoryNotification, "log"),
		tool(operationToolSettings, "Settings", "Application preferences and configuration", OperationCategorySystem, "preferences"),
		tool(operationToolContexts, "Contexts", "Browse and apply project contexts", OperationCategoryScope, "scopes"),
		tool(operationToolReference, "Reference", "Keyboard shortcuts and command reference", OperationCategorySystem, "help", "keyboard shortcuts"),
		tool(operationToolTutor, "Tutor", "Guided walkthrough of the daily loop", OperationCategorySystem, "tutorial"),
		tool(operationToolAsk, "Ask AI", "Ask the configured AI provider about this workspace", OperationCategorySystem, "ai"),
	}
}

// globalActionDescriptors are the non-destination global operations: creators
// that open an existing creator route, folder actions, and sandbox reset.
func globalActionDescriptors() []OperationDescriptor {
	action := func(
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
			TargetSource:      OperationTargetGlobal,
			TargetCardinality: TargetCardinalityNone,
			Risk:              risk,
		}
	}
	return []OperationDescriptor{
		action(operationGlobalNewProject, "New Project", "Create a project", OperationCategoryProject, OperationRiskMutation, "create project"),
		action(operationGlobalNewTaskTemplate, "New Task Template", "Create a task template", OperationCategorySystem, OperationRiskMutation, "create template"),
		action(operationGlobalNewWorkflowRule, "New Workflow Rule", "Create a workflow automation rule", OperationCategorySystem, OperationRiskMutation, "create rule"),
		action(operationGlobalOpenDataFolder, "Open Data Folder", "Reveal the workspace data directory", OperationCategorySystem, OperationRiskReadOnly, "reveal data"),
		action(operationGlobalOpenLogFolder, "Open Log Folder", "Reveal the machine-local log directory", OperationCategorySystem, OperationRiskReadOnly, "reveal logs"),
		action(operationGlobalSandboxReset, "Reset Sandbox", "Delete every sandbox task", OperationCategorySystem, OperationRiskDestructive, "clear sandbox"),
	}
}

// registerGlobalOperations binds every global destination and action to the one
// descriptor-driven dispatcher. Sandbox reset delegates confirmation because the
// sandbox route already previews the exact item count before deleting anything.
func (m *AppModel) registerGlobalOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	descriptors := append(globalToolDescriptors(), globalActionDescriptors()...)
	for _, descriptor := range descriptors {
		if err := m.registerOperation(descriptor, OperationBinding{
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
			Handler:                  m.executeGlobalOperation,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeGlobalOperation(invocation OperationInvocation) OperationHandlerResult {
	message := m.globalOperationMessage(invocation.Descriptor.ID, invocation.Context)
	if message == nil {
		return OperationHandlerResult{
			Err: fmt.Errorf("global operation %s has no route activation", invocation.Descriptor.ID),
		}
	}
	return OperationHandlerResult{Message: message}
}

// globalOperationMessage maps a global operation onto its typed route
// activation. Project scope comes from the frozen context, never from the live
// cursor, so a Finder invocation opens the scope the user was actually in.
//
//nolint:gocyclo // One auditable switch keeps the global destination catalog in a single place.
func (m *AppModel) globalOperationMessage(id OperationID, ctx CommandContext) tea.Msg {
	project := m.projectByID(ctx.FocusedProjectID)
	projectSet := ActiveProjectSetScope(ctx.ScopeProjectIDs)
	//nolint:exhaustive // Only global operations are registered with this handler.
	switch id {
	case operationToolDailyReview:
		return ShowDailyReviewMsg{}
	case operationToolWeeklyReview:
		return ShowWeeklyReviewMsg{ProjectSet: projectSet}
	case operationToolActivity:
		if project != nil {
			return ShowProjectRecentActivityMsg{Project: project}
		}
		return ShowRecentActivityMsg{}
	case operationToolMetrics:
		return ShowMetricsDashboardMsg{Project: project}
	case operationToolRoadmap:
		return ShowRoadmapMsg{Project: project}
	case operationToolNotes:
		if project != nil {
			return ShowNotesMsg{Project: project}
		}
		return ShowGlobalNotesMsg{}
	case operationToolBlockers:
		if project != nil {
			return ShowBlockersMsg{Project: project}
		}
		return ShowGlobalBlockersMsg{}
	case operationToolWorkflowRules:
		return ShowWorkflowRulesMsg{}
	case operationToolScoringProfiles:
		return ShowScoreProfileManagerMsg{ProjectID: ctx.FocusedProjectID}
	case operationToolTaskTemplates:
		return ShowTaskTemplatesMsg{}
	case operationToolTags:
		if project != nil {
			return ShowTagManagerMsg{Project: project}
		}
		return ShowGlobalTagManagerMsg{ProjectSet: projectSet}
	case operationToolNotifications:
		return ShowNotificationCenterMsg{}
	case operationToolMessageLog:
		return ShowMessageLogMsg{}
	case operationToolSettings:
		return ShowManageHubMsg{Params: ManageParams{Module: ManageSettings, Scope: AppEntityScope()}}
	case operationToolContexts:
		return ShowManageHubMsg{Params: ManageParams{Module: ManageContexts, Scope: AppEntityScope()}}
	case operationToolReference:
		return ShowHelpMsg{
			SourceView: m.currentView(),
			ViewName:   "Outliner",
			Items:      m.coreInputHelpItems(),
		}
	case operationToolTutor:
		return ShowTutorMsg{}
	case operationToolAsk:
		return ShowAskMsg{}
	case operationGlobalNewProject:
		return CreateProjectMsg{}
	case operationGlobalNewTaskTemplate:
		return ShowTaskTemplateCreatorMsg{}
	case operationGlobalNewWorkflowRule:
		return ShowWorkflowRuleCreatorMsg{}
	case operationGlobalOpenDataFolder:
		return OpenDataFolderMsg{Project: project}
	case operationGlobalOpenLogFolder:
		return OpenLogDirMsg{}
	case operationGlobalSandboxReset:
		return SandboxResetRequestMsg{}
	default:
		return nil
	}
}

// globalToolCatalog projects the registered tool descriptors applicable in ctx
// into Finder's Tools category.
func globalToolCatalog(registry *OperationRegistry, ctx CommandContext) []FinderTool {
	if registry == nil {
		return nil
	}
	descriptors, err := registry.ForPlacement(ctx, PlacementFinderTool)
	if err != nil {
		return nil
	}
	tools := make([]FinderTool, 0, len(descriptors))
	for _, descriptor := range descriptors {
		tools = append(tools, FinderTool{
			ID:          string(descriptor.ID),
			Label:       descriptor.Label,
			Description: descriptor.Description,
			Aliases:     descriptor.Aliases,
		})
	}
	return tools
}
