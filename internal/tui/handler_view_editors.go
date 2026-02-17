package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// handleEditorMessages handles text editor, recurrence editor, glow, and external editor messages.
// Returns true if the message was handled.
func (m *AppModel) handleEditorMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
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

	case ShowDescriptionEditorMsg:
		// Store context for when editing completes
		m.pendingDescEdit = &struct {
			Task    *models.Task
			Project *models.Project
		}{
			Task:    msg.Task,
			Project: msg.Project,
		}
		editor := NewTextEditorModel("description", msg.Task.Description)
		m.showView(ViewTextEditor, editor)
		return true, editor.Init()

	case TextEditorDoneMsg:
		m.popView()
		// Check if this is a description edit from td keybinding
		if m.pendingDescEdit != nil {
			pending := m.pendingDescEdit
			m.pendingDescEdit = nil
			return true, func() tea.Msg {
				return DescriptionEditorDoneMsg{
					Task:           pending.Task,
					Project:        pending.Project,
					NewDescription: msg.Value,
					Canceled:       msg.Canceled,
				}
			}
		}
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

	case GlowViewMsg:
		return m.handleGlowView(msg)

	case GlowViewDoneMsg:
		// Force re-layout of kanban view after returning from glow pager
		if m.kanban != nil {
			m.kanban.SetSize(m.width, m.height-2)
		}
		return true, nil

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

	editor := external.ResolveEditor(m.config.Editor)
	c := external.EditorCommand(editor, tmpPath)

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

	c := exec.Command("sh", "-c", fmt.Sprintf("CLICOLOR_FORCE=1 glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return GlowViewDoneMsg{}
	})
}
