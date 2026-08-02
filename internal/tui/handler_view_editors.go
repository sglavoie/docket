package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// handleEditorMessages handles text editor, glow, and external editor messages.
// Returns true if the message was handled.
func (m *AppModel) handleEditorMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTextEditorMsg:
		return m.showViewInit(ViewTextEditor, NewTextEditorModel(msg.FieldName, msg.CurrentValue))

	case ShowTaskTextEditorMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		fieldName := ""
		currentValue := ""
		switch msg.Operation {
		case app.TaskOperationEditTitle:
			fieldName = "title"
			currentValue = msg.Task.Title
		case app.TaskOperationEditDescription:
			fieldName = "description"
			currentValue = msg.Task.Description
		default:
			return true, errorCmd("unsupported task text operation %q", msg.Operation)
		}
		m.pendingTaskTextEdit = &struct {
			Identity  TaskIdentity
			Operation app.TaskOperation
			Original  string
		}{
			Identity:  taskIdentityFor(msg.Task, msg.Project),
			Operation: msg.Operation,
			Original:  currentValue,
		}
		return m.showViewInit(ViewTextEditor, NewTextEditorModel(fieldName, currentValue))

	case ShowDescriptionEditorMsg:
		// Store context for when editing completes
		m.pendingDescEdit = &struct {
			Task    *models.Task
			Project *models.Project
		}{
			Task:    msg.Task,
			Project: msg.Project,
		}
		return m.showViewInit(ViewTextEditor, NewTextEditorModel("description", msg.Task.Description))

	case TextEditorDoneMsg:
		m.popView()
		if m.pendingTaskTextEdit != nil {
			pending := m.pendingTaskTextEdit
			m.pendingTaskTextEdit = nil
			if msg.Canceled || msg.Value == pending.Original {
				return true, nil
			}
			action := "Edit Title"
			summary := "Title updated"
			if pending.Operation == app.TaskOperationEditDescription {
				action = "Edit Description"
				summary = "Description updated"
			}
			return true, m.secondaryFeatures.TaskItem.ExecuteOperation(app.TaskOperationRequest{
				Operation: pending.Operation,
				Targets:   []app.TaskLocator{pending.Identity.Locator()},
				Text:      msg.Value,
			}, action, summary)
		}
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
		m.clearStatus()
		// Forward to text editor view to update its textarea
		cmd := m.views.Update(m.currentView(), msg)
		return true, cmd

	case GlowViewMsg:
		return m.handleGlowView(msg)

	case GlowViewDoneMsg:
		m.clearStatus()
		return true, nil

	case OpenLogDirMsg:
		c := exec.Command("open", logging.LogDir())
		return true, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to open log directory: %w", err)}
			}
			return nil
		})

	case OpenDataFolderMsg:
		var path string
		if msg.Project != nil {
			path = m.runtime.ProjectPath(msg.Project.Slug)
		} else {
			path = m.runtime.DataDir()
		}

		editor, editorErr := external.ResolveEditor("")
		if editorErr != nil {
			return true, errorCmd("%s", editorErr)
		}
		c := external.OpenFolderCommand(editor, path)
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
	tmpPath, err := external.CreateTempFileWithContent(msg.CurrentValue, "tt-text-", ".md")
	if err != nil {
		return true, errorCmd("%s", err)
	}

	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return true, errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	m.setInfoStatus("Opening in editor...")
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
	tmpPath, err := external.CreateTempFileWithContent(content, "tt-glow-", ".md")
	if err != nil {
		return true, errorCmd("%s", err)
	}

	m.setInfoStatus("Opening in glow...")
	c := exec.Command("sh", "-c", fmt.Sprintf("CLICOLOR_FORCE=1 glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return GlowViewDoneMsg{}
	})
}
