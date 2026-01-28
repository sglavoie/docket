package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleViewMessages handles view navigation and display messages.
// Returns true if the message was handled.
func (m *AppModel) handleViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate search messages to dedicated handler
	if handled, cmd := m.handleSearchMessages(msg); handled {
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

	case SettingsMsg:
		m.showView(ViewSettings, NewSettingsModel(m.config, m.configManager))
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

	case WorkflowRulesUpdatedMsg:
		// Rules updated, just refresh the view (it's already updated internally)
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
		if msg.Task != nil && m.projectTree != nil {
			m.projectTree.NavigateToTask(msg.Task)
		}
		return true, nil

	case CloseTaskJumperMsg:
		m.popView()
		return true, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectPreview, NewProjectPreviewModel(msg.Project))
		}
		return true, nil

	case CloseProjectPreviewMsg:
		m.popView()
		return true, nil

	case ShowDueDateOverviewMsg:
		m.showView(ViewDueDateOverview, NewDueDateOverviewModel(m.config))
		return true, nil

	case CloseDueDateOverviewMsg:
		m.popView()
		return true, nil

	case ShowDailyReviewMsg:
		m.showView(ViewDailyReview, NewDailyReviewModel(m.config))
		return true, nil

	case CloseDailyReviewMsg:
		m.popView()
		return true, nil

	case DailyReviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.projectTree.NavigateToTask(msg.Task)
			m.pushView(ViewProjectTree)
		}
		return true, nil

	case ShowFocusViewMsg:
		m.showView(ViewFocusView, NewFocusViewModel(m.config))
		return true, nil

	case CloseFocusViewMsg:
		m.popView()
		return true, nil

	case FocusViewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.projectTree.NavigateToTask(msg.Task)
			m.pushView(ViewProjectTree)
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

	case GlowViewMsg:
		return m.handleGlowView(msg)

	case GlowViewDoneMsg:
		// Force re-layout of kanban view after returning from glow pager
		if m.kanban != nil {
			m.kanban.SetSize(m.width, m.height-2)
		}
		return true, nil

	case ShowScratchpadMsg:
		return m.handleShowScratchpad(msg)

	case EditScratchpadMsg:
		return m.handleEditScratchpad(msg)

	case ScratchpadEditedMsg:
		if msg.Err != nil {
			return true, errorCmd("scratchpad edit failed: %w", msg.Err)
		}
		m.refreshProjectTree()
		return true, nil

	case ShowContextPickerMsg:
		if msg.Dashboard != nil {
			m.showView(ViewContextPicker, NewContextPickerModel(msg.Dashboard, m.config))
		}
		return true, nil

	case ShowContextPickerForCycleMsg:
		m.showView(ViewContextPicker, NewContextPickerModelFromConfig(m.config))
		return true, nil

	case ContextPickerDoneMsg:
		m.popView()
		if !msg.Canceled {
			// Persist context filter to config - custom and regular contexts are mutually exclusive
			if msg.IsCustomContext {
				m.config.FilterCustomContext = msg.SelectedContext
				m.config.FilterContext = ""
			} else {
				m.config.FilterContext = msg.SelectedContext
				m.config.FilterCustomContext = ""
			}
			_ = m.configManager.Save(m.config)

			if msg.FromCycle {
				// Refresh the Project Cycle Manager with new context
				if cycleManager, ok := m.views.Get(ViewProjectCycleManager).(*ProjectCycleManagerModel); ok {
					cycleManager.RefreshWithContext()
				}
			} else if m.dashboard != nil {
				// Update dashboard filter
				if msg.IsCustomContext {
					m.dashboard.filterContext = ""
					m.dashboard.filterCustomContext = msg.SelectedContext
				} else {
					m.dashboard.filterContext = msg.SelectedContext
					m.dashboard.filterCustomContext = ""
				}
				visible := m.dashboard.visibleProjects()
				if m.dashboard.cursor >= len(visible) {
					if len(visible) > 0 {
						m.dashboard.cursor = len(visible) - 1
					} else {
						m.dashboard.cursor = 0
					}
				}
			}
		}
		return true, nil

	case ShowCustomContextEditorMsg:
		m.showView(ViewCustomContextEditor, NewCustomContextEditorModel(m.config, msg.EditName))
		return true, nil

	case CustomContextDeletedMsg:
		// Save config after deletion
		_ = m.configManager.Save(m.config)
		// If the deleted context was active, clear the dashboard filter
		if msg.WasActive && m.dashboard != nil {
			m.dashboard.filterCustomContext = ""
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = "Deleted custom context: " + msg.Name
		return true, statusTimeout(2 * time.Second)

	case CustomContextSavedMsg:
		// Save config and go back to context picker
		_ = m.configManager.Save(m.config)
		m.popView() // Pop editor view
		// Refresh context picker if it's the current view
		if m.currentView() == ViewContextPicker {
			if m.dashboard != nil {
				picker := NewContextPickerModel(m.dashboard, m.config)
				picker.SetSize(m.width, m.height-2)
				m.views.Register(ViewContextPicker, picker)
			}
		}
		// Show status message
		if msg.IsNew {
			m.statusMessage = fmt.Sprintf("Created custom context '%s' with %d projects", msg.Name, msg.ProjectCount)
		} else {
			m.statusMessage = fmt.Sprintf("Updated custom context '%s' with %d projects", msg.Name, msg.ProjectCount)
		}
		return true, statusTimeout(2 * time.Second)

	case ContextRenamedMsg:
		// Save config after renaming a context
		_ = m.configManager.Save(m.config)
		// Refresh dashboard if needed
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = fmt.Sprintf("Renamed '%s' to '%s' (%d projects)", msg.OldName, msg.NewName, msg.ProjectCount)
		return true, statusTimeout(2 * time.Second)

	case StatusClearMsg:
		m.statusMessage = ""
		return true, nil

	case MoveBlockedMsg:
		m.statusMessage = msg.Reason
		return true, statusTimeout(2 * time.Second)

	case ShowPriorityViewMsg:
		if msg.Project != nil {
			m.showView(ViewPriorityView, NewPriorityViewModel(msg.Project))
		}
		return true, nil

	case ClosePriorityViewMsg:
		m.popView()
		return true, nil

	case PriorityViewSelectMsg:
		m.popView()
		if msg.Task != nil {
			m.projectTree.NavigateToTask(msg.Task)
			m.pushView(ViewProjectTree)
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
			// Store the tasks for when priority is selected
			m.pendingBulkTasks = msg.Tasks
			// Show the priority picker (uses first task as reference)
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil

	case PaletteBulkTagMsg:
		// Add tags to multiple tasks from palette
		// For simplicity, collect all tasks from the first project and use existing bulk tag selector
		if len(msg.Tasks) > 0 {
			// Group by project and use first project's tasks
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
			// Store tasks and show date input for first task (will apply to all)
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewDateInput, NewDateInputModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil

	case ShowRecurrenceEditorMsg:
		editor := NewRecurrenceEditorModel(msg.CurrentPattern, msg.DueDate)
		m.showView(ViewRecurrenceEditor, editor)
		return true, nil

	case RecurrenceEditorDoneMsg:
		m.popView()
		// Forward the message to the parent view (task editor or task creator)
		// so it can update the recurrence pattern
		cmd := m.views.Update(m.currentView(), msg)
		return true, cmd

	case ShowTextEditorMsg:
		editor := NewTextEditorModel(msg.FieldName, msg.CurrentValue)
		m.showView(ViewTextEditor, editor)
		return true, editor.Init()

	case TextEditorDoneMsg:
		m.popView()
		// Forward the message to the parent view (task creator or task editor)
		// so it can update the title/description
		cmd := m.views.Update(m.currentView(), msg)
		return true, cmd

	case TextEditorExternalMsg:
		return m.handleTextEditorExternal(msg)

	case TextEditorExternalDoneMsg:
		// Forward to text editor view to update its textarea
		cmd := m.views.Update(m.currentView(), msg)
		return true, cmd

	case OpenDataFolderMsg:
		var path string
		if msg.Project != nil {
			path = m.config.GetProjectPath(msg.Project.Slug)
		} else {
			path = m.config.DataDir
		}

		editor := m.config.Editor
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "code"
		}

		c := exec.Command(editor, path)
		return true, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to open folder: %w", err)}
			}
			return nil
		})

	case ShowBlockersMsg:
		if msg.Project != nil {
			editor := m.config.Editor
			m.showView(ViewBlockers, NewBlockersModelWithEditor(msg.Project, editor))
		}
		return true, nil

	case CloseBlockersMsg:
		m.popView()
		return true, nil

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		// Blockers were modified, refresh dashboard to update indicators
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowTaskAttachmentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(msg.Task, msg.Project))
		}
		return true, nil

	case CloseTaskAttachmentsMsg:
		m.popView()
		return true, nil

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		// Refresh dashboard to update any indicators
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowKanbanMsg:
		if msg.Project != nil {
			m.kanban = NewKanbanViewModel(msg.Project, m.config, m.configManager)
			m.showView(ViewKanban, m.kanban)
		}
		return true, nil

	case CloseKanbanMsg:
		m.popView()
		m.kanban = nil
		m.reloadAndRefreshProjectTree()
		return true, nil
	}
	return false, nil
}

// handleTextEditorExternal opens an external editor for text field editing.
func (m *AppModel) handleTextEditorExternal(msg TextEditorExternalMsg) (bool, tea.Cmd) {
	tmpFile, err := os.CreateTemp("", "tt-text-*.md")
	if err != nil {
		return true, errorCmd("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(msg.CurrentValue)
	_ = tmpFile.Close()

	editor := m.config.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	fieldName := msg.FieldName
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return TextEditorExternalDoneMsg{FieldName: fieldName, Value: "", Canceled: true}
		}
		content, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return TextEditorExternalDoneMsg{FieldName: fieldName, Value: "", Canceled: true}
		}
		return TextEditorExternalDoneMsg{FieldName: fieldName, Value: string(content), Canceled: false}
	})
}

// handleGlowView opens task in glow pager.
func (m *AppModel) handleGlowView(msg GlowViewMsg) (bool, tea.Cmd) {
	if msg.Task == nil || !external.GlowAvailable() {
		return true, nil
	}

	content := external.FormatTaskAsMarkdown(msg.Task)
	tmpFile, err := os.CreateTemp("", "tt-glow-*.md")
	if err != nil {
		return true, errorCmd("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return GlowViewDoneMsg{}
	})
}

// handleShowScratchpad opens the project scratchpad in glow pager.
func (m *AppModel) handleShowScratchpad(msg ShowScratchpadMsg) (bool, tea.Cmd) {
	if msg.Project == nil {
		return true, nil
	}

	content, err := storage.LoadScratchpad(msg.Project)
	if err != nil {
		return true, errorCmd("failed to load scratchpad: %w", err)
	}

	if content == "" || strings.TrimSpace(content) == strings.TrimSpace(storage.DefaultScratchpadTemplate) {
		m.statusMessage = "Scratchpad is empty. Press ] to edit."
		return true, statusTimeout(3 * time.Second)
	}

	if !external.GlowAvailable() {
		// Fallback: just show a message
		m.statusMessage = "Scratchpad exists but glow not available"
		return true, statusTimeout(3 * time.Second)
	}

	// Add header for nicer display
	displayContent := fmt.Sprintf("# 🗒️ Scratchpad: %s\n\n%s", msg.Project.Name, content)

	tmpFile, err := os.CreateTemp("", "tt-scratchpad-*.md")
	if err != nil {
		return true, errorCmd("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(displayContent)
	_ = tmpFile.Close()

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return GlowViewDoneMsg{}
	})
}

// handleEditScratchpad opens the project scratchpad in an external editor.
func (m *AppModel) handleEditScratchpad(msg EditScratchpadMsg) (bool, tea.Cmd) {
	if msg.Project == nil {
		return true, nil
	}

	scratchpadPath, err := storage.EnsureScratchpadFile(msg.Project)
	if err != nil {
		return true, errorCmd("failed to create scratchpad: %w", err)
	}

	editor := m.config.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", scratchpadPath)
	} else {
		c = exec.Command(editor, scratchpadPath)
	}

	project := msg.Project
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return ScratchpadEditedMsg{Project: project, Err: err}
		}
		return ScratchpadEditedMsg{Project: project, Err: nil}
	})
}
