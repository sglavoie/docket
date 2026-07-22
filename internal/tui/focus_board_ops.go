package tui

import (
	"os/exec"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

type focusBoardReloadMode int

const (
	focusBoardReloadPreserveSelection focusBoardReloadMode = iota
	focusBoardReloadNextActionable
)

type focusBoardReloadIntent struct {
	valid  bool
	mode   focusBoardReloadMode
	taskID string
	cursor int
}

// markDone marks the current task as done via the shared workflow pipeline.
func (m *FocusBoardModel) markDone() (tea.Model, tea.Cmd) {
	m.captureReloadIntent(focusBoardReloadNextActionable)
	cmd := wfChangeState(m, models.StateDone)
	m.setCompletionFlash()
	m.zenAdvance()
	return m, tea.Batch(cmd, completionFlashScheduler())
}

// dismiss cancels the current task via the shared workflow pipeline.
func (m *FocusBoardModel) dismiss() (tea.Model, tea.Cmd) {
	m.captureReloadIntent(focusBoardReloadNextActionable)
	cmd := wfChangeState(m, models.StateCanceled)
	m.setMutationFlash(stateMutationFlashBg(models.StateCanceled))
	m.zenAdvance()
	return m, tea.Batch(cmd, mutationFlashTimeout())
}

// zenAdvance moves the cursor to the next incomplete task when in zen/tunnel mode.
func (m *FocusBoardModel) zenAdvance() {
	if m.focusMode == focusModeList {
		return
	}
	m.advanceToNextActionableRow()
}

// handleEnter opens the selected task detail view.
func (m *FocusBoardModel) handleEnter() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		return ShowTaskDetailMsg{Task: task, Project: project}
	}
}

// taskOpsView implementation for FocusBoardModel.

func (m *FocusBoardModel) getTaskRef() (*models.Task, *models.Project) {
	row := m.getRowAtCursor()
	if row == nil {
		return nil, nil
	}
	return row.Task, row.Project
}

func (m *FocusBoardModel) getRuntime() *config.RuntimeState  { return m.runtime }
func (m *FocusBoardModel) getConfigManager() *config.Manager { return m.configManager }

// workflowOpsView accessors: FocusBoard does not embed ViewBundle, so it exposes
// its own single-task coordinator and enabled rules for the shared wf* helpers.
func (m *FocusBoardModel) taskItemCoordinator() TaskItemCoordinator { return m.taskItem }
func (m *FocusBoardModel) workflowRules() []workflow.WorkflowRule {
	if m.runtime == nil {
		return nil
	}
	return m.runtime.Automation().GetEnabledWorkflowRules()
}
func (m *FocusBoardModel) reloadData() {
	intent := m.consumeReloadIntent(focusBoardReloadPreserveSelection)
	m.loadItems()
	m.restoreCursorAfterReload(intent)
}

func (m *FocusBoardModel) refreshData() {
	m.reloadData()
	m.clampCursor()
}

func (m *FocusBoardModel) captureReloadIntent(mode focusBoardReloadMode) {
	m.pendingReloadIntent = m.currentReloadIntent(mode)
}

func (m *FocusBoardModel) currentReloadIntent(mode focusBoardReloadMode) focusBoardReloadIntent {
	intent := focusBoardReloadIntent{
		valid:  true,
		mode:   mode,
		cursor: m.cursor,
	}
	if row := m.getRowAtCursor(); row != nil && row.Task != nil {
		intent.taskID = row.Task.ID
	}
	return intent
}

func (m *FocusBoardModel) consumeReloadIntent(defaultMode focusBoardReloadMode) focusBoardReloadIntent {
	if m.pendingReloadIntent.valid {
		intent := m.pendingReloadIntent
		m.pendingReloadIntent = focusBoardReloadIntent{}
		return intent
	}
	return m.currentReloadIntent(defaultMode)
}

func (m *FocusBoardModel) restoreCursorAfterReload(intent focusBoardReloadIntent) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if intent.mode == focusBoardReloadNextActionable {
		if idx := m.nextActionableRowIndex(intent.cursor); idx >= 0 {
			m.cursor = idx
			return
		}
	}
	if intent.taskID != "" {
		if idx := restoreCursorByID(intent.taskID, m.filtered, func(r boardRow) string {
			if r.Task != nil {
				return r.Task.ID
			}
			return ""
		}); idx >= 0 {
			m.cursor = idx
			return
		}
	}
	m.cursor = intent.cursor
	m.clampCursor()
}

func (m *FocusBoardModel) advanceToNextActionableRow() bool {
	if idx := m.nextActionableRowIndex(m.cursor); idx >= 0 {
		m.cursor = idx
		return true
	}
	return false
}

func (m *FocusBoardModel) nextActionableRowIndex(start int) int {
	if len(m.filtered) == 0 {
		return -1
	}
	if start < 0 {
		start = 0
	}
	if start >= len(m.filtered) {
		start = len(m.filtered) - 1
	}
	for offset := 0; offset < len(m.filtered); offset++ {
		idx := (start + offset) % len(m.filtered)
		if m.isActionableFlowRow(m.filtered[idx]) {
			return idx
		}
	}
	return -1
}

func (m *FocusBoardModel) isActionableFlowRow(row boardRow) bool {
	return row.Task != nil && !row.Task.IsComplete()
}

func (m *FocusBoardModel) setStatus(msg string) { m.statusMessage = msg }

func (m *FocusBoardModel) parentRowIndices() []int {
	var indices []int
	for i, row := range m.filtered {
		if row.Task != nil && row.TreePrefix == "" {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *FocusBoardModel) focusBoardGoToNextParent() tea.Cmd {
	old := m.cursor
	cmd := m.SectionGoToNext(m.parentRowIndices())
	m.clampCursor()
	if cmd != nil {
		return cmd
	}
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *FocusBoardModel) focusBoardGoToPrevParent() tea.Cmd {
	old := m.cursor
	cmd := m.SectionGoToPrev(m.parentRowIndices())
	m.clampCursor()
	if cmd != nil {
		return cmd
	}
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *FocusBoardModel) ResetCursor() { m.cursor = 0 }

func (m *FocusBoardModel) JumpToTop() tea.Cmd {
	old := m.cursor
	m.cursor = 0
	m.clampCursor()
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}

// handleDailyPlanReorder swaps the cursor item with an adjacent item in the Today tab.
func (m *FocusBoardModel) handleDailyPlanReorder(delta int) (tea.Model, tea.Cmd) {
	if m.tabs.Active != boardTabToday {
		return m, nil
	}
	if m.reorderBlockedBySecondarySort("Today order") {
		return m, nil
	}
	to := m.cursor + delta
	if m.cursor < 0 || m.cursor >= len(m.filtered) || to < 0 || to >= len(m.filtered) {
		return m, nil
	}

	curRow := m.filtered[m.cursor]
	targetRow := m.filtered[to]
	if curRow.Task == nil || targetRow.Task == nil || m.dailyPlan == nil {
		return m, nil
	}

	// Auto-expanded children cannot be reordered — they follow their parent.
	if curRow.TreePrefix != "" || targetRow.TreePrefix != "" {
		return m, nil
	}

	// Only reorder within the same completion group (active-first partition).
	if curRow.Task.IsComplete() != targetRow.Task.IsComplete() {
		return m, nil
	}

	// Build a detached candidate so a failed checked commit cannot mutate the
	// shared Daily Plan through its aliased slice backing array.
	dp := append([]FocusTask(nil), (*m.dailyPlan)...)
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

	if err := m.persistDailyPlan(dp); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return m, nil
	}
	*m.dailyPlan = dp

	// Publish the display changes only after the candidate is durable.
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

// persistDailyPlan saves a candidate daily plan order through a checked planning
// commit. It does not publish the candidate to the shared Daily Plan; the caller
// does that only after this helper succeeds.
func (m *FocusBoardModel) persistDailyPlan(dp []FocusTask) error {
	today := time.Now().Format("2006-01-02")
	items := make([]config.DailyPlanItem, len(dp))
	for i, ft := range dp {
		items[i] = config.DailyPlanItem{
			TaskID:      ft.Task.ID,
			ProjectID:   ft.Project.ID,
			PlannedDate: today,
		}
	}
	return m.commitPlanning(func(p *config.PlanningState) {
		p.SetDailyPlan(items)
	})
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
