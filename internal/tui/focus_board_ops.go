package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// markDone marks the current task as done via the shared workflow pipeline.
func (m *FocusBoardModel) markDone() (tea.Model, tea.Cmd) {
	model, cmd := taskOpsChangeStatus(m, models.StatusDone)
	m.setCompletionFlash()
	m.zenAdvance()
	return model, tea.Batch(cmd, completionFlashScheduler())
}

// dismiss cancels the current task via the shared workflow pipeline.
func (m *FocusBoardModel) dismiss() (tea.Model, tea.Cmd) {
	model, cmd := taskOpsChangeStatus(m, models.StatusCanceled)
	m.setMutationFlash(statusMutationFlashBg(models.StatusCanceled))
	m.zenAdvance()
	return model, tea.Batch(cmd, mutationFlashTimeout())
}

// zenAdvance moves the cursor to the next incomplete task when in zen/tunnel mode.
func (m *FocusBoardModel) zenAdvance() {
	if m.focusMode == focusModeList {
		return
	}
	for i := m.cursor; i < len(m.filtered); i++ {
		if m.filtered[i].Task != nil && !m.filtered[i].Task.Status.IsComplete() {
			m.cursor = i
			return
		}
	}
	for i := 0; i < m.cursor && i < len(m.filtered); i++ {
		if m.filtered[i].Task != nil && !m.filtered[i].Task.Status.IsComplete() {
			m.cursor = i
			return
		}
	}
}

// handleEnter navigates to the task in its project.
func (m *FocusBoardModel) handleEnter() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		return FocusBoardSelectMsg{Task: row.Task, Project: row.Project}
	}
}

// deleteItem deletes the task at cursor via the shared workflow pipeline.
func (m *FocusBoardModel) deleteItem() (tea.Model, tea.Cmd) {
	return taskOpsDeleteTask(m)
}

// taskOpsView implementation for FocusBoardModel.

func (m *FocusBoardModel) getTaskRef() (*models.Task, *models.Project) {
	row := m.getRowAtCursor()
	if row == nil {
		return nil, nil
	}
	return row.Task, row.Project
}

func (m *FocusBoardModel) getPersister() Persister           { return m.projectService }
func (m *FocusBoardModel) getConfigManager() *config.Manager { return m.configManager }
func (m *FocusBoardModel) reloadData() {
	var taskID string
	if row := m.getRowAtCursor(); row != nil && row.Task != nil {
		taskID = row.Task.ID
	}
	m.loadItems()
	if taskID != "" {
		if idx := restoreCursorByID(taskID, m.filtered, func(r boardRow) string {
			if r.Task != nil {
				return r.Task.ID
			}
			return ""
		}); idx >= 0 {
			m.cursor = idx
		}
	}
}
func (m *FocusBoardModel) setErr(err error)     { m.statusMessage = err.Error() }
func (m *FocusBoardModel) setStatus(msg string) { m.statusMessage = msg }
func (m *FocusBoardModel) viewID() ViewType     { return ViewFocusBoard }
func (m *FocusBoardModel) viewName() string     { return "Today" }

func (m *FocusBoardModel) showDeleteConfirm(shortID int) {
	if row := m.getRowAtCursor(); row != nil {
		m.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", shortID))
	}
}

func (m *FocusBoardModel) ResetCursor() { m.cursor = 0 }

func (m *FocusBoardModel) JumpToTop() tea.Cmd {
	old := m.cursor
	m.cursor = 0
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}

// handleDailyPlanReorder swaps the cursor item with an adjacent item in the Today tab.
func (m *FocusBoardModel) handleDailyPlanReorder(delta int) (tea.Model, tea.Cmd) {
	if m.tabs.Active != boardTabToday {
		return m, nil
	}
	to := m.cursor + delta
	if m.cursor < 0 || m.cursor >= len(m.filtered) || to < 0 || to >= len(m.filtered) {
		return m, nil
	}

	curRow := m.filtered[m.cursor]
	targetRow := m.filtered[to]

	// Auto-expanded children cannot be reordered — they follow their parent.
	if curRow.TreePrefix != "" || targetRow.TreePrefix != "" {
		return m, nil
	}

	// Only reorder within the same completion group (active-first partition).
	if curRow.Task.Status.IsComplete() != targetRow.Task.Status.IsComplete() {
		return m, nil
	}

	// Swap in the canonical dailyPlan slice.
	dp := *m.dailyPlan
	curIdx, targetIdx := -1, -1
	for i, ft := range dp {
		switch ft.Task.ID {
		case curRow.Task.ID:
			curIdx = i
		case targetRow.Task.ID:
			targetIdx = i
		}
	}
	if curIdx < 0 || targetIdx < 0 {
		return m, nil
	}
	dp[curIdx], dp[targetIdx] = dp[targetIdx], dp[curIdx]

	m.persistDailyPlan()

	// Swap in display slices and move cursor.
	m.filtered[m.cursor], m.filtered[to] = m.filtered[to], m.filtered[m.cursor]

	// todayRows may differ from filtered when search/filters are active,
	// so look up the correct indices by task ID instead of using cursor position.
	todayCurIdx, todayToIdx := -1, -1
	for i, r := range m.todayRows {
		switch r.Task.ID {
		case curRow.Task.ID:
			todayCurIdx = i
		case targetRow.Task.ID:
			todayToIdx = i
		}
	}
	if todayCurIdx >= 0 && todayToIdx >= 0 {
		m.todayRows[todayCurIdx], m.todayRows[todayToIdx] = m.todayRows[todayToIdx], m.todayRows[todayCurIdx]
	}

	m.cursor = to
	return m, nil
}

// persistDailyPlan saves the current daily plan order to config.
func (m *FocusBoardModel) persistDailyPlan() {
	if m.configManager == nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	dp := *m.dailyPlan
	items := make([]config.DailyPlanItem, len(dp))
	for i, ft := range dp {
		items[i] = config.DailyPlanItem{
			TaskID:      ft.Task.ID,
			ProjectID:   ft.Project.ID,
			PlannedDate: today,
		}
	}
	m.config.SetDailyPlan(items)
	if err := m.configManager.Save(m.config); err != nil {
		m.setStatus("Failed to save: " + err.Error())
	}
}

// openBrowserCmd opens a URL in the default browser.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
		}
		return nil
	}
}
