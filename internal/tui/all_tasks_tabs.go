package tui

import (
	"image/color"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

var defaultBuiltInTabNames = []string{"All", "Active", "Parked", "Due Soon", "High Pri", "Blocked", "By Score"}

const (
	builtInTabActive  = 1
	builtInTabParked  = 2
	builtInTabDueSoon = 3
	builtInTabHighPri = 4
	builtInTabBlocked = 5
	builtInTabByScore = 6
)

// overflowTabCap is the maximum number of visible tabs (including the "More…" entry).
const overflowTabCap = 7

// buildTabEntries constructs the fixed Task Workspace focus tabs.
func (m *AllTasksViewModel) buildTabEntries() {
	entries := make([]TabEntry, 0, len(defaultBuiltInTabNames))
	for i, name := range defaultBuiltInTabNames {
		entries = append(entries, TabEntry{Name: name, IsBuiltIn: true, BuiltInID: i})
	}

	// Store the full list; then cap visible entries at overflowTabCap.
	m.allTabEntries = entries
	if len(entries) > overflowTabCap {
		visible := make([]TabEntry, overflowTabCap)
		copy(visible, entries[:overflowTabCap-1])
		visible[overflowTabCap-1] = TabEntry{Name: "More…", IsOverflow: true}
		m.tabEntries = visible
	} else {
		m.tabEntries = entries
	}

	names := make([]string, len(m.tabEntries))
	tabColors := make([]color.Color, len(m.tabEntries))
	for i, e := range m.tabEntries {
		switch {
		case e.IsOverflow:
			names[i] = "More…"
		case !e.IsBuiltIn:
			names[i] = "● " + e.Name
			tabColors[i] = ColorAccent
		default:
			names[i] = e.Name
		}
	}
	m.tabs.Names = names
	m.tabs.TabColors = tabColors

	// Clamp activeTabIdx and sync tabs.Active.
	if m.activeTabIdx >= len(m.allTabEntries) {
		m.activeTabIdx = 0
	}
	m.tabs.Active = min(m.activeTabIdx, len(m.tabEntries)-1)
}

// activeTabEntry returns the TabEntry for the currently active tab,
// using activeTabIdx into allTabEntries.
func (m *AllTasksViewModel) activeTabEntry() *TabEntry {
	if m.activeTabIdx >= 0 && m.activeTabIdx < len(m.allTabEntries) {
		return &m.allTabEntries[m.activeTabIdx]
	}
	if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
		return &m.tabEntries[m.tabs.Active]
	}
	return nil
}

// applyTabFilter filters tasks based on the active tab entry.
func (m *AllTasksViewModel) applyTabFilter(tasks []CrossProjectTask) []CrossProjectTask {
	entry := m.activeTabEntry()
	if entry == nil || entry.IsOverflow {
		return tasks
	}

	if entry.IsBuiltIn {
		return m.applyBuiltInTabFilter(tasks, entry.BuiltInID)
	}

	// Saved filter tab
	sf := m.runtime.Planning().GetSavedFilter(entry.SavedFilterID)
	if sf == nil {
		return tasks
	}
	return m.applySavedFilterToAllTasks(tasks, sf)
}

// applyBuiltInTabFilter applies the filter logic for built-in tabs.
func (m *AllTasksViewModel) applyBuiltInTabFilter(tasks []CrossProjectTask, builtInID int) []CrossProjectTask {
	now := time.Now()
	today := util.StartOfDay(now)

	switch builtInID {
	case builtInTabActive:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.IsInState(models.StateInProgress) {
				result = append(result, item)
			}
		}
		return result

	case builtInTabParked:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.IsParked() {
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
			if item.Task.IsInState(models.StateWaiting) {
				result = append(result, item)
			}
		}
		return result

	case builtInTabByScore:
		var result []CrossProjectTask
		for _, item := range tasks {
			profile := m.runtime.Automation().GetProjectScoringProfile(item.Project.ID)
			if profile != nil && HasManualScores(item.Task, profile) {
				result = append(result, item)
			}
		}
		sort.SliceStable(result, func(i, j int) bool {
			pi := m.runtime.Automation().GetProjectScoringProfile(result[i].Project.ID)
			pj := m.runtime.Automation().GetProjectScoringProfile(result[j].Project.ID)
			si := scoring.ComputeScore(result[i].Task, pi)
			sj := scoring.ComputeScore(result[j].Task, pj)
			return si > sj
		})
		return result
	}

	return tasks
}

// applySavedFilterToAllTasks filters tasks using a saved filter.
// For criteria-mode filters, applies per-task criteria matching.
// For snapshot-mode filters, applies flat filter fields.
func (m *AllTasksViewModel) applySavedFilterToAllTasks(tasks []CrossProjectTask, sf *config.SavedFilter) []CrossProjectTask {
	var result []CrossProjectTask
	for _, item := range tasks {
		// Skip completed tasks unless explicitly included
		if item.Task.IsTerminalState() && !sf.IncludeCompleted {
			continue
		}
		if passesSavedFilterCriteria(item.Task, item.Project, sf, m.runtime.Automation()) {
			result = append(result, item)
		}
	}
	return result
}
