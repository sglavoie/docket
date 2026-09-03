package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// handleEditorMessages handles text editor, glow, and external editor messages.
// Returns true if the message was handled.
func (m *AppModel) handleEditorMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTextEditorMsg:
		return m.showViewInit(presentation.ViewTextEditor, NewTextEditorModel(msg.FieldName, msg.CurrentValue))

	case ShowTaskTextEditorMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		var fieldName, currentValue string
		switch msg.Operation {
		case app.TaskOperationEditDescription:
			fieldName = "description"
			currentValue = msg.Task.Description
		default:
			return true, errorCmd("unsupported task text operation %q", msg.Operation)
		}
		m.pendingTaskTextEdit = &struct {
			Identity  taskref.TaskIdentity
			Operation app.TaskOperation
			Original  string
		}{
			Identity:  taskref.For(msg.Task, msg.Project),
			Operation: msg.Operation,
			Original:  currentValue,
		}
		return m.showViewInit(presentation.ViewTextEditor, NewTextEditorModel(fieldName, currentValue))

	case TextEditorDoneMsg:
		m.popView()
		if m.pendingTaskTextEdit != nil {
			pending := m.pendingTaskTextEdit
			m.pendingTaskTextEdit = nil
			if msg.Canceled || msg.Value == pending.Original {
				return true, nil
			}
			action := "Edit Description"
			summary := "Description updated"
			return true, m.secondaryFeatures.TaskItem.ExecuteOperation(app.TaskOperationRequest{
				Operation: pending.Operation,
				Targets:   []app.TaskLocator{pending.Identity.Locator()},
				Text:      msg.Value,
			}, action, summary)
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
