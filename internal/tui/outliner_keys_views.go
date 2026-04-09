package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *OutlinerModel) handleTaskViewKey(key string, onHeader bool) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "o":
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			project := m.project
			if project == nil {
				row, proj, valid := m.crossGetMutationTarget()
				if !valid {
					return m, nil, true
				}
				task = row.task
				project = proj
			}
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}, true
		}
		return m, nil, true

	case "e":
		if m.project == nil {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				m.setStatus("bulk edit not supported in cross-project mode")
				return m, nil, true
			}
			row, proj, valid := m.crossGetMutationTarget()
			if !valid {
				return m, nil, true
			}
			task := row.task
			return m, func() tea.Msg { return EditTaskMsg{Task: task, Project: proj} }, true
		}
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return m, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   m.getSelectedTasks(),
					Project: m.project,
				}
			}, true
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			return m, func() tea.Msg {
				return EditTaskMsg{Task: task}
			}, true
		}
		return m, nil, true

	case "E":
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			task := m.rows[m.cursor].task
			if m.project == nil {
				row, proj, valid := m.crossGetMutationTarget()
				if !valid {
					return m, nil, true
				}
				task = row.task
				m.crossQuickEditProject = proj
			}
			return m, m.quickEdit.Enter(task, false), true
		}
		return m, nil, true

	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks?", len(m.selectedTasks)))
			return m, nil, true
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) && !onHeader {
			m.showDeleteConfirm(m.rows[m.cursor].task)
		}
		return m, nil, true

	case "ctrl+f":
		return m, func() tea.Msg {
			return ShowFilterMsg{CommonFilter: &m.filter.CommonFilterState, Project: m.project}
		}, true
	}

	return nil, nil, false
}

func (m *OutlinerModel) handleUtilityKey(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "v":
		m.toggleMultiSelectMode()
		return m, nil, true

	case "y", "Y", "ctrl+y":
		if handled, cmd := handleClipboardForPrefixableView(key, m); handled {
			return m, cmd, true
		}
		return m, nil, true

	case "T":
		if m.project == nil {
			tags := BuildTagList(m.crossMergedTaskSlice(), false)
			return m, func() tea.Msg {
				return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewCrossProjectOutliner, Title: "Filter by Tags"}
			}, true
		}
		cmd, _ := handleTagKey(m, ViewOutliner, false, m.filter.Tags)
		return m, cmd, true

	case "n":
		return m, m.jumpToNextSection(1), true

	case "N":
		return m, m.jumpToNextSection(-1), true

	case "C":
		return m, func() tea.Msg {
			return ShowContextPickerMsg{}
		}, true

	case "f":
		if m.project == nil {
			return m, nil, true
		}
		return m, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}, true

	case "A":
		if m.project == nil {
			return m, nil, true
		}
		if m.activePerspective == PerspectiveCompleted {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirm.Show("archive-tasks",
					fmt.Sprintf("Archive %d completed task(s) to completed.md?", completedCount))
			}
			return m, nil, true
		}
		return m, func() tea.Msg {
			return ShowAllTasksMsg{}
		}, true

	case "/":
		return m, m.ActivateSearch(), true

	case "?":
		if m.project == nil {
			return m, helpCmd(ViewCrossProjectOutliner, "Cross-Project Outliner", m.GetHelpItems()), true
		}
		return m, helpCmd(ViewOutliner, "Outliner", m.GetHelpItems()), true

	case "<", ">", "H", "L":
		if m.project == nil {
			cmd, _ := m.crossHandleTypeChange(key)
			return m, cmd, true
		}
		cmd, _ := handleTypeChangeKey(key, m)
		return m, cmd, true

	case "*":
		if m.project == nil {
			cmd, _ := m.crossToggleActive()
			return m, cmd, true
		}
		cmd, _ := handleToggleActiveKey(m)
		return m, cmd, true

	case "[", "]":
		if tabs := m.customSectionTabs(); tabs != nil {
			tab := m.customSectionTabIndex()
			cycleCustomSectionTab(key, &tab, len(tabs), func() {
				m.setCustomSectionTabIndex(tab)
				m.rebuildRows()
			})
		}
		return m, nil, true

	case "ctrl+a":
		if m.project == nil {
			return m, nil, true
		}
		if m.project.Sandbox {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
			}, true
		}
		return m, m.archiveProject(), true

	case "ctrl+t":
		if m.project == nil {
			return m, nil, true
		}
		return m, func() tea.Msg {
			return ShowTagManagerMsg{Project: m.project}
		}, true

	case "ctrl+s":
		if m.project == nil {
			return m, nil, true
		}
		return m, func() tea.Msg {
			return ShowSectionManagerMsg{ProjectTree: nil, Project: m.project}
		}, true

	case "1", "2", "3", "4", "5", "0":
		m.clearActiveSavedFilter()
		handleSortKey(key, &m.sort,
			func() { m.rebuildRows() },
			func(SortField) { m.rebuildRows() },
		)
		return m, nil, true
	}

	return nil, nil, false
}
