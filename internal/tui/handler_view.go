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
			m.showView(ViewContextPicker, NewContextPickerModel(msg.Dashboard))
		}
		return true, nil

	case ContextPickerDoneMsg:
		m.popView()
		if !msg.Cancelled && m.dashboard != nil {
			m.dashboard.filterContext = msg.SelectedContext
			visible := m.dashboard.visibleProjects()
			if m.dashboard.cursor >= len(visible) {
				if len(visible) > 0 {
					m.dashboard.cursor = len(visible) - 1
				} else {
					m.dashboard.cursor = 0
				}
			}
		}
		return true, nil

	case StatusClearMsg:
		m.statusMessage = ""
		return true, nil

	case MoveBlockedMsg:
		m.statusMessage = msg.Reason
		return true, statusTimeout(2 * time.Second)

	case ShowGlobalSearchMsg:
		m.globalSearch = NewGlobalSearchModel(m.config)
		m.showView(ViewGlobalSearch, m.globalSearch)
		return true, m.globalSearch.Init()

	case CloseGlobalSearchMsg:
		m.popView()
		return true, nil

	case ShowCommandPaletteMsg:
		// Pass the current view (before pushing the palette) for context-aware command filtering
		// Pass the current project to filter tasks (nil = show all projects' tasks)
		currentView := m.currentView()
		palette := NewCommandPaletteModel(m.config, m.configManager, m.commandRegistry, currentView, m.currentProject)
		m.showView(ViewCommandPalette, palette)
		return true, palette.Init()

	case CloseCommandPaletteMsg:
		m.popView()
		return true, nil

	case CommandPaletteExecuteMsg:
		// Check if the inner message opens a modal view that should return to the palette
		keepPalette := false
		if msg.ResultMsg != nil {
			switch msg.ResultMsg.(type) {
			case EditTaskMsg, SetDueDateMsg, ShowSectionPickerMsg, ShowPriorityPickerMsg, GlowViewMsg:
				keepPalette = true
			}
		}
		// Only close command palette for non-modal actions
		if !keepPalette {
			m.popView()
		}
		// Process the inner message through handlers
		if msg.ResultMsg != nil {
			return m.processMessage(msg.ResultMsg)
		}
		return true, nil

	case GlobalSearchSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.projectTree.NavigateToTask(msg.Task)
			m.showView(ViewProjectTree, m.projectTree)
		}
		return true, nil

	case ShowPriorityViewMsg:
		if msg.Project != nil {
			m.showView(ViewPriorityView, NewPriorityViewModel(msg.Project))
		}
		return true, nil

	case ClosePriorityViewMsg:
		m.popView()
		return true, nil

	case ShowQuickCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewQuickCapture, NewQuickCaptureModel(msg.Project))
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

	case ShowSaveSearchDialogMsg:
		// Show a simple text input dialog for the search name
		m.showView(ViewSaveSearchInput, NewSaveSearchInputModel(msg.Query))
		return true, nil

	case SaveSearchMsg:
		// Save the search to config
		m.config.SaveSearch(msg.Name, msg.Query)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to save search: %w", err)
		}
		// Pop the input dialog
		m.popView()
		// Refresh the command palette if it's still showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil

	case DeleteSavedSearchMsg:
		// Delete the saved search from config
		m.config.DeleteSavedSearch(msg.Name)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to delete saved search: %w", err)
		}
		// Refresh the command palette if it's showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil

	case ExecuteSavedSearchMsg:
		// Execute a saved search by setting the query in the palette
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.searchInput.SetValue(msg.Query)
				palette.filterItems()
			}
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
	}
	return false, nil
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
	tmpFile.WriteString(content)
	tmpFile.Close()

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return nil
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
	displayContent := fmt.Sprintf("# Scratchpad: %s\n\n%s", msg.Project.Name, content)

	tmpFile, err := os.CreateTemp("", "tt-scratchpad-*.md")
	if err != nil {
		return true, errorCmd("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.WriteString(displayContent)
	tmpFile.Close()

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return nil
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
