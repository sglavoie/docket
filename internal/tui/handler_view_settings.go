package tui

import (
	tea "charm.land/bubbletea/v2"

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
		sm.tabs.Active = 8
		sm.focusIndex = tabFieldStart[8]
		sm.focusCurrentField()
		m.showView(ViewSettings, sm)
		return true, nil

	case SettingsSavedMsg:
		m.popView()
		ApplyTheme(NewThemeConfig(m.config))
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
		m.refreshTaskTemplatesView()
		return true, nil

	case TaskTemplateEditedMsg:
		m.popView()
		m.refreshTaskTemplatesView()
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

		if msg.Template != nil && template.HasVariables(msg.Template) {
			m.pendingVarFillerTemplate = msg.Template
			filler := NewTemplateVarFillerModel(msg.Template)
			m.showView(ViewTemplateVarFiller, filler)
			return true, filler.Init()
		}

		return m.showTaskCreatorForTemplate(msg.Template)

	case TemplateVarFillerDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingVarFillerTemplate = nil
			if m.currentProject != nil {
				picker := NewTemplatePickerModel(m.config, m.currentProject)
				m.showView(ViewTaskTemplatePicker, picker)
				return true, picker.Init()
			}
			return true, nil
		}

		m.pendingVarFillerTemplate = nil
		return m.showTaskCreatorForTemplate(msg.Template)

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
		m.refreshWorkflowRulesView()
		return true, nil

	case ShowWorkflowWizardMsg:
		wizard := NewWorkflowWizardModel(m.config, m.configManager)
		m.showView(ViewWorkflowWizard, wizard)
		return true, wizard.Init()

	case WorkflowWizardSavedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case WorkflowRuleEditedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
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
		creator := NewTaskCreatorModelWithTemplate(m.currentProject, m.config, m.configManager, parentTask, taskTemplate)
		m.showView(ViewTaskCreator, creator)
		return true, creator.Init()
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
