package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// coreMoveCursor takes a shared list-navigation key rather than a delta so steps,
// edge jumps, and half-page jumps all inherit the same viewport sizing, boundary
// flashing, and non-navigable row skipping.
func (m *AppModel) coreMoveCursor(key string) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	previous := out.Cursor
	handled, cmd := listsurface.HandleListNavigation(
		key,
		&out.Cursor,
		len(out.Rows),
		out.ContentHeight(),
		&out.CursorFlash,
	)
	if !handled {
		return nil
	}
	cmd = out.CrossSkipNonNavigableRows(previous, cmd)
	out.UpdateRangeSelection()
	out.VoidRelocationCaptureAfterNavigation(previous)
	return cmd
}

// windowMiddleIndex is the offset of the middle row inside a window of the given
// height. zz reads it to decide where "middle" is.
func windowMiddleIndex(height int) int { return (height - 1) / 2 }

// viewScrollAnchor names where in the window zt/zz/zb park the cursor's row.
type viewScrollAnchor int

const (
	viewScrollAnchorTop viewScrollAnchor = iota
	viewScrollAnchorMiddle
	viewScrollAnchorBottom
)

// coreScrollCursorTo moves the window so the cursor's row sits at the requested
// position, leaving the cursor itself alone. The written offset still contains
// the cursor, which is exactly what adjustPersistentScrollOffset leaves
// untouched on the next render. Success is silent: a status here would arm the
// footer flash for a change already fully visible on screen.
func (m *AppModel) coreScrollCursorTo(anchor viewScrollAnchor) operation.OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return operation.OperationHandlerResult{}
	}
	start, end := out.VisibleRowWindow()
	if start >= end {
		return operation.OperationHandlerResult{}
	}
	height := end - start
	desired := out.Cursor
	switch anchor {
	case viewScrollAnchorMiddle:
		desired -= windowMiddleIndex(height)
	case viewScrollAnchorBottom:
		desired -= height - 1
	case viewScrollAnchorTop:
	}
	out.ViewScrollOffset = min(max(desired, 0), max(len(out.Rows)-height, 0))
	return operation.OperationHandlerResult{}
}

// coreGotoTaskProject is the task-row counterpart to Enter on a project header:
// it scopes the Outliner to the cursor task's project and forces All so the
// whole tree is listed rather than the origin mode's narrowing. The scope push
// captures the frame Esc restores, so the mode switch must follow it — reversed,
// Esc would return to All instead of the mode the jump was pressed from. A mode
// that lists the task where All does not (a Done task reached from Recent) keeps
// the origin instead, and says so.
func (m *AppModel) coreGotoTaskProject(targets operation.ResolvedOperationTargets) operation.OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return operation.OperationHandlerResult{}
	}
	// Search is dismissed before the origin mode is read and before the scope
	// push, since that push captures the frame Esc restores and it must be the
	// pre-search one.
	out.ClearSearch()
	task, project := m.taskAndProjectForTargets(targets)
	if task == nil || project == nil {
		return operation.OperationHandlerResult{}
	}
	origin := out.ActiveOutlinerModeEntry()
	previous := out.Cursor

	out.PushProjectScope(project.ID)
	out.SelectOutlinerMode(chrome.ModeAll)
	out.NavigateToTaskInProject(task, project)

	message := "Project: " + project.Name
	if _, found := out.RowIndexForTask(taskref.For(task, project)); !found && origin.ID != chrome.ModeAll {
		out.SelectOutlinerMode(origin.ID)
		out.NavigateToTaskInProject(task, project)
		message += " — kept " + origin.Label + ", the task is not listed in All"
	}
	m.setInfoStatus(message)
	return operation.OperationHandlerResult{
		Message: commandMessage(tea.Batch(
			listsurface.FlashIfJumped(previous, out.Cursor, &out.CursorFlash),
			status.DefaultStatusTimeout(),
		)),
	}
}

func (m *AppModel) coreReorder(delta int) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil || !out.SimplifiedReorderEligible() {
		if out != nil {
			out.OrderingUnavailableStatus()
		}
		return nil
	}
	if out.IsPinnedReorderRow() {
		return out.PinnedReorderMove(delta)
	}
	if out.IsPlanningReorderMode() {
		_, cmd := out.PlanningReorderMove(delta)
		return cmd
	}
	return out.ReorderTask(delta)
}

// coreReorderEdge is the single-press equivalent of repeating coreReorder until
// the row can move no further, and takes the same ordering-context dispatch so
// the two families can never disagree about which durable order a row belongs to.
func (m *AppModel) coreReorderEdge(delta int) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil || !out.SimplifiedReorderEligible() {
		if out != nil {
			out.OrderingUnavailableStatus()
		}
		return nil
	}
	if out.IsPinnedReorderRow() {
		return out.PinnedReorderEdge(delta)
	}
	if out.IsPlanningReorderMode() {
		_, cmd := out.PlanningReorderEdge(delta)
		return cmd
	}
	return out.ReorderTaskEdge(delta)
}

func (m *AppModel) coreUndo() tea.Cmd {
	if m.feedback.undoLive(time.Now()) {
		callback := m.feedback.undoCb
		m.clearStatus()
		return callback()
	}
	if cmd := m.handleUndoToggle(); cmd != nil {
		return cmd
	}
	_, project := m.getCurrentCursorTaskAndProject()
	return m.performGlobalTaskUndoRedoForProject(project, false)
}

func (m *AppModel) coreRedo() tea.Cmd {
	if cmd := m.handleRedoToggle(); cmd != nil {
		return cmd
	}
	_, project := m.getCurrentCursorTaskAndProject()
	return m.performGlobalTaskUndoRedoForProject(project, true)
}

// coreToggleTheme flips exactly between the two maintained machine-local
// variants and rebuilds every active style immediately.
// coreResetToLaunchView is 0: the view returns to the one a lens-free launch
// produces and says which axes it had to put down to get there. It takes the
// population reload the F-chord facets owe whenever it cleared one of them,
// through the same command Fx raises, so the projects on screen are the Active
// Projects the launch view names rather than the wider set a facet had opened.
func (m *AppModel) coreResetToLaunchView() operation.OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	cleared, reload := out.ResetToLaunchView()
	if len(cleared) == 0 {
		return operation.OperationHandlerResult{Status: "Already at the launch view"}
	}
	status := "Launch view restored · cleared " + strings.Join(cleared, ", ")
	if reload {
		return operation.OperationHandlerResult{
			Message: operation.OperationCommandResultMsg{Cmd: m.projectFilterReloadCmd(status)},
		}
	}
	return operation.OperationHandlerResult{Status: status}
}

func (m *AppModel) coreToggleTheme() error {
	if m.runtime == nil || m.runtime.Machine() == nil {
		return fmt.Errorf("machine theme is unavailable")
	}
	machine := *m.runtime.Machine()
	if machine.Theme == "light" {
		machine.Theme = "dark"
	} else {
		machine.Theme = "light"
	}
	if err := m.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save theme: %w", err)
	}
	theme.ApplyTheme(theme.NewThemeConfig(m.runtime.Machine()))
	m.setInfoStatus("Theme: " + machine.Theme)
	return nil
}

// coreToggleMenuBar shows and hides the Outliner's menu bar. Visibility is
// machine-local for the same reason the theme is: chrome follows the terminal
// you are sitting at rather than the workspace you opened, so a machine that
// wants the bar keeps it without every other machine inheriting it.
//
// Nothing is recomputed here. The bar costs one row of the list, and
// contentHeight is the one place that subtracts it, so the next render reads
// the committed flag and every budget follows from that single definition.
func (m *AppModel) coreToggleMenuBar() error {
	if m.runtime == nil || m.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.runtime.Machine()
	machine.MenuBarVisible = !machine.MenuBarVisible
	if err := m.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save menu bar visibility: %w", err)
	}
	if machine.MenuBarVisible {
		m.setInfoStatus("Menu bar shown")
	} else {
		m.setInfoStatus("Menu bar hidden")
	}
	return nil
}

// coreOpenMenuBar drops the first menu out of the bar, revealing the bar itself
// first when it is hidden. Revealing rather than refusing is what makes this one
// key enough to find the whole grammar: a user who has never pressed `M` has no
// way to know the band exists, so the key that opens a menu has to be able to
// put one on screen from nothing.
//
// The reveal commits the same machine-local flag `M` writes, so a menu opened
// out of a hidden bar leaves the bar shown afterwards rather than snapping back
// — the state a user can see is the state that was saved.
func (m *AppModel) coreOpenMenuBar() error {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return fmt.Errorf("the menu bar belongs to the Outliner")
	}
	if !out.MenuBarVisible() {
		if m.runtime == nil || m.runtime.Machine() == nil {
			return fmt.Errorf("machine chrome state is unavailable")
		}
		machine := *m.runtime.Machine()
		machine.MenuBarVisible = true
		if err := m.runtime.CommitMachine(&machine); err != nil {
			return fmt.Errorf("save menu bar visibility: %w", err)
		}
	}
	out.OpenMenuBar()
	return nil
}

// coreInputHelpItems documents the Outliner from the two grammars a user meets
// there: the registry's keys and the shared sigil language they type into
// capture, inline edit, and search. The grammar rows are appended here rather
// than inside HelpItemsForInputStrategy so that stays a pure registry
// projection and the grammar section stays Outliner-scoped.
func (m *AppModel) coreInputHelpItems() []operation.HelpItem {
	if m.operations.registry == nil {
		return nil
	}
	items := m.operations.registry.HelpItemsForInputStrategy(
		operation.InputStrategySimplified,
		m.operationCommandContext(),
	)
	return append(items, help.GrammarHelpItems()...)
}
