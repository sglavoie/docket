package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskDecisionsModel) saveDecision() tea.Cmd {
	task, err := appendDecision(
		m.project,
		m.task.ID,
		m.textInput.Value(),
		strings.TrimSpace(m.rationaleInput.Value()),
		m.selectedCategory(),
	)
	if err != nil {
		m.err = err
		return nil
	}

	m.task = task
	m.cancelAddMode()
	m.err = nil
	m.cursor = len(m.task.Decisions) - 1

	return persistTaskItemMutation(m.project, DecisionAddedMsg{Task: task})
}

func (m *TaskDecisionsModel) executeDelete() tea.Cmd {
	return executeTaskItemDeleteMutation(taskItemDeleteMutation{
		project: m.project,
		task:    &m.task,
		cursor:  &m.cursor,
		remove: func(task *models.Task, index int) bool {
			return task.RemoveDecision(index)
		},
		count: func(task *models.Task) int {
			return len(task.Decisions)
		},
		errorMessage: "failed to remove decision",
		success: func(task *models.Task) tea.Msg {
			return DecisionDeletedMsg{Task: task}
		},
	})
}

func (m *TaskDecisionsModel) startEditDecision() tea.Cmd {
	decision := m.task.Decisions[m.cursor]
	m.editMode = true
	m.editIndex = m.cursor
	m.editActiveField = 0
	m.editTextInput.SetValue(decision.Text)
	m.editRationale.SetValue(decision.Rationale)
	m.editCategoryIdx = decisionCategoryIndex(decision.Category)
	m.editTextInput.Focus()
	m.editRationale.Blur()
	return textarea.Blink
}

func (m *TaskDecisionsModel) saveEditedDecision() tea.Cmd {
	text := strings.TrimSpace(m.editTextInput.Value())
	if text == "" {
		m.err = fmt.Errorf("decision text cannot be empty")
		return nil
	}

	refreshTaskReference(m.project, &m.task)
	if m.editIndex >= len(m.task.Decisions) {
		m.cancelEditMode()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("decision was deleted externally")} }
	}

	m.task.Decisions[m.editIndex].Text = text
	m.task.Decisions[m.editIndex].Rationale = strings.TrimSpace(m.editRationale.Value())
	m.task.Decisions[m.editIndex].Category = m.selectedEditCategory()
	m.cancelEditMode()
	m.err = nil

	return persistTaskItemMutation(m.project, DecisionEditedMsg{Task: m.task})
}

func (m *TaskDecisionsModel) editInExternalEditor() tea.Cmd {
	decision := m.task.Decisions[m.cursor]
	editIndex := m.cursor
	content := m.formatDecisionForEditor(editIndex, decision.Text, decision.Rationale, true)

	task := m.task
	project := m.project
	return m.editDecisionWithExternalEditor(content, true, func(newText, newRationale string) tea.Msg {
		if newText == decision.Text && newRationale == decision.Rationale {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}
		if editIndex >= len(task.Decisions) {
			return ErrorMsg{Err: fmt.Errorf("decision was deleted externally")}
		}

		task.Decisions[editIndex].Text = newText
		task.Decisions[editIndex].Rationale = newRationale
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return DecisionEditedMsg{Task: task}
	})
}

func (m *TaskDecisionsModel) addDecisionInExternalEditor() tea.Cmd {
	currentText := strings.TrimSpace(m.textInput.Value())
	currentRationale := strings.TrimSpace(m.rationaleInput.Value())
	content := m.formatNewDecisionForEditor(currentText, currentRationale)

	task := m.task
	project := m.project
	category := m.selectedCategory()
	m.cancelAddMode()

	return m.editDecisionWithExternalEditor(content, false, func(newText, newRationale string) tea.Msg {
		if newText == "" {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		task.AddDecision(newText, newRationale, category)
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return DecisionAddedMsg{Task: task}
	})
}

func (m *TaskDecisionsModel) toggleResolved() tea.Cmd {
	return executeTaskItemToggleMutation(taskItemToggleMutation{
		project: m.project,
		task:    &m.task,
		cursor:  m.cursor,
		toggle: func(task *models.Task, index int) bool {
			return task.ToggleDecisionResolved(index)
		},
		errorMessage: "failed to toggle decision resolved state",
		success: func(task *models.Task) tea.Msg {
			return DecisionResolvedMsg{Task: task}
		},
	})
}

func (m *TaskDecisionsModel) editDecisionInExternalEditorFromEditMode() tea.Cmd {
	if m.editIndex >= len(m.task.Decisions) {
		m.cancelEditMode()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("decision was deleted externally")} }
	}
	decision := m.task.Decisions[m.editIndex]
	editIndex := m.editIndex
	currentText := strings.TrimSpace(m.editTextInput.Value())
	currentRationale := strings.TrimSpace(m.editRationale.Value())
	content := m.formatDecisionForEditor(editIndex, currentText, currentRationale, true)

	task := m.task
	project := m.project
	originalText := decision.Text
	originalRationale := decision.Rationale
	m.cancelEditMode()

	return m.editDecisionWithExternalEditor(content, true, func(newText, newRationale string) tea.Msg {
		if newText == originalText && newRationale == originalRationale {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}
		if editIndex >= len(task.Decisions) {
			return ErrorMsg{Err: fmt.Errorf("decision was deleted externally")}
		}

		task.Decisions[editIndex].Text = newText
		task.Decisions[editIndex].Rationale = newRationale
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return DecisionEditedMsg{Task: task}
	})
}

func (m *TaskDecisionsModel) copyDecisionText() tea.Cmd {
	decision := m.task.Decisions[m.cursor]

	if err := copyTextToClipboard(decision.Text); err != nil {
		if errors.Is(err, errClipboardUnsupported) {
			m.setStatus(err.Error())
			return nil
		}
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return nil
	}

	m.setStatus("Decision copied to clipboard")
	return nil
}

func (m *TaskDecisionsModel) editDecisionWithExternalEditor(content string, emptyIsError bool, apply func(text, rationale string) tea.Msg) tea.Cmd {
	tmpPath, err := m.createDecisionTempFile(content)
	if err != nil {
		m.err = err
		return nil
	}

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}

	command := external.EditorCommand(editor, tmpPath)
	return tea.ExecProcess(command, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		newText, newRationale, readErr := readEditedDecisionContent(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: readErr}
		}
		if newText == "" && emptyIsError {
			return ErrorMsg{Err: fmt.Errorf("decision text cannot be empty")}
		}

		return apply(newText, newRationale)
	})
}

func (m *TaskDecisionsModel) createDecisionTempFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "tt-decision-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpPath, nil
}

func (m *TaskDecisionsModel) formatDecisionForEditor(index int, text, rationale string, isEdit bool) string {
	action := "Edit"
	if !isEdit {
		action = "New"
	}
	sep := "\n---\n"
	body := text + sep + rationale
	return fmt.Sprintf("# %s decision %d on #%d: %s\n\n<!-- Text above ---, rationale below. -->\n\n%s", action, index, m.task.ShortID, m.task.Title, body)
}

func (m *TaskDecisionsModel) formatNewDecisionForEditor(text, rationale string) string {
	sep := "\n---\n"
	body := text + sep + rationale
	return fmt.Sprintf("# New decision on #%d: %s\n\n<!-- Text above ---, rationale below. -->\n\n%s", m.task.ShortID, m.task.Title, body)
}

// readEditedDecisionContent reads the temp file, strips the header, and splits on "---".
func readEditedDecisionContent(tmpPath string) (text, rationale string, err error) {
	raw, readErr := os.ReadFile(tmpPath)
	_ = os.Remove(tmpPath)
	if readErr != nil {
		return "", "", fmt.Errorf("failed to read modified file: %w", readErr)
	}

	content := string(raw)
	// Strip header: skip lines until past the HTML comment marker
	lines := strings.Split(content, "\n")
	var bodyLines []string
	pastHeader := false
	for _, line := range lines {
		if pastHeader {
			bodyLines = append(bodyLines, line)
			continue
		}
		if strings.HasPrefix(line, "<!--") {
			pastHeader = true
			continue
		}
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))

	// Split on first "---" separator
	parts := strings.SplitN(body, "\n---\n", 2)
	text = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		rationale = strings.TrimSpace(parts[1])
	}

	return text, rationale, nil
}

// decisionCategoryIndex returns the index of the given category in ValidDecisionCategories.
func decisionCategoryIndex(cat models.DecisionCategory) int {
	for i, c := range models.ValidDecisionCategories() {
		if c == cat {
			return i
		}
	}
	return 0
}
