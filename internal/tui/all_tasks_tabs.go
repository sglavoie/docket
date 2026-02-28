package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

var defaultBuiltInTabNames = []string{"All", "Active", "Due Soon", "High Pri", "Blocked", "By Score"}

const (
	builtInTabAll     = 0
	builtInTabActive  = 1
	builtInTabDueSoon = 2
	builtInTabHighPri = 3
	builtInTabBlocked = 4
	builtInTabByScore = 5
)

// buildTabEntries constructs the ordered list of tab entries from config.
// If AllTasksTabOrder is set, it determines ordering; otherwise built-in tabs
// come first followed by smart lists in definition order.
func (m *AllTasksViewModel) buildTabEntries() {
	builtInByName := make(map[string]int)
	for i, name := range defaultBuiltInTabNames {
		builtInByName[name] = i
	}

	smartByID := make(map[string]bool)
	for _, sl := range m.config.SmartLists {
		smartByID[sl.ID] = true
	}

	var entries []TabEntry
	used := make(map[string]bool)

	if len(m.config.AllTasksTabOrder) > 0 {
		for _, key := range m.config.AllTasksTabOrder {
			if id, ok := builtInByName[key]; ok && !used[key] {
				entries = append(entries, TabEntry{Name: key, IsBuiltIn: true, BuiltInID: id})
				used[key] = true
			} else if smartByID[key] && !used[key] {
				sl := m.config.GetSmartList(key)
				if sl != nil {
					entries = append(entries, TabEntry{Name: sl.Name, IsBuiltIn: false, SmartListID: key})
					used[key] = true
				}
			}
		}
	}

	// Append any built-in tabs not yet included (preserve default order)
	for i, name := range defaultBuiltInTabNames {
		if !used[name] {
			entries = append(entries, TabEntry{Name: name, IsBuiltIn: true, BuiltInID: i})
		}
	}

	// Append any smart lists not yet included
	for _, sl := range m.config.SmartLists {
		if !used[sl.ID] {
			entries = append(entries, TabEntry{Name: sl.Name, IsBuiltIn: false, SmartListID: sl.ID})
		}
	}

	m.tabEntries = entries

	names := make([]string, len(entries))
	for i, e := range entries {
		if !e.IsBuiltIn {
			names[i] = "● " + e.Name
		} else {
			names[i] = e.Name
		}
	}
	m.tabs.Names = names

	// Clamp active tab in case entries shrank
	if m.tabs.Active >= len(entries) {
		m.tabs.Active = 0
	}
}

// applyTabFilter filters tasks based on the active tab entry.
func (m *AllTasksViewModel) applyTabFilter(tasks []CrossProjectTask) []CrossProjectTask {
	if m.tabs.Active < 0 || m.tabs.Active >= len(m.tabEntries) {
		return tasks
	}
	entry := m.tabEntries[m.tabs.Active]

	if entry.IsBuiltIn {
		return m.applyBuiltInTabFilter(tasks, entry.BuiltInID)
	}

	// Smart list tab
	sl := m.config.GetSmartList(entry.SmartListID)
	if sl == nil {
		return tasks
	}
	return m.applySmartListFilter(tasks, sl)
}

// applyBuiltInTabFilter applies the filter logic for built-in tabs.
func (m *AllTasksViewModel) applyBuiltInTabFilter(tasks []CrossProjectTask, builtInID int) []CrossProjectTask {
	now := time.Now()
	today := util.StartOfDay(now)

	switch builtInID {
	case builtInTabActive:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.Status == models.StatusInProgress {
				result = append(result, item)
			}
		}
		return result

	case builtInTabDueSoon:
		sevenDaysLater := today.AddDate(0, 0, 7)
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.DueDate == nil {
				continue
			}
			dueDay := util.StartOfDay(*item.Task.DueDate)
			if dueDay.Before(sevenDaysLater) {
				result = append(result, item)
			}
		}
		return result

	case builtInTabHighPri:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.Priority <= 2 {
				result = append(result, item)
			}
		}
		return result

	case builtInTabBlocked:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.Status == models.StatusBlocked {
				result = append(result, item)
			}
		}
		return result

	case builtInTabByScore:
		var result []CrossProjectTask
		for _, item := range tasks {
			profile := m.config.GetProjectScoringProfile(item.Project.ID)
			if profile != nil && HasManualScores(item.Task, profile) {
				result = append(result, item)
			}
		}
		sort.SliceStable(result, func(i, j int) bool {
			pi := m.config.GetProjectScoringProfile(result[i].Project.ID)
			pj := m.config.GetProjectScoringProfile(result[j].Project.ID)
			si := scoring.ComputeScore(result[i].Task, pi)
			sj := scoring.ComputeScore(result[j].Task, pj)
			return si > sj
		})
		return result
	}

	return tasks
}

// applySmartListFilter filters tasks using a smart list's criteria.
func (m *AllTasksViewModel) applySmartListFilter(tasks []CrossProjectTask, sl *config.SmartList) []CrossProjectTask {
	var result []CrossProjectTask
	for _, item := range tasks {
		// Smart lists skip completed tasks unless explicitly included
		if item.Task.Status.IsComplete() && !sl.IncludeCompleted {
			continue
		}
		if passesSmartListCriteria(item.Task, item.Project, sl, m.config) {
			result = append(result, item)
		}
	}
	return result
}

// ensureTabOrderInitialized populates AllTasksTabOrder from the current
// tabEntries if it hasn't been set yet, so that reordering has something
// to permute.
func (m *AllTasksViewModel) ensureTabOrderInitialized() {
	if len(m.config.AllTasksTabOrder) > 0 {
		return
	}
	order := make([]string, len(m.tabEntries))
	for i, e := range m.tabEntries {
		if e.IsBuiltIn {
			order[i] = e.Name
		} else {
			order[i] = e.SmartListID
		}
	}
	m.config.AllTasksTabOrder = order
}

// moveTabLeft swaps the active tab with the one to its left.
func (m *AllTasksViewModel) moveTabLeft() {
	if m.tabs.Active <= 0 {
		return
	}
	m.ensureTabOrderInitialized()
	i := m.tabs.Active
	m.config.AllTasksTabOrder[i-1], m.config.AllTasksTabOrder[i] = m.config.AllTasksTabOrder[i], m.config.AllTasksTabOrder[i-1]
	m.tabs.Active--
	m.buildTabEntries()
	_ = m.configManager.Save(m.config)
	m.statusMessage = "Tab moved left"
}

// moveTabRight swaps the active tab with the one to its right.
func (m *AllTasksViewModel) moveTabRight() {
	if m.tabs.Active >= len(m.tabEntries)-1 {
		return
	}
	m.ensureTabOrderInitialized()
	i := m.tabs.Active
	m.config.AllTasksTabOrder[i], m.config.AllTasksTabOrder[i+1] = m.config.AllTasksTabOrder[i+1], m.config.AllTasksTabOrder[i]
	m.tabs.Active++
	m.buildTabEntries()
	_ = m.configManager.Save(m.config)
	m.statusMessage = "Tab moved right"
}

// handleDeleteSmartList deletes the smart list on the currently active tab.
func (m *AllTasksViewModel) handleDeleteSmartList() (tea.Model, tea.Cmd) {
	if m.tabs.Active < 0 || m.tabs.Active >= len(m.tabEntries) {
		return m, nil
	}
	entry := m.tabEntries[m.tabs.Active]
	if entry.IsBuiltIn || entry.SmartListID == "" {
		return m, nil
	}
	id := entry.SmartListID
	return m, func() tea.Msg {
		return SmartListDeletedMsg{ID: id}
	}
}
