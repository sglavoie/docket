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
		m.showView(ViewSettings, NewSettingsModel(m.deps))
		return true, nil

	case SettingsSavedMsg:
		m.popView()
		settings := m.runtime.Settings()
		ApplyTheme(NewThemeConfig(settings))
		icons.SetUseEmoji(settings.GetUseEmoji())
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		if settings.GetAutoTheme() {
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
		m.showView(ViewWorkflowRules, NewWorkflowRulesModel(m.runtime))
		return true, nil

	case ShowTaskTemplatesMsg:
		m.showView(ViewTaskTemplates, NewTaskTemplatesModel(m.runtime))
		return true, nil

	case TaskTemplatesUpdatedMsg:
		return true, nil

	case ShowTaskTemplateCreatorMsg:
		return m.showViewInit(ViewTaskTemplateCreator, NewTaskTemplateCreatorModel(m.runtime))

	case ShowTaskTemplateEditorMsg:
		return m.showViewInit(ViewTaskTemplateEditor, NewTaskTemplateEditorModel(m.runtime, msg.Template))

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
			return m.showViewInit(ViewTaskTemplatePicker, NewTemplatePickerModel(m.runtime.Automation().GetEnabledTaskTemplates(), msg.Project))
		}
		return true, nil

	case TemplatePickerDoneMsg:
		m.popView()

		if msg.Canceled {
			m.pendingCreateTaskParent = nil
			m.pendingCreateTaskProject = nil
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
			project := m.pendingCreateTaskProject
			if project == nil {
				project = m.currentProject
			}
			if project != nil {
				return m.showViewInit(ViewTaskTemplatePicker, NewTemplatePickerModel(m.runtime.Automation().GetEnabledTaskTemplates(), project))
			}
			return true, nil
		}

		m.pendingVarFillerTemplate = nil
		return m.showTaskCreatorForTemplate(msg.Template)

	case WorkflowRulesUpdatedMsg:
		return true, nil

	case ShowWorkflowRuleCreatorMsg:
		return m.showViewInit(ViewWorkflowRuleCreator, NewWorkflowRuleCreatorModel(m.runtime))

	case ShowWorkflowRuleEditorMsg:
		return m.showViewInit(ViewWorkflowRuleEditor, NewWorkflowRuleEditorModel(m.runtime, msg.Rule))

	case WorkflowRuleCreatedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case ShowWorkflowWizardMsg:
		return m.showViewInit(ViewWorkflowWizard, NewWorkflowWizardModel(m.runtime))

	case WorkflowWizardSavedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case WorkflowRuleEditedMsg:
		m.popView()
		m.refreshWorkflowRulesView()
		return true, nil

	case ShowWorkflowDryRunMsg:
		return m.showViewInit(ViewWorkflowDryRun, NewWorkflowDryRunModel(m.runtime.DataDir(), m.configManager, m.persister, msg.SingleRule))

	case ShowFilterBuilderMsg:
		return m.showViewInit(ViewFilterBuilder, NewFilterBuilderModel(m.runtime))

	case ShowFilterEditorMsg:
		return m.showViewInit(ViewFilterBuilder, NewFilterEditorModel(m.runtime, msg.Filter))

	case FilterBuilderSavedMsg:
		m.popView()
		m.refreshAllTasksTabs()
		return true, nil
	}

	return false, nil
}

func (m *AppModel) refreshTaskTemplatesView() {
	if m.currentView() != ViewTaskTemplates {
		return
	}

	templatesView := NewTaskTemplatesModel(m.runtime)
	templatesView.SetSize(m.width, m.height-2)
	m.views.Register(ViewTaskTemplates, templatesView)
}

func (m *AppModel) showTaskCreatorForTemplate(taskTemplate *template.TaskTemplate) (bool, tea.Cmd) {
	parentTask := m.pendingCreateTaskParent
	project := m.pendingCreateTaskProject
	m.pendingCreateTaskParent = nil
	m.pendingCreateTaskProject = nil
	if project == nil {
		project = m.currentProject
	}
	if project != nil {
		return m.showViewInit(ViewTaskForm, NewTaskFormModelCreateFromTemplate(project, m.runtime, parentTask, taskTemplate))
	}

	return true, nil
}

func (m *AppModel) refreshWorkflowRulesView() {
	if m.currentView() != ViewWorkflowRules {
		return
	}

	rulesView := NewWorkflowRulesModel(m.runtime)
	rulesView.SetSize(m.width, m.height-2)
	m.views.Register(ViewWorkflowRules, rulesView)
}
