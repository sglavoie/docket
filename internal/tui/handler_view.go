package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// handleViewMessages handles view navigation and display messages.
// Returns true if the message was handled.
func (m *AppModel) handleViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate search messages to dedicated handler
	if handled, cmd := m.handleSearchMessages(msg); handled {
		return handled, cmd
	}

	// Delegate saved filter messages to dedicated handler
	if handled, cmd := m.handleSavedFilterMessages(msg); handled {
		return handled, cmd
	}

	// Delegate notes messages to dedicated handler
	if handled, cmd := m.handleNotesMessages(msg); handled {
		return handled, cmd
	}

	// Delegate context messages to dedicated handler
	if handled, cmd := m.handleContextMessages(msg); handled {
		return handled, cmd
	}

	// Delegate scoring messages to dedicated handler
	if handled, cmd := m.handleScoringMessages(msg); handled {
		return handled, cmd
	}

	// Delegate editor messages to dedicated handler
	if handled, cmd := m.handleEditorMessages(msg); handled {
		return handled, cmd
	}

	switch msg := msg.(type) {
	case BackMsg:
		m.popView()
		return true, nil

	case RefreshMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)

	case NavigateToViewMsg:
		return m.handleNavigateToView(msg)

	case SettingsMsg:
		m.showView(ViewSettings, NewSettingsModel(m.config, m.configManager))
		return true, nil

	case SettingsStagesMsg:
		sm := NewSettingsModel(m.config, m.configManager)
		sm.blurCurrentField()
		sm.tabs.Active = 7
		sm.focusIndex = tabFieldStart[7]
		sm.focusCurrentField()
		m.showView(ViewSettings, sm)
		return true, nil

	case SettingsSavedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowWorkflowRulesMsg:
		m.showView(ViewWorkflowRules, NewWorkflowRulesModel(m.config, m.configManager))
		return true, nil

	case ShowTaskTemplatesMsg:
		m.showView(ViewTaskTemplates, NewTaskTemplatesModel(m.config, m.configManager))
		return true, nil

	case TaskTemplatesUpdatedMsg:
		return true, nil

	case ShowTaskTemplateCreatorMsg:
		creator := NewTaskTemplateCreatorModel(m.config, m.configManager)
		m.showView(ViewTaskTemplateCreator, creator)
		return true, creator.Init()

	case ShowTaskTemplateEditorMsg:
		editor := NewTaskTemplateEditorModel(m.config, m.configManager, msg.Template)
		m.showView(ViewTaskTemplateEditor, editor)
		return true, editor.Init()

	case TaskTemplateCreatedMsg:
		m.popView()
		// Refresh the templates list if it's visible by replacing it in the registry
		if m.currentView() == ViewTaskTemplates {
			templatesView := NewTaskTemplatesModel(m.config, m.configManager)
			templatesView.SetSize(m.width, m.height-2)
			m.views.Register(ViewTaskTemplates, templatesView)
		}
		return true, nil

	case TaskTemplateEditedMsg:
		m.popView()
		// Refresh the templates list if it's visible by replacing it in the registry
		if m.currentView() == ViewTaskTemplates {
			templatesView := NewTaskTemplatesModel(m.config, m.configManager)
			templatesView.SetSize(m.width, m.height-2)
			m.views.Register(ViewTaskTemplates, templatesView)
		}
		return true, nil

	case ShowTemplatePickerMsg:
		if msg.Project != nil {
			picker := NewTemplatePickerModel(m.config, msg.Project)
			m.showView(ViewTaskTemplatePicker, picker)
			return true, picker.Init()
		}
		return true, nil

	case TemplatePickerDoneMsg:
		m.popView()

		if msg.Canceled {
			m.pendingCreateTaskParent = nil
			return true, nil
		}
		// If template has variables, show the var filler before task creator
		if msg.Template != nil && template.HasVariables(msg.Template) {
			m.pendingVarFillerTemplate = msg.Template
			filler := NewTemplateVarFillerModel(msg.Template)
			m.showView(ViewTemplateVarFiller, filler)
			return true, filler.Init()
		}
		// No variables, go directly to task creator
		parentTask := m.pendingCreateTaskParent
		m.pendingCreateTaskParent = nil
		if m.currentProject != nil {
			creator := NewTaskCreatorModelWithTemplate(m.currentProject, m.config, m.configManager, parentTask, msg.Template)
			m.showView(ViewTaskCreator, creator)
			return true, creator.Init()
		}
		return true, nil

	case TemplateVarFillerDoneMsg:
		m.popView()
		if msg.Canceled {
			// Return to template picker so user can pick a different template
			m.pendingVarFillerTemplate = nil
			if m.currentProject != nil {
				picker := NewTemplatePickerModel(m.config, m.currentProject)
				m.showView(ViewTaskTemplatePicker, picker)
				return true, picker.Init()
			}
			return true, nil
		}
		// Proceed to task creator with resolved template
		parentTask := m.pendingCreateTaskParent
		m.pendingCreateTaskParent = nil
		m.pendingVarFillerTemplate = nil
		if m.currentProject != nil {
			creator := NewTaskCreatorModelWithTemplate(m.currentProject, m.config, m.configManager, parentTask, msg.Template)
			m.showView(ViewTaskCreator, creator)
			return true, creator.Init()
		}
		return true, nil

	case WorkflowRulesUpdatedMsg:
		return true, nil

	case ShowWorkflowRuleCreatorMsg:
		creator := NewWorkflowRuleCreatorModel(m.config, m.configManager)
		m.showView(ViewWorkflowRuleCreator, creator)
		return true, creator.Init()

	case ShowWorkflowRuleEditorMsg:
		editor := NewWorkflowRuleEditorModel(m.config, m.configManager, msg.Rule)
		m.showView(ViewWorkflowRuleEditor, editor)
		return true, editor.Init()

	case WorkflowRuleCreatedMsg:
		m.popView()
		// Refresh the rules list if it's visible by replacing it in the registry
		if m.currentView() == ViewWorkflowRules {
			rulesView := NewWorkflowRulesModel(m.config, m.configManager)
			rulesView.SetSize(m.width, m.height-2)
			m.views.Register(ViewWorkflowRules, rulesView)
		}
		return true, nil

	case WorkflowRuleEditedMsg:
		m.popView()
		// Refresh the rules list if it's visible by replacing it in the registry
		if m.currentView() == ViewWorkflowRules {
			rulesView := NewWorkflowRulesModel(m.config, m.configManager)
			rulesView.SetSize(m.width, m.height-2)
			m.views.Register(ViewWorkflowRules, rulesView)
		}
		return true, nil

	case ShowWorkflowDryRunMsg:
		dryRun := NewWorkflowDryRunModel(m.config, m.configManager, m.persister, msg.SingleRule)
		m.showView(ViewWorkflowDryRun, dryRun)
		return true, dryRun.Init()

	case ShowFilterBuilderMsg:
		builder := NewFilterBuilderModel(m.config, m.configManager)
		m.showView(ViewFilterBuilder, builder)
		return true, builder.Init()

	case ShowFilterEditorMsg:
		builder := NewFilterEditorModel(m.config, m.configManager, msg.Filter)
		m.showView(ViewFilterBuilder, builder)
		return true, builder.Init()

	case FilterBuilderSavedMsg:
		m.popView()
		// Rebuild AllTasks tab entries if it's on the stack
		if m.allTasksView != nil {
			m.allTasksView.buildTabEntries()
			m.allTasksView.loadAllTasks()
		}
		return true, nil

	case ShowHelpMsg:
		m.helpView = NewHelpViewModel(msg.Items, msg.SourceView, msg.ViewName)
		m.showView(ViewHelp, m.helpView)
		return true, nil

	case CloseHelpMsg:
		m.popView()
		return true, nil

	case ShowFilterMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewFilter, NewFilterViewModel(msg.ProjectTree))
		} else if msg.CommonFilter != nil {
			m.showView(ViewFilter, NewFilterViewModelForOutliner(msg.CommonFilter, msg.Project))
		}
		return true, nil

	case CloseFilterMsg:
		m.popView()
		return true, nil

	case ShowTaskJumperMsg:
		if msg.Project != nil {
			m.taskJumper = NewTaskJumperModel(msg.Project)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		return true, nil

	case TaskJumperSelectMsg:
		m.popView()
		if msg.Task != nil {
			switch m.currentView() {
			case ViewOutliner:
				if m.outliner != nil {
					m.outliner.NavigateToSidebarTask(msg.Task)
				}
			default:
				if m.projectTree != nil {
					return true, m.projectTree.NavigateToTask(msg.Task)
				}
			}
		}
		return true, nil

	case CloseTaskJumperMsg:
		m.popView()
		return true, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectPreview, NewProjectPreviewModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case CloseProjectPreviewMsg:
		m.popView()
		return true, nil

	case ShowStatusInputMsg:
		if msg.Project != nil {
			m.showView(ViewStatusInput, NewStatusInputModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ShowStatusIconPickerMsg:
		if msg.Project != nil {
			m.showView(ViewStatusIconPicker, NewStatusIconPickerModel(msg.Project))
		}
		return true, nil

	case StatusIconSelectedMsg:
		m.popView() // pop icon picker
		if msg.Project == nil {
			return true, nil
		}
		if !msg.Canceled {
			msg.Project.StatusIcon = msg.Key
			if msg.Key != "" {
				msg.Project.StatusIconSource = "user"
			} else {
				msg.Project.StatusIconSource = ""
			}
			msg.Project.UpdatedAt = time.Now()
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
		}
		// Proceed to text input regardless (cancel skips icon change, not text input)
		m.showView(ViewStatusInput, NewStatusInputModel(msg.Project, m.config, m.configManager))
		return true, nil

	case StatusUpdatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.loadProjects()
		}
		m.setSuccessStatus("Status updated")
		return true, defaultStatusTimeout()

	case ShowDueDateOverviewMsg:
		m.showView(ViewDueDateOverview, NewDueDateOverviewModel(m.config, m.configManager))
		return true, nil

	case CloseDueDateOverviewMsg:
		m.popView()
		return true, nil

	case ShowMetricsDashboardMsg:
		if msg.Project != nil {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.config, msg.Project)
		} else {
			m.metricsDashboard = NewMetricsDashboardModel(m.config)
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
		return true, nil

	case CloseMetricsDashboardMsg:
		m.popToAnchorView()
		return true, nil

	case MetricsDashboardSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowDailyReviewMsg:
		m.showView(ViewDailyReview, NewDailyReviewModel(m.config, m.configManager))
		return true, nil

	case CloseDailyReviewMsg:
		m.popView()
		return true, nil

	case MarkDailyReviewDoneMsg:
		m.config.SetLastDailyReviewAt(time.Now())
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Daily review completed")
		return true, nil

	case MarkWeeklyReviewDoneMsg:
		m.config.SetLastWeeklyReviewAt(time.Now())
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review completed")
		return true, nil

	case ResetDailyReviewMsg:
		m.config.ClearLastDailyReviewAt()
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Daily review reset")
		return true, nil

	case ResetWeeklyReviewMsg:
		m.config.ClearLastWeeklyReviewAt()
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review reset")
		return true, nil

	case ShowTutorMsg:
		m.showView(ViewTutor, NewTutorModel())
		return true, nil

	case ShowAskMsg:
		askView := NewAskViewModel(m.currentProject, m.cliReference)
		m.showView(ViewAsk, askView)
		return true, askView.Init()

	case ShowWeeklyReviewMsg:
		m.showView(ViewWeeklyReview, NewWeeklyReviewModel(m.config, m.configManager))
		return true, nil

	case CloseWeeklyReviewMsg:
		m.popView()
		return true, nil

	case WeeklyReviewSelectProjectMsg:
		m.popView()
		if msg.Project != nil {
			return true, m.navigateToTaskInProject(nil, msg.Project)
		}
		return true, nil

	case WeeklyReviewSelectTaskMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case DailyReviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowAllTasksMsg:
		m.showView(ViewAllTasks, NewAllTasksViewModel(m.config, m.configManager))
		return true, nil

	case CloseAllTasksMsg:
		m.popToAnchorView()
		return true, nil

	case AllTasksSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowFocusBoardMsg:
		m.reevaluateAlerts()
		m.showView(ViewFocusBoard, NewFocusBoardModel(m.config, m.configManager))
		return true, nil

	case CloseFocusBoardMsg:
		m.popToAnchorView()
		return true, nil

	case FocusBoardSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case FocusBoardRefreshMsg:
		if v := m.views.Get(ViewFocusBoard); v != nil {
			v.Update(msg)
		}
		return true, nil

	case ShowFocusViewMsg:
		m.showView(ViewFocusView, NewFocusViewModel(m.config, m.configManager, &m.focusQueue, m.focusQueueIDs, m.focusDismissedIDs))
		return true, nil

	case FocusViewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowRecentActivityMsg:
		m.showView(ViewRecentActivity, NewRecentActivityModel(m.config))
		return true, nil

	case CloseRecentActivityMsg:
		m.popToAnchorView()
		return true, nil

	case ShowProjectRecentActivityMsg:
		if msg.Project != nil {
			m.showView(ViewProjectRecentActivity, NewProjectRecentActivityModel(m.config, msg.Project))
		}
		return true, nil

	case CloseProjectRecentActivityMsg:
		m.popView()
		return true, nil

	case ShowRecentActivityPreviewMsg:
		preview := NewRecentActivityPreviewModel(msg.Task, msg.Project)
		m.showView(ViewRecentActivityPreview, preview)
		return true, nil

	case CloseRecentActivityPreviewMsg:
		m.popView()
		return true, nil

	case RecentActivitySelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowProjectFocusViewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectFocusView, NewProjectFocusViewModel(msg.Project))
		}
		return true, nil

	case CloseProjectFocusViewMsg:
		m.popView()
		return true, nil

	case DueDateOverviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.currentProject = msg.Project
			return true, func() tea.Msg {
				return GlowViewMsg{Task: msg.Task}
			}
		}
		return true, nil

	case StatusClearMsg:
		m.statusMessage = ""
		m.statusIsInfo = false
		return true, nil

	case StatusMsg:
		m.statusMessage = string(msg)
		m.statusIsInfo = false
		return true, defaultStatusTimeout()

	case InfoStatusMsg:
		m.statusMessage = string(msg)
		m.statusIsInfo = true
		return true, defaultStatusTimeout()

	case MoveBlockedMsg:
		m.statusMessage = msg.Reason
		m.statusIsInfo = true
		return true, defaultStatusTimeout()

	case ShowPriorityViewMsg:
		if msg.Project != nil {
			m.showView(ViewPriorityView, NewPriorityViewModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ClosePriorityViewMsg:
		m.popView()
		return true, nil

	case PriorityViewSelectMsg:
		m.popView()
		if msg.Task != nil && m.currentProject != nil {
			return true, m.navigateToTaskInProject(msg.Task, m.currentProject)
		}
		return true, nil

	case ShowQuickCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewQuickCapture, NewQuickCaptureModel(msg.Project, msg.AllProjects))
		}
		return true, nil

	case CloseQuickCaptureMsg:
		m.popView()
		return true, nil

	case ShowBulkCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewBulkCapture, NewBulkCaptureModel(msg.Project))
		}
		return true, nil

	case CloseBulkCaptureMsg:
		m.popView()
		return true, nil

	case ShowBulkEditMsg:
		if len(msg.Tasks) > 0 && msg.Project != nil {
			m.showView(ViewBulkEdit, NewBulkEditModel(msg.Tasks, msg.Project))
		}
		return true, nil

	case BulkEditDoneMsg:
		m.popView()
		if msg.Applied && m.projectTree != nil {
			return true, m.projectTree.applyBulkChanges(msg.Changes, msg.Tasks)
		}
		return true, nil

	case PaletteBulkPriorityMsg:
		// Set priority for multiple tasks from palette - show priority picker
		if len(msg.Tasks) > 0 {
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil

	case PaletteBulkTagMsg:
		// Add tags to multiple tasks from palette
		if len(msg.Tasks) > 0 {
			firstProject := msg.Tasks[0].Project
			var tasks []*models.Task
			for _, item := range msg.Tasks {
				if item.Project.ID == firstProject.ID {
					tasks = append(tasks, item.Task)
				}
			}
			if len(tasks) > 0 {
				m.showView(ViewBulkTagSelector, NewBulkTagSelectorModel(tasks, firstProject))
			}
		}
		return true, nil

	case PaletteBulkDueDateMsg:
		// Set due date for multiple tasks from palette
		if len(msg.Tasks) > 0 {
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewDateInput, NewDateInputModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil

	case ShowBlockersMsg:
		if msg.Project != nil {
			editor := m.config.Editor
			m.showView(ViewBlockers, NewBlockersModelWithEditor(msg.Project, editor))
		}
		return true, nil

	case CloseBlockersMsg:
		m.popView()
		return true, nil

	case ShowGlobalBlockersMsg:
		m.showView(ViewGlobalBlockers, NewGlobalBlockersModel(m.config))
		return true, nil

	case CloseGlobalBlockersMsg:
		m.popToAnchorView()
		return true, nil

	case BlockerGoToProjectMsg:
		m.popView()
		if msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.pushView(ViewProjectTree)
		}
		return true, nil

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowTaskAttachmentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(msg.Task, msg.Project, m.config))
		}
		return true, nil

	case CloseTaskAttachmentsMsg:
		m.popView()
		return true, nil

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowOutlinerMsg:
		if msg.Project != nil {
			if msg.Temporary {
				// Drill-down from cross-project outliner: push a fresh temporary
				// OutlinerModel without touching m.outliner so the existing
				// single-project session is preserved.
				fresh := NewOutlinerModel(msg.Project, m.config, m.configManager)
				fresh.OpenAtTask = msg.OpenAtTask
				m.showView(ViewTemporaryOutliner, fresh)
				return true, nil
			}
			replacing := m.isOnProjectView()
			// Extract shared state from tree before creating new outliner
			var shared *sharedViewState
			if replacing && m.projectTree != nil {
				shared = m.extractSharedStateFromTree()
			}
			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
			m.outliner = NewOutlinerModel(msg.Project, m.config, m.configManager)
			m.restoreOutlinerToggleState()
			if shared != nil {
				m.applySharedStateToOutliner(shared)
			} else {
				m.restoreOutlinerFilterState()
			}
			if replacing {
				m.replaceView(ViewOutliner, m.outliner)
			} else {
				m.showView(ViewOutliner, m.outliner)
			}
		}
		return true, nil

	case CloseOutlinerMsg:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.popView()
		m.outliner = nil
		return true, nil

	case ShowGlobalOutlinerMsg:
		if m.crossOutliner == nil {
			m.crossOutliner = NewOutlinerModel(nil, m.config, m.configManager)
			m.showView(ViewCrossProjectOutliner, m.crossOutliner)
			return true, m.crossOutliner.Init()
		}
		// If already viewing the cross-project outliner, reload in-place without
		// pushing a duplicate onto the view stack.
		if m.currentView() == ViewCrossProjectOutliner {
			return true, m.crossOutliner.reloadCrossProjects()
		}
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return true, m.crossOutliner.reloadCrossProjects()

	case CloseGlobalOutlinerMsg:
		m.popToAnchorView()
		return true, nil

	// CloseCrossOutlinerMsg produces identical runtime behavior to
	// CloseGlobalOutlinerMsg (pop to anchor, preserve m.crossOutliner) but
	// is a semantically distinct message so callers can target the
	// cross-project outliner specifically without coupling to the global one.
	case CloseCrossOutlinerMsg:
		m.popToAnchorView()
		return true, nil

	case ShowProjectTreeMsg:
		if msg.Project != nil {
			replacing := m.isOnProjectView()
			if replacing {
				// Extract shared state from outliner before rebuilding tree
				var shared *sharedViewState
				if m.outliner != nil {
					shared = m.extractSharedStateFromOutliner()
				}
				m.saveOutlinerToggleState()
				m.saveOutlinerFilterState()
				m.refreshProjectTree()
				if shared != nil {
					m.applySharedStateToTree(shared)
				}
				m.replaceView(ViewProjectTree, m.projectTree)
			} else {
				if m.projectTree == nil {
					m.switchToProject(msg.Project, true)
				}
				m.showView(ViewProjectTree, m.projectTree)
			}
		}
		return true, nil

	case ShowNotificationCenterMsg:
		m.reevaluateAlerts()
		m.showView(ViewNotificationCenter, NewNotificationCenterModel(m.proactiveAlerts, m.rawAlerts, m.dismissedSet))
		return true, nil

	case AlertCenterNavigateMsg:
		m.popView() // close alert center
		switch msg.Category {
		case alerts.CategoryDailyReview:
			m.showView(ViewDailyReview, NewDailyReviewModel(m.config, m.configManager))
			return true, nil
		case alerts.CategoryWeeklyReview:
			m.showView(ViewWeeklyReview, NewWeeklyReviewModel(m.config, m.configManager))
			return true, nil
		}
		if msg.ProjectID == "" {
			return true, nil
		}
		project := m.config.GetProject(msg.ProjectID)
		if project == nil {
			return true, nil
		}
		if msg.TaskID != "" {
			task := project.GetTaskByID(msg.TaskID)
			if task == nil {
				// Task may not be loaded yet
				_ = m.persister.Load(project)
				task = project.GetTaskByID(msg.TaskID)
			}
			if task != nil {
				return true, m.navigateToTaskInProject(task, project)
			}
		}
		return true, m.navigateToTaskInProject(nil, project)

	case DismissAlertMsg:
		m.config.DismissAlert(msg.Key, msg.Category)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.refreshNotificationCenter()
		m.setSuccessStatus("Alert dismissed")
		return true, defaultStatusTimeout()

	case UndismissAlertMsg:
		m.config.UndismissAlert(msg.Key)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.refreshNotificationCenter()
		m.setSuccessStatus("Alert restored")
		return true, defaultStatusTimeout()

	case CloseNotificationCenterMsg:
		m.popView()
		return true, nil
	}
	return false, nil
}
