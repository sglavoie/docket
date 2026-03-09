package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/template"
)

func (m *TaskTemplateCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		section, idx := m.getFocusSection()

		// Handle suggestions dropdown navigation
		if m.showSuggestions {
			suggestions := m.getCurrentSuggestions()
			switch msg.String() {
			case "esc":
				m.hideSuggestions()
				return m, nil
			case "up", "ctrl+k", "ctrl+p":
				if m.suggestionCursor > 0 {
					m.suggestionCursor--
				}
				return m, nil
			case "down", "ctrl+j", "ctrl+n":
				if m.suggestionCursor < len(suggestions)-1 {
					m.suggestionCursor++
				}
				return m, nil
			case "tab", "enter":
				switch section {
				case "section":
					m.acceptSectionSuggestion()
				case "tags":
					m.acceptTagSuggestion()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "esc":
			// Close the creator
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "[", "]":
			lastTab := templateTabChecklist
			if m.currentTypeCanHaveChildren() {
				lastTab = templateTabSubtasks
			}
			s := formTabState{&m.activeTab, &m.focusIndex, &m.scrollOffset}
			return m, handleFormTabSwitch(m, s, templateTabFields, lastTab, msg.String() == "]")

		case "tab", "shift+tab":
			m.hideSuggestions()
			s := formTabState{&m.activeTab, &m.focusIndex, &m.scrollOffset}
			return m, handleFormTabKey(m, s, msg.String() == "tab")

		case "ctrl+s":
			// Save template
			return m, m.saveTemplate()

		case "ctrl+a":
			// Switch to checklist tab and add new item
			m.err = nil
			m.blurCurrentField()
			m.activeTab = templateTabChecklist
			factory := DefaultFormFieldFactory()
			input := factory.NewTextInput("Checklist item")
			m.checklist = append(m.checklist, input)
			// Move focus to the new checklist item
			m.focusIndex = m.checklistStartIndex() + len(m.checklist) - 1
			m.scrollOffset = 0
			cmdFocus := m.focusCurrentField()
			return m, cmdFocus

		case "ctrl+t":
			// Switch to subtasks tab and add new item (only for Epic/Story)
			if !m.currentTypeCanHaveChildren() {
				m.err = fmt.Errorf("%s cannot have subtasks", m.types[m.typeCursor])
				return m, nil
			}
			m.err = nil
			m.blurCurrentField()
			m.activeTab = templateTabSubtasks
			factory := DefaultFormFieldFactory()
			input := factory.NewTextInput("Subtask title")
			m.subtasks = append(m.subtasks, SubtaskFormState{titleInput: input, typeCursor: 0, optional: false})
			// Move focus to the new subtask
			m.focusIndex = m.subtaskStartIndex() + len(m.subtasks) - 1
			m.scrollOffset = 0
			cmdFocus := m.focusCurrentField()
			return m, cmdFocus

		case "ctrl+d":
			// Delete current item (checklist or subtask)
			if section == "checklist" && len(m.checklist) > 0 {
				m.checklist = append(m.checklist[:idx], m.checklist[idx+1:]...)
				if m.focusIndex >= m.focusFieldCount() {
					m.focusIndex = m.focusFieldCount() - 1
				}
				if m.focusIndex < 0 {
					m.focusIndex = 0
				}
				cmdFocus := m.focusCurrentField()
				return m, cmdFocus
			}
			if section == "subtask" && len(m.subtasks) > 0 {
				m.subtasks = append(m.subtasks[:idx], m.subtasks[idx+1:]...)
				if m.focusIndex >= m.focusFieldCount() {
					m.focusIndex = m.focusFieldCount() - 1
				}
				if m.focusIndex < 0 {
					m.focusIndex = 0
				}
				cmdFocus := m.focusCurrentField()
				return m, cmdFocus
			}

		case "left", "h":
			// Navigate selector fields
			switch section {
			case "type":
				if m.typeCursor > 0 {
					m.typeCursor--
				}
				return m, nil
			case "category":
				if m.categoryCursor > 0 {
					m.categoryCursor--
				}
				return m, nil
			case "priority":
				if m.priorityCursor > 0 {
					m.priorityCursor--
				}
				return m, nil
			case "subtask":
				// Cycle subtask type left
				if idx >= 0 && idx < len(m.subtasks) {
					if m.subtasks[idx].typeCursor > 0 {
						m.subtasks[idx].typeCursor--
					}
				}
				return m, nil
			}

		case "right", "l":
			// Navigate selector fields
			switch section {
			case "type":
				if m.typeCursor < len(m.types)-1 {
					m.typeCursor++
				}
				return m, nil
			case "category":
				if m.categoryCursor < len(m.categories)-1 {
					m.categoryCursor++
				}
				return m, nil
			case "priority":
				if m.priorityCursor < 5 {
					m.priorityCursor++
				}
				return m, nil
			case "subtask":
				// Cycle subtask type right
				if idx >= 0 && idx < len(m.subtasks) {
					if m.subtasks[idx].typeCursor < len(subtaskTypes)-1 {
						m.subtasks[idx].typeCursor++
					}
				}
				return m, nil
			}

		case "ctrl+o":
			// Toggle optional for subtask
			if section == "subtask" && idx >= 0 && idx < len(m.subtasks) {
				m.subtasks[idx].optional = !m.subtasks[idx].optional
				return m, nil
			}

		case "ctrl+k":
			// Move item up
			if section == "checklist" && idx > 0 && idx < len(m.checklist) {
				m.checklist[idx], m.checklist[idx-1] = m.checklist[idx-1], m.checklist[idx]
				m.focusIndex--
				return m, nil
			}
			if section == "subtask" && idx > 0 && idx < len(m.subtasks) {
				m.subtasks[idx], m.subtasks[idx-1] = m.subtasks[idx-1], m.subtasks[idx]
				m.focusIndex--
				return m, nil
			}

		case "ctrl+j":
			// Move item down
			if section == "checklist" && idx >= 0 && idx < len(m.checklist)-1 {
				m.checklist[idx], m.checklist[idx+1] = m.checklist[idx+1], m.checklist[idx]
				m.focusIndex++
				return m, nil
			}
			if section == "subtask" && idx >= 0 && idx < len(m.subtasks)-1 {
				m.subtasks[idx], m.subtasks[idx+1] = m.subtasks[idx+1], m.subtasks[idx]
				m.focusIndex++
				return m, nil
			}

		case "1", "2", "3", "4", "5":
			// Quick priority selection
			if section == "priority" {
				m.priorityCursor = int(msg.String()[0] - '0')
				return m, nil
			}

		case "down":
			// Show all suggestions when dropdown not visible
			switch section {
			case "section":
				m.showAllSectionSuggestions()
				return m, nil
			case "tags":
				m.showAllTagSuggestions()
				return m, nil
			}
		}

		// Update focused text input
		switch section {
		case "name":
			m.nameInput, cmd = m.nameInput.Update(msg)
		case "section":
			prevValue := m.sectionInput.Value()
			m.sectionInput, cmd = m.sectionInput.Update(msg)
			if m.sectionInput.Value() != prevValue {
				m.updateSectionSuggestions()
			}
		case "tags":
			prevValue := m.tagsInput.Value()
			m.tagsInput, cmd = m.tagsInput.Update(msg)
			if m.tagsInput.Value() != prevValue {
				m.updateTagSuggestions()
			}
		case "description":
			m.descInput, cmd = m.descInput.Update(msg)
		case "checklist":
			if idx >= 0 && idx < len(m.checklist) {
				m.checklist[idx], cmd = m.checklist[idx].Update(msg)
			}
		case "subtask":
			if idx >= 0 && idx < len(m.subtasks) {
				m.subtasks[idx].titleInput, cmd = m.subtasks[idx].titleInput.Update(msg)
			}
		}
	}

	return m, cmd
}

func (m *TaskTemplateCreatorModel) blurCurrentField() {
	section, idx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Blur()
	case "section":
		m.sectionInput.Blur()
	case "tags":
		m.tagsInput.Blur()
	case "description":
		m.descInput.Blur()
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx].Blur()
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput.Blur()
		}
	}
}

func (m *TaskTemplateCreatorModel) focusCurrentField() tea.Cmd {
	section, idx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Focus()
	case "section":
		m.sectionInput.Focus()
	case "tags":
		m.tagsInput.Focus()
	case "description":
		return m.descInput.Focus()
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx].Focus()
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput.Focus()
		}
	}
	return nil
}

func (m *TaskTemplateCreatorModel) saveTemplate() tea.Cmd {
	return func() tea.Msg {
		// Validate name
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("template name is required")}
		}

		// Build template
		var t template.TaskTemplate
		if m.editingTemplate != nil {
			t = *m.editingTemplate
		} else {
			t.ID = uuid.New().String()[:8]
			t.Enabled = true
		}

		t.Name = name
		t.Type = m.types[m.typeCursor]

		if m.categoryCursor > 0 {
			t.Category = m.categories[m.categoryCursor]
		} else {
			t.Category = ""
		}

		t.Priority = m.priorityCursor
		t.Section = strings.TrimSpace(m.sectionInput.Value())
		t.Description = strings.TrimSpace(m.descInput.Value())

		// Parse tags
		tagsStr := strings.TrimSpace(m.tagsInput.Value())
		if tagsStr != "" {
			var tags []string
			for tag := range strings.SplitSeq(tagsStr, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
			t.Tags = tags
		} else {
			t.Tags = nil
		}

		// Build checklist (available for all types)
		var checklist []string
		for _, item := range m.checklist {
			text := strings.TrimSpace(item.Value())
			if text != "" {
				checklist = append(checklist, text)
			}
		}
		t.Checklist = checklist

		// Build children/subtasks (only for Epic/Story)
		var children []template.TemplateChild
		if t.Type.CanHaveChildren() {
			for _, st := range m.subtasks {
				title := strings.TrimSpace(st.titleInput.Value())
				if title != "" {
					children = append(children, template.TemplateChild{
						Title:    title,
						Type:     subtaskTypes[st.typeCursor],
						Optional: st.optional,
					})
				}
			}
		}
		t.Children = children

		// Validate variable tokens
		if err := template.ValidateTokens(&t); err != nil {
			return ErrorMsg{Err: err}
		}

		// Validate
		if err := t.Validate(); err != nil {
			return ErrorMsg{Err: err}
		}

		// Save
		if m.editingTemplate != nil {
			if !m.config.UpdateTaskTemplate(t) {
				return ErrorMsg{Err: fmt.Errorf("failed to update template")}
			}
		} else {
			m.config.AddTaskTemplate(t)
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		if m.editingTemplate != nil {
			return TaskTemplateEditedMsg{Template: t}
		}
		return TaskTemplateCreatedMsg{Template: t}
	}
}
