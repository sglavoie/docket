package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/template"
)

// handleViewSettingsMessages handles settings, workflows, templates, and filter builder views.
func (m *AppModel) handleViewSettingsMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case SettingsMsg:
		m.showView(ViewSettings, NewSettingsModel(m.config, m.configManager))
		return true, nil

	case SettingsStagesMsg:
		sm := NewSettingsModel(m.config, m.configManager)
		sm.blurCurrentField()
		sm.tabs.Active = tabStages
		sm.subTab = 0
		sm.focusIndex = tabFieldStart[tabStages]
		sm.syncVisualFromOld()
		sm.focusCurrentField()
		m.showView(ViewSettings, sm)
		return true, nil

	case SettingsSavedMsg:
		m.popView()
		ApplyTheme(NewThemeConfig(m.config))
		icons.SetUseEmoji(m.config.GetUseEmoji())
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		if m.config.GetAutoTheme() {
			return true, tea.Tick(appearanceCheckInterval, func(time.Time) tea.Msg {
				return systemAppearanceCheckMsg{}
			})
		}
		return true, nil

	case presetSavedMsg:
		m.setSuccessStatus("Preset saved: " + msg.name)
		return true, defaultStatusTimeout()

	case presetLoadedMsg:
		m.setSuccessStatus("Preset loaded: " + msg.name)
		return true, defaultStatusTimeout()

	case presetResetMsg:
		m.setInfoStatus("Palette slot reset to defaults")
		return true, defaultStatusTimeout()

	case presetErrMsg:
		m.err = fmt.Errorf("preset error: %s", msg.err)
		return true, statusTimeout(5 * time.Second)

	case ShowWorkflowRulesMsg:
		m.showView(ViewWorkflowRules, NewWorkflowRulesModel(m.config, m.configManager))
		return true, nil

	case ShowTaskTemplatesMsg:
		m.showView(ViewTaskTemplates, NewTaskTemplatesModel(m.config, m.configManager))
		return true, nil

	case TaskTemplatesUpdatedMsg:
		return true, nil

	case ShowTaskTemplateCreatorMsg:
		return m.showViewInit(ViewTaskTemplateCreator, NewTaskTemplateCreatorModel(m.config, m.configManager))

	case ShowTaskTemplateEditorMsg:
		return m.showViewInit(ViewTaskTemplateEditor, NewTaskTemplateEditorModel(m.config, m.configManager, msg.Template))

	case TaskTemplateCreatedMsg:
		m.popView()
		m.refreshTaskTemplatesView()
		return true, nil

	case TaskTemplateEditedMsg:
		m.popView()
		m.refreshTaskTemplatesView()
		return true, nil

	case ShowTemplatePickerMsg:
		if msg.Project != nil {
			return m.showViewInit(ViewTaskTemplatePicker, NewTemplatePickerModel(m.config, msg.Project))
		}
		return true, nil

	case TemplatePickerDoneMsg:
		m.popView()

		if msg.Canceled {
			m.pendingCreateTaskParent = nil
			return true, nil
		}

		if msg.Template != nil && template.HasVariables(msg.Template) {
			m.pendingVarFillerTemplate = msg.Template
			return m.showViewInit(ViewTemplateVarFiller, NewTemplateVarFillerModel(msg.Template))
		}

		return m.showTaskCreatorForTemplate(msg.Template)

	case TemplateVarFillerDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingVarFillerTemplate = nil
			if m.currentProject != nil {
				return m.showViewInit(ViewTaskTemplatePicker, NewTemplatePickerModel(m.config, m.currentProject))
			}
			return true, nil
		}

		m.pendingVarFillerTemplate = nil
		return m.showTaskCreatorForTemplate(msg.Template)

	case WorkflowRulesUpdatedMsg:
		return true, nil

	case ShowWorkflowRuleCreatorMsg:
		return m.showViewInit(ViewWorkflowRuleCreator, NewWorkflowRuleCreatorModel(m.config, m.configManager))

	case ShowWorkflowRuleEditorMsg:
		return m.showViewInit(ViewWorkflowRuleEditor, NewWorkflowRuleEditorModel(m.config, m.configManager, msg.Rule))

	case WorkflowRuleCreatedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case ShowWorkflowWizardMsg:
		return m.showViewInit(ViewWorkflowWizard, NewWorkflowWizardModel(m.config, m.configManager))

	case WorkflowWizardSavedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case WorkflowRuleEditedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case ShowWorkflowDryRunMsg:
		return m.showViewInit(ViewWorkflowDryRun, NewWorkflowDryRunModel(m.config, m.configManager, m.persister, msg.SingleRule))

	case ShowFilterBuilderMsg:
		return m.showViewInit(ViewFilterBuilder, NewFilterBuilderModel(m.config, m.configManager))

	case ShowFilterEditorMsg:
		return m.showViewInit(ViewFilterBuilder, NewFilterEditorModel(m.config, m.configManager, msg.Filter))

	case FilterBuilderSavedMsg:
		m.popView()
		if m.allTasksView != nil {
			m.allTasksView.buildTabEntries()
			m.allTasksView.loadAllTasks()
		}
		return true, nil
	}

	return false, nil
}

func (m *AppModel) refreshTaskTemplatesView() {
	if m.currentView() != ViewTaskTemplates {
		return
	}

	templatesView := NewTaskTemplatesModel(m.config, m.configManager)
	templatesView.SetSize(m.width, m.height-2)
	m.views.Register(ViewTaskTemplates, templatesView)
}

func (m *AppModel) showTaskCreatorForTemplate(taskTemplate *template.TaskTemplate) (bool, tea.Cmd) {
	parentTask := m.pendingCreateTaskParent
	m.pendingCreateTaskParent = nil
	if m.currentProject != nil {
		return m.showViewInit(ViewTaskForm, NewTaskFormModelCreateFromTemplate(m.currentProject, m.config, m.configManager, parentTask, taskTemplate))
	}

	return true, nil
}

func (m *AppModel) refreshWorkflowRulesView() {
	if m.currentView() != ViewWorkflowRules {
		return
	}

	rulesView := NewWorkflowRulesModel(m.config, m.configManager)
	rulesView.SetSize(m.width, m.height-2)
	m.views.Register(ViewWorkflowRules, rulesView)
}
