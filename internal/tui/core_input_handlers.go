package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func commandMessage(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return operation.OperationCommandResultMsg{Cmd: cmd}
}

// coreNotesScope keeps Ctrl+N on All Notes everywhere except a project header
// row, which names one project explicitly rather than incidentally — the same
// exception coreCaptureMessage makes, read through the same predicate, so no
// task cursor can narrow the manager. The header is re-resolved against the
// shell's note-owner population, since an archived project the archived facet
// loaded is never an owner and would only bounce back to All Notes.
func (m *AppModel) coreNotesScope() route.NotesScope {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return route.AllNotesScope()
	}
	header := out.CursorProjectHeaderProject()
	if header == nil || m.activeProjectByID(header.ID) == nil {
		return route.AllNotesScope()
	}
	return route.ProjectNotesScope(header.ID)
}

// coreCaptureMessage routes base capture to the workspace-owned System Inbox
// and project-focus capture to that focused project. A task Focus wins first
// and captures a direct child of the focus root, which is an explicit
// projection anchor pushed by Enter rather than the cursor. At the nesting cap
// it degrades to plain capture into the focus root's project. Cursor project
// identity is deliberately ignored at base scope, with one exception: a project
// header row names a project explicitly rather than incidentally, so capture
// there lands in it. cursorProjectHeaderProject returns nil on every task row,
// so the invariant holds verbatim everywhere else.
func (m *AppModel) coreCaptureMessage() tea.Msg {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return m.captureComposerMessage(nil, nil)
	}
	var project *models.Project
	if identity, focused := out.FocusRootIdentity(); focused {
		if task, focusProject := m.taskAndProjectForIdentity(identity); task != nil && focusProject != nil {
			if taskAcceptsDeeperChild(task) {
				return taskform.ShowTaskComposerMsg{Preset: m.focusChildComposerPreset(focusProject, task)}
			}
			project = focusProject
		}
	}
	if project == nil {
		switch {
		case out.Project != nil:
			project = out.Project
		case out.IsProjectScoped():
			project = out.ScopedProject()
		}
	}
	if project == nil {
		project = out.CursorProjectHeaderProject()
	}
	if project == nil {
		// Resolve through RuntimeState's canonical workspace-owned Unfiled
		// reference, then prefer the shell's loaded project pointer when it is
		// available so capture never borrows the base cursor's project.
		var unfiled *models.Project
		if m.domain.runtime != nil {
			unfiled = m.domain.runtime.SystemUnfiledProject()
			if unfiled != nil {
				if loaded := m.projectByID(unfiled.ID); loaded != nil {
					unfiled = loaded
				}
			} else if id := m.domain.runtime.Catalog().SystemUnfiledProjectID; id != "" {
				unfiled = m.projectByID(id)
			}
		}
		return m.captureComposerMessage(unfiled, m.availableProjects())
	}
	return m.captureComposerMessage(project, m.availableProjects())
}

func operationTargetLocators(targets operation.ResolvedOperationTargets) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			locators = append(locators, target.Task.Locator())
		}
	}
	return locators
}

func planningMemberLocators(members map[taskref.TaskIdentity]bool) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(members))
	for identity, member := range members {
		if member {
			locators = append(locators, identity.Locator())
		}
	}
	return locators
}

func (m *AppModel) corePlanningOperation(
	targets operation.ResolvedOperationTargets,
	action app.PlanningAction,
	capture *outliner.OutlinerRelocationCapture,
) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	summary := fmt.Sprintf("Updated %d task(s)", len(locators))
	switch action {
	case app.PlanningAddToday:
		summary = fmt.Sprintf("Added %d task(s) to Today", len(locators))
	case app.PlanningRemoveToday:
		summary = fmt.Sprintf("Removed %d task(s) from Today", len(locators))
	case app.PlanningAddNext:
		summary = fmt.Sprintf("Moved %d task(s) to Next", len(locators))
	case app.PlanningRemoveNext:
		summary = fmt.Sprintf("Removed %d task(s) from Next", len(locators))
	case app.PlanningBacklog:
		summary = fmt.Sprintf("Moved %d task(s) to Backlog", len(locators))
	}
	return withOutlinerRelocationCapture(
		m.secondaryFeatures.Bulk.ApplyPlanningLocators(locators, action, summary),
		capture,
	)
}

func (m *AppModel) coreMoveToBacklogSingle(capture *outliner.OutlinerRelocationCapture) tea.Cmd {
	task, project := m.outlinerTarget()
	if task == nil || project == nil {
		return nil
	}
	fresh, _, transactionID, err := m.applyTaskPlanningAction(task.ID, project.ID, app.PlanningBacklog)
	if err != nil {
		m.feedback.err = err
		return status.ErrorCmd("%s", err)
	}
	successMessage := fmt.Sprintf("Moved to Backlog: %s", taskTitleOrFallback(fresh, task))
	return m.finishSynchronousOutlinerRelocation(m.queueToggleResultCmd(), capture, successMessage, transactionID)
}

func (m *AppModel) coreToggleToday(targets operation.ResolvedOperationTargets, capture *outliner.OutlinerRelocationCapture) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	action, err := app.ResolvePlanningToggleAction(
		app.PlanningHorizonToday,
		locators,
		planningMemberLocators(m.todayIDs),
	)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return m.corePlanningOperation(targets, action, capture)
}

func (m *AppModel) coreToggleNext(targets operation.ResolvedOperationTargets, capture *outliner.OutlinerRelocationCapture) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	action, err := app.ResolvePlanningToggleAction(
		app.PlanningHorizonNext,
		locators,
		planningMemberLocators(m.nextOrderIDs),
	)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return m.corePlanningOperation(targets, action, capture)
}

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
	out.AdoptNavigatedCursor(previous)
	return cmd
}

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
	id := operation.OperationCoreScrollCursorTop
	switch anchor {
	case viewScrollAnchorMiddle:
		id = operation.OperationCoreScrollCursorMiddle
	case viewScrollAnchorBottom:
		id = operation.OperationCoreScrollCursorBottom
	case viewScrollAnchorTop:
	}
	out.ViewScrollOffset = out.CursorScrollOffset(id)
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

// coreGotoRelation opens the relationship picker over the cursor task. The hop
// itself belongs to the Outliner, since it is a presentation move the return
// trail has to be able to undo; the shell only names the subject and surfaces a
// refusal the picker cannot open under.
func (m *AppModel) coreGotoRelation(targets operation.ResolvedOperationTargets) operation.OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return operation.OperationHandlerResult{}
	}
	task, project := m.taskAndProjectForTargets(targets)
	if task == nil || project == nil {
		return operation.OperationHandlerResult{}
	}
	if err := out.OpenRelationPicker(task, project); err != nil {
		return operation.OperationHandlerResult{Status: err.Error()}
	}
	return operation.OperationHandlerResult{}
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
			Message: operation.OperationCommandResultMsg{Cmd: m.filters.ProjectFilterReloadCmd(m, status)},
		}
	}
	return operation.OperationHandlerResult{Status: status}
}

func (m *AppModel) coreToggleTheme() error {
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine theme is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	if machine.Theme == "light" {
		machine.Theme = "dark"
	} else {
		machine.Theme = "light"
	}
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save theme: %w", err)
	}
	theme.ApplyTheme(theme.NewThemeConfig(m.domain.runtime.Machine()))
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
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	machine.MenuBarVisible = !machine.MenuBarVisible
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
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
// The reveal is the open menu's own, not a write to the machine-local flag `M`
// writes: an open menu draws the band it hangs from, and closing it returns
// visibility to whatever the user last chose. Showing the bar is still `M`'s
// alone, so a menu opened out of a hidden bar borrows the row and gives it back.
func (m *AppModel) coreOpenMenuBar() error {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return fmt.Errorf("the menu bar belongs to the Outliner")
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
	if m.operations.Registry == nil {
		return nil
	}
	items := m.operations.Registry.HelpItemsForInputStrategy(
		operation.InputStrategySimplified,
		m.operationCommandContext(),
	)
	// This workspace action is menu-only; its name and search vocabulary still
	// come from the descriptor authority, with its actual menu route as the hint.
	if descriptor, ok := m.operations.Registry.Descriptor(operation.OperationCoreSetUpCheckIns); ok && descriptor.Applicable(m.operationCommandContext()) {
		items = append(items, operation.HelpItem{
			Key: "Ctrl+M → Task", Description: descriptor.Label,
			Category: operation.OperationCategoryLabel(descriptor.Category), Aliases: descriptor.Aliases,
		})
	}
	return append(items, help.GrammarHelpItems()...)
}
