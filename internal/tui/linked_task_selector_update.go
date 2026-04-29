package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *LinkedTaskSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeypress(msg)
	}

	return m, nil
}

func (m *LinkedTaskSelectorModel) handleKeypress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pendingPrefix != "" && isModifierOnly(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		prefix := m.pendingPrefix
		m.pendingPrefix = ""
		if key != "esc" && prefix == "g" {
			_, cmd := handleGotoTopPrefix(key, &m.cursor, &m.cursorFlash)
			return m, cmd
		}
		return m, nil
	}

	if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterRelations); handled {
		return model, cmd
	}

	switch key {
	case "/":
		return m, m.ActivateSearch()

	case "esc":
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.filterRelations()
			return m, nil
		}
		return m, func() tea.Msg { return CloseLinkedTaskSelectorMsg{} }

	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.filtered) {
			return m, nil
		}

		rel := m.filtered[m.cursor]
		if rel.IsMissing {
			return m, m.unlinkMissingTaskFromFiltered(m.cursor)
		}

		project := rel.Project
		if project == nil {
			project = m.project
		}

		return m, func() tea.Msg {
			return LinkedTaskSelectedMsg{Task: rel.Task, Project: project, TaskRef: rel.TaskRef}
		}

	case "x", "d":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			return m, m.unlinkTaskFromFiltered(m.cursor)
		}
		return m, nil

	case "g":
		m.pendingPrefix = "g"
		return m, nil
	}

	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	return m, nil
}

// filterRelations filters the relations based on the current search query.
func (m *LinkedTaskSelectorModel) filterRelations() {
	query := strings.ToLower(strings.TrimSpace(m.SearchQuery()))
	if query == "" {
		m.filtered = m.relations
		m.cursor = 0
		m.scrollOffset = 0
		return
	}

	m.filtered = nil
	for _, rel := range m.relations {
		searchText := linkedTaskSearchText(rel)
		if strings.Contains(searchText, query) {
			m.filtered = append(m.filtered, rel)
		}
	}

	m.cursor = 0
	m.scrollOffset = 0
}

func linkedTaskSearchText(rel LinkedTaskRelation) string {
	if rel.IsMissing {
		ref := models.ParseTaskRef(rel.TaskRef)
		if ref.IsCrossProject() {
			if rel.Project != nil {
				return strings.ToLower(rel.Project.Name + " missing")
			}
			return strings.ToLower(ref.ProjectID + " missing")
		}
		return "missing"
	}

	searchText := strings.ToLower(rel.Task.Title + " " + fmt.Sprintf("#%d", rel.Task.ShortID))
	if rel.Project != nil {
		searchText += " " + strings.ToLower(rel.Project.Name)
	}
	return searchText
}

// unlinkTask removes the link at the given index.
func (m *LinkedTaskSelectorModel) unlinkTask(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.relations) {
		return nil
	}

	rel := m.relations[idx]
	switch rel.RelationType {
	case "blocked by":
		m.sourceTask.RemoveDependency(rel.TaskRef)
	case "related":
		m.sourceTask.RemoveRelatedTo(rel.TaskRef)
	case "blocks":
		if rel.Task != nil {
			rel.Task.RemoveDependency(m.sourceTask.ID)
		}
	}

	project := m.project
	return func() tea.Msg {
		if err := storage.BackupProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup: %w", err)}
		}
		if err := storage.SaveProjectLocked(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
		if err := storage.LoadProject(project); err != nil {
			logging.Warn("failed to reload project %s: %v", project.Name, err)
		}
		return CloseLinkedTaskSelectorMsg{}
	}
}

// unlinkTaskFromFiltered removes the link at the given index in the filtered list.
func (m *LinkedTaskSelectorModel) unlinkTaskFromFiltered(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.filtered) {
		return nil
	}

	filteredRel := m.filtered[idx]
	for originalIndex, rel := range m.relations {
		if rel.TaskRef == filteredRel.TaskRef && rel.RelationType == filteredRel.RelationType {
			return m.unlinkTask(originalIndex)
		}
	}

	return nil
}

// unlinkMissingTaskFromFiltered handles unlinking a broken/missing link from the filtered list.
func (m *LinkedTaskSelectorModel) unlinkMissingTaskFromFiltered(idx int) tea.Cmd {
	return m.unlinkTaskFromFiltered(idx)
}
