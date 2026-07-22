package tui

// withPresentation associates an implementation renderer with a typed route.
// Route identity and parameters remain authoritative.
func withPresentation(route Route, view PresentationID, projectID, taskID string) Route {
	route.presentation = view
	route.hasPresentation = true
	return route
}

// routeForPresentation resolves a renderer to its canonical typed route.
func routeForPresentation(view PresentationID, projectID, taskID string) (route Route, ok bool, err error) { //nolint:gocyclo
	var id RouteID
	var params RouteParams
	switch view {
	case ViewDashboard, ViewProjectList:
		id, params = Home, HomeParams{}
	case ViewFocusBoard:
		id, params = Planner, PlannerParams{Tab: PlannerToday}
	case ViewOutliner:
		id, params = TaskWorkspace, TaskWorkspaceParams{Scope: ProjectEntityScope(projectID)}
	case ViewTemporaryOutliner:
		id, params = TaskWorkspace, TaskWorkspaceParams{Scope: ProjectEntityScope(projectID)}
	case ViewCrossProjectOutliner:
		id, params = TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()}
	case ViewDailyReview:
		id, params = ReviewHub, ReviewParams{Module: ReviewDaily}
	case ViewWeeklyReview:
		id, params = ReviewHub, ReviewParams{Module: ReviewWeekly}
	case ViewRecentActivity, ViewProjectRecentActivity:
		id, params = ReviewHub, ReviewParams{Module: ReviewActivity}
	case ViewMetricsDashboard:
		id, params = ReviewHub, ReviewParams{Module: ReviewMetrics}
	case ViewRoadmap:
		id, params = ReviewHub, ReviewParams{Module: ReviewRoadmap}
	case ViewInboxProcessor:
		id, params = ManageHub, ManageParams{Module: ManageInbox, Scope: ProjectEntityScope(projectID)}
	case ViewGlobalInboxProcessor:
		id, params = ManageHub, ManageParams{Module: ManageInbox, Scope: AppEntityScope()}
	case ViewNotes:
		id, params = ManageHub, ManageParams{Module: ManageNotes, Scope: ProjectEntityScope(projectID)}
	case ViewGlobalNotes:
		id, params = ManageHub, ManageParams{Module: ManageNotes, Scope: AppEntityScope()}
	case ViewBlockers:
		id, params = ManageHub, ManageParams{Module: ManageBlockers, Scope: ProjectEntityScope(projectID)}
	case ViewGlobalBlockers:
		id, params = ManageHub, ManageParams{Module: ManageBlockers, Scope: AppEntityScope()}
	case ViewWorkflowRules, ViewScoreProfileManager:
		id, params = ManageHub, ManageParams{Module: ManageRules, Scope: AppEntityScope()}
	case ViewTaskTemplates:
		id, params = ManageHub, ManageParams{Module: ManageTemplates, Scope: AppEntityScope()}
	case ViewTagManager:
		id, params = ManageHub, ManageParams{Module: ManageTags, Scope: ProjectEntityScope(projectID)}
	case ViewGlobalTagManager:
		id, params = ManageHub, ManageParams{Module: ManageTags, Scope: AppEntityScope()}
	case ViewNotificationCenter:
		id, params = ManageHub, ManageParams{
			Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationAlerts,
		}
	case ViewMessageLog:
		id, params = ManageHub, ManageParams{
			Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationLog,
		}
	case ViewSettings:
		id, params = ManageHub, ManageParams{Module: ManageSettings, Scope: AppEntityScope()}
	case ViewTaskDetail:
		id, params = TaskDetail, TaskDetailParams{ProjectID: projectID, TaskID: taskID, Tab: TaskOverview}
	case ViewTaskChecklist:
		id, params = TaskDetail, TaskDetailParams{ProjectID: projectID, TaskID: taskID, Tab: TaskChecklist}
	case ViewTaskComments:
		id, params = TaskDetail, TaskDetailParams{ProjectID: projectID, TaskID: taskID, Tab: TaskComments}
	case ViewTaskDecisions:
		id, params = TaskDetail, TaskDetailParams{ProjectID: projectID, TaskID: taskID, Tab: TaskDecisions}
	case ViewTaskAttachments:
		id, params = TaskDetail, TaskDetailParams{ProjectID: projectID, TaskID: taskID, Tab: TaskAttachments}
	case ViewTaskActivity:
		id, params = TaskDetail, TaskDetailParams{ProjectID: projectID, TaskID: taskID, Tab: TaskActivity}
	case ViewHelp:
		id, params = Reference, ReferenceParams{Kind: ReferenceHelp}
	case ViewAsk:
		id, params = Reference, ReferenceParams{Kind: ReferenceAsk}
	case ViewTutor:
		id, params = Reference, ReferenceParams{Kind: ReferenceTutor}
	default:
		return Route{}, false, nil
	}

	route, err = NewRoute(id, params)
	if err != nil {
		return Route{}, true, err
	}
	return withPresentation(route, view, projectID, taskID), true, nil
}

// overlayForPresentation maps a transient renderer to the typed overlay stack.
// Reference presentations are represented as reference overlays.
func overlayForPresentation(view PresentationID, projectID, taskID string) (Overlay, bool, error) {
	if route, ok, err := routeForPresentation(view, projectID, taskID); err != nil {
		return Overlay{}, ok, err
	} else if ok && route.ID() == Reference {
		overlay, overlayErr := NewReferenceOverlay(route)
		if overlayErr == nil {
			overlay.presentation, overlay.hasPresentation = view, true
		}
		return overlay, true, overlayErr
	}

	kind, mapped := presentationOverlayKind(view)
	if !mapped {
		return Overlay{}, false, nil
	}
	overlay, err := NewOverlay(kind, view.String())
	if err != nil {
		return Overlay{}, true, err
	}
	overlay.presentation, overlay.hasPresentation = view, true
	return overlay, true, nil
}

func presentationOverlayKind(view PresentationID) (OverlayKind, bool) {
	switch view {
	case ViewCommandPalette, ViewGlobalSearch:
		return OverlayPalette, true
	case ViewQuickCapture, ViewBulkCapture, ViewBulkEdit, ViewLinkedTaskCreator, ViewTaskForm, ViewTemplateVarFiller:
		return OverlayComposer, true
	case ViewProjectSelector, ViewProjectPicker:
		return OverlayProjectPicker, true
	case ViewSectionManager, ViewColumnPicker:
		return OverlayViewMenu, true
	case ViewFilter, ViewFilterBuilder, ViewTagFilterSelector, ViewSectionFilterSelector, ViewProjectFilterSelector, ViewAttrFilterPicker, ViewNotesSearch:
		return OverlaySearch, true
	case ViewTaskJumper:
		return OverlayTaskJump, true
	case ViewProjectPreview, ViewRecentActivityPreview:
		return OverlayPeek, true
	case ViewProjectCreator, ViewProjectEditor, ViewWorkflowRuleCreator, ViewWorkflowRuleEditor, ViewWorkflowWizard,
		ViewTaskTemplateCreator, ViewTaskTemplateEditor, ViewCustomContextEditor, ViewTextEditor:
		return OverlayForm, true
	case ViewConflictResolution:
		return OverlayConfirmation, true
	case ViewDateInput, ViewDependencySelector, ViewDependencyViewer, ViewSectionPicker,
		ViewPriorityPicker, ViewContextPicker, ViewParentSelector, ViewTagSelector,
		ViewBulkTagSelector, ViewTrashedProjects, ViewRecurrenceEditor,
		ViewLinkedTaskSelector, ViewCrossProjectLinker, ViewNoteTaskLinker,
		ViewNoteTagSelector, ViewSnoozeInput, ViewParkInput, ViewTaskTemplatePicker,
		ViewScoreEditor, ViewSaveFilterInput, ViewScoringProfilePicker,
		ViewWorkflowDryRun, ViewStatusInput, ViewStatusIconPicker:
		return OverlayPicker, true
	default:
		return 0, false
	}
}
