package outliner

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// handleKeyMsg processes only the inline controls owned by the Outliner. Root
// operations are dispatched by OperationRegistry before the model sees them.
func (m *OutlinerModel) HandleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A deliberate navigation or edit supersedes a transient message's window.
	m.feedbackBudget = feedbackBudgetAnchor{}
	key := msg.String()

	// The relationship picker is read before everything else: it is a modal
	// overlay drawn over the whole frame, so every key below it would act on rows
	// that are not on screen.
	if m.RelationPickerOpen() {
		return m.handleRelationPickerKey(msg)
	}

	// An open menu is read first: its navigation keys are a gated keymap the
	// footer is advertising, and h/j/k/l mean something else entirely to the
	// cascade below. Unlike the readout it is non-modal — an unrecognized key
	// closes the menu and is re-emitted so the root grammar runs it — so nothing
	// below is unreachable while a menu is up, only one update later.
	if m.MenuBar.Open {
		return m.handleMenuBarKey(msg)
	}

	// The expanded query readout owns input outright while it is open: its peel
	// digits are a gated keymap the footer is advertising, and letting a root key
	// through underneath would run an act the on-screen footer does not name.
	if m.QueryReadout.Expanded {
		return m.HandleQueryReadoutKey(key)
	}

	if m.IsSearchMode() && m.Search.historyPicker != nil {
		return m, m.updateSearchHistoryPicker(msg)
	}
	if m.IsSearchMode() && key == "ctrl+r" {
		return m, m.openSearchHistoryPicker()
	}

	// An open suggestion pane claims Tab, Esc, and its navigation keys before
	// the search cascade below sees them, which is what makes the first Esc close
	// the pane and the next one clear or restore the search.
	if m.IsSearchMode() && m.HandleSearchAutocompleteKey(key) {
		return m, nil
	}

	// Up and Down walk the session's submitted queries, below the pane's claim
	// above so an open pane keeps them for its own cursor. The walk is confined to
	// the input: no root key is bound for it, and the shell routes every key here
	// while search mode owns the input.
	if m.recallSearchHistory(key) {
		return m, nil
	}

	// Handle search mode input first
	if m.IsSearchMode() && key == "esc" {
		if m.IsRangeSelectionActive() {
			m.FreezeRangeSelection()
			return m, nil
		}
		m.ClearSearch()
		return m, nil
	}
	// Submitting is what retains a query, so an abandoned one never joins the
	// ring. This runs before HandleSearchMode consumes the key, which is the only
	// place the submitted text is still readable as input rather than as a query.
	var historyCmd tea.Cmd
	if m.IsSearchMode() && key == "enter" {
		if err := m.recordSearchHistoryEntry(m.SearchInputValue()); err != nil {
			historyCmd = status.ErrorCmd("Search works, but history could not be saved: %v", err)
		}
	}
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		// Typing ends the walk, so the next Up starts from the newest query again
		// rather than from wherever the abandoned walk had reached.
		m.Search.history.resetRecall()
		RebuildOutlinerPreservingCursor(m)
	}); handled {
		// Recompute unconditionally rather than on value change alone: a bare caret
		// move leaves the query identical while putting a different token under it.
		m.RefreshSearchAutocomplete()
		return model, tea.Batch(cmd, historyCmd)
	}

	// Handle quick edit mode input first
	if m.QuickEdit.IsActive() {
		return m.HandleQuickEditMode(msg)
	}

	// Handle confirm dialog before anything else
	if m.Confirm.IsActive() {
		return m.handleConfirmDialog(key)
	}

	return m, nil
}

// handleConfirmDialog processes key input while a confirmation dialog is active.
//
// Task deletion is shell-owned, so this handler only owns the Outliner-local
// project archive confirmation.
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.Confirm.Action()

	// Standard y/n handling
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case projecthub.ProjectArchiveConfirmAction:
				return m, func() tea.Msg { return projecthub.ProjectArchiveConfirmedMsg{} }
			}
		}
		return m, nil
	}
	return m, nil
}

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.Folds.ExpandedNodes) == 0 {
		m.Folds.resetNodes()
		if m.Project != nil {
			m.markAllExpanded(m.Project.ID, m.Project.GetRootTasks())
		} else {
			for _, p := range m.Population.Projects {
				m.markAllExpanded(p.ID, p.GetRootTasks())
			}
		}
	}
}

// markAllExpanded marks all parent tasks as expanded recursively.
func (m *OutlinerModel) markAllExpanded(projectID string, tasks []*models.Task) {
	m.markAllFolds(projectID, tasks, true)
}

// markAllFolds writes one fold state onto every parent task in the subtree and
// reports whether any of them moved. Childless tasks stay absent: they have
// nothing to fold, and an entry for one would only make isExpanded answer a
// question nobody asks.
func (m *OutlinerModel) markAllFolds(projectID string, tasks []*models.Task, expanded bool) bool {
	changed := false
	for _, task := range tasks {
		if len(task.Children) == 0 {
			continue
		}
		identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
		if previous, known := m.Folds.ExpandedNodes[identity]; !known || previous != expanded {
			changed = true
		}
		m.Folds.setNode(identity, expanded)
		if m.markAllFolds(projectID, task.Children, expanded) {
			changed = true
		}
	}
	return changed
}

// anyTaskFoldCollapsed reports whether any task subtree is currently folded. An
// absent key reads as expanded, so only a false entry counts. It is the one
// definition of that question, shared by the direction rule t decides with and
// by the mutation below that has to know whether unfolding changes anything.
func (m *OutlinerModel) AnyTaskFoldCollapsed() bool {
	if m == nil {
		return false
	}
	return slices.Contains(slices.Collect(maps.Values(m.Folds.ExpandedNodes)), false)
}

// setAllTaskFolds drives every task subtree to one fold state and reports
// whether that changed anything, so the caller can refuse instead of rebuilding
// rows nothing moved. It owns expandedNodes alone: sections answer to s through
// collapsedSections, and checklists default collapsed under their own map.
//
// The walk covers every loaded project rather than the active projection,
// matching markAllExpanded. Folds are project-qualified and outlive a mode
// change, so folding only what a filter happens to show would leave the modes
// beside it disagreeing about the same task.
func (m *OutlinerModel) setAllTaskFolds(expanded bool) bool {
	if m == nil {
		return false
	}
	if expanded {
		// An empty map is the canonical all-expanded state, so clearing beats
		// writing true over every node. Only a false entry hides anything, so
		// only a false entry makes this a change.
		if !m.AnyTaskFoldCollapsed() {
			return false
		}
		m.Folds.resetNodes()
		return true
	}

	if m.Project != nil {
		return m.markAllFolds(m.Project.ID, m.Project.GetRootTasks(), false)
	}
	changed := false
	for _, project := range m.Population.Projects {
		if m.markAllFolds(project.ID, project.GetRootTasks(), false) {
			changed = true
		}
	}
	return changed
}

// parentIDOf returns the parent task ID for t, or "" if t is a root.
func parentIDOf(t *models.Task) string {
	if t == nil || t.ParentID == nil {
		return ""
	}
	return *t.ParentID
}

// visibleSiblings returns the visible sibling rows in m.rows around cursorIdx —
// tasks sharing the same parent ID and ProjectID as the cursor row, gathered
// across a window bounded by the cursor row's own group. Non-sibling task rows
// inside the window (e.g. expanded descendants of a sibling) are skipped rather
// than treated as hard stops, so J/K reorders and move-to-top/bottom can target
// siblings on either side of an expanded parent. Returns nil when cursorIdx is
// out of range or points at a header/separator/nil task row. The group boundary
// confines reorders to a single section bucket (and to a single project in
// cross-project mode) without dereferencing potentially stale cloned-parent
// Children slices, and without depending on a header row being drawn between
// two groups. Under the default priority order the window is also one priority
// band, since that is the only run manual order still arranges.
func (m *OutlinerModel) visibleSiblings(cursorIdx int) []*models.Task {
	if cursorIdx < 0 || cursorIdx >= len(m.Rows) {
		return nil
	}
	cursorRow := m.Rows[cursorIdx]
	if !cursorRow.IsTaskRow() || cursorRow.Task == nil {
		return nil
	}
	cursorParentID := parentIDOf(cursorRow.Task)
	cursorProjectID := cursorRow.ProjectID
	priorityBanded := m.sortsByPriorityBand()

	// isBoundary reports whether a row terminates the reorder window. Anything
	// that is not a task row ends it — headers, separators, checklist items — and
	// so does a task row belonging to another group, which is what keeps a
	// reorder inside one section bucket and one project.
	isBoundary := func(row OutlinerRow) bool {
		if !row.IsTaskRow() {
			return true
		}
		return row.Group != cursorRow.Group
	}

	// isSibling reports whether a task row shares the cursor's parent (and
	// project). Non-matching task rows inside the window are skipped, not stops.
	isSibling := func(row OutlinerRow) bool {
		if !row.IsTaskRow() || row.Task == nil {
			return false
		}
		if parentIDOf(row.Task) != cursorParentID {
			return false
		}
		if row.ProjectID != cursorProjectID {
			return false
		}
		if priorityBanded && comparePriorityBand(row.Task, cursorRow.Task) != 0 {
			return false
		}
		return true
	}

	var leftSide []*models.Task
	for i := cursorIdx - 1; i >= 0; i-- {
		if isBoundary(m.Rows[i]) {
			break
		}
		if isSibling(m.Rows[i]) {
			leftSide = append(leftSide, m.Rows[i].Task)
		}
	}
	// Reverse leftSide so siblings appear in row order.
	for i, j := 0, len(leftSide)-1; i < j; i, j = i+1, j-1 {
		leftSide[i], leftSide[j] = leftSide[j], leftSide[i]
	}

	siblings := append(leftSide, cursorRow.Task)
	for i := cursorIdx + 1; i < len(m.Rows); i++ {
		if isBoundary(m.Rows[i]) {
			break
		}
		if isSibling(m.Rows[i]) {
			siblings = append(siblings, m.Rows[i].Task)
		}
	}
	return siblings
}

// moveCursorToTask positions the cursor on the row matching the given task ID.
// Falls back to clamping if the task is not found in the current rows.
func (m *OutlinerModel) MoveCursorToTask(taskID string) {
	for i, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			m.Cursor = i
			return
		}
	}
	m.ClampCursor()
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) HandleQuickEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.QuickEdit.SetContext(m.QuickEdit.GetProject(), m.quickEditProjects())
	transition, result, cmd := m.QuickEdit.Handle(msg)
	if transition != grammar.QuickEditCommit || result == nil {
		return m, cmd
	}
	project := m.QuickEdit.GetProject()
	task := m.QuickEdit.GetTask()
	if project == nil || task == nil || project.GetTaskByID(task.ID) != task {
		m.QuickEdit.RejectCommit("task edit context is unavailable")
		return m, nil
	}
	source := taskref.For(task, project)
	relocation := m.CaptureOutlinerRelocation([]taskref.TaskIdentity{source})
	m.QuickEdit.BeginCommit()
	return m, m.taskItem.EditTaskLine(m.TaskLineEditRequest(source, result), relocation)
}

func (m *OutlinerModel) quickEditProjects() []*models.Project {
	if m.Project != nil {
		return []*models.Project{m.Project}
	}
	return m.Population.Projects
}

func TaskLineEditRequest(source taskref.TaskIdentity, result *grammar.QuickEditResult) app.TaskLineEditRequest {
	req := app.TaskLineEditRequest{
		Source:               source.Locator(),
		Title:                result.Title,
		DueSet:               result.DueDateSet,
		DeferSet:             result.DeferSet,
		TagsToAdd:            append([]string(nil), result.TagsToAdd...),
		TagsToRemove:         append([]string(nil), result.TagsToRemove...),
		DestinationProjectID: source.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
	}
	if result.PrioritySet {
		priority := result.Priority
		req.Priority = &priority
	}
	if result.ContextSet {
		context := result.ContextValue
		if result.ContextClear {
			context = ""
		}
		req.Context = &context
	}
	if result.DueDateSet && !result.DueDateClear {
		req.Due = result.DueDate
	}
	if result.DeferSet && !result.DeferClear {
		req.Defer = result.DeferUntil
	}
	if result.DestinationSet {
		destination := result.Destination
		req.Destination = &destination
	}
	if result.PinnedSet {
		pinned := result.Pinned
		req.Pinned = &pinned
	}
	if result.ReviewSet {
		review := result.Review
		req.Review = &review
	}
	if result.TargetProject != nil {
		req.DestinationProjectID = result.TargetProject.ID
	}
	switch result.ParentMode {
	case grammar.QuickEditParentPreserve:
		// Preserve is already the request default.
	case grammar.QuickEditParentRoot:
		req.Parent.Mode = app.TaskLineParentRoot
	case grammar.QuickEditParentSet:
		req.Parent.Mode = app.TaskLineParentSet
		if result.Parent != nil {
			req.Parent.Parent = app.TaskLocator{
				ProjectID: req.DestinationProjectID,
				TaskID:    result.Parent.ID,
			}
		}
	}
	// A dependency locator is fully qualified at parse time and is independent of
	// where the edited task lands, so a cross-project move carries it unchanged.
	req.Dependencies = result.DependencyPatch()
	// Children resolve in the effective project, which is the destination
	// whenever the edit also crosses a boundary.
	for _, child := range result.Children {
		req.Children = append(req.Children, app.TaskLocator{
			ProjectID: req.DestinationProjectID,
			TaskID:    child.ID,
		})
	}
	return req
}

// TaskLineEditRequest binds every live inline edit to an explicit planning
// builder instead of inheriting whichever default a prior service call left
// installed. Cross-project edits carry a complete planning-move delta, so they
// use the structural builder that reconciles against the newest on-disk state.
func (m *OutlinerModel) TaskLineEditRequest(source taskref.TaskIdentity, result *grammar.QuickEditResult) app.TaskLineEditRequest {
	req := TaskLineEditRequest(source, result)
	req.PlanningArtifacts = coordinator.PlanningArtifacts(m.Runtime, m.configManager)
	if req.DestinationProjectID != source.ProjectID {
		req.PlanningArtifacts = coordinator.StructuralPlanningArtifacts(m.Runtime, m.configManager)
	}
	return req
}

// escSteps defines the Esc dismissal cascade for Outliner, narrowest to widest:
// Peek → active range → frozen selection → search query → active lens →
// Focus stack → cross-project Focus stack → section zoom → project scope.
func (m *OutlinerModel) EscSteps() []listsurface.EscStep {
	return []listsurface.EscStep{
		// This step reads the intent rather than the drawn dock, because it is the
		// intent it puts down: a frame with no room for a three-row dock still holds
		// PeekMode, and a cascade gated on what is painted would leave that flag with
		// no key to clear it, so the dock would spring back the moment the terminal
		// grew. What the frame decides is whether the dock is drawn, never whether
		// Esc can close it.
		{Active: func() bool { return m != nil && m.PeekMode }, Action: escActionClosePeek, Clear: func() {
			m.PeekMode = false
		}},
		{Active: func() bool { return m.IsRangeSelectionActive() }, Action: escActionFreezeRange, Clear: func() {
			m.FreezeRangeSelection()
		}},
		{Active: func() bool { return m.MultiSelectMode }, Action: escActionClearSelection, Clear: func() {
			m.ToggleMultiSelectMode()
		}},
		// One relationship hop unwinds after the selection and before the search,
		// because a hop replaced the whole projection: the query, lens, focus, zoom,
		// and scope steps below it are all axes the return frame is about to restore
		// wholesale, and stepping through them first would unwind the destination's
		// state on the way back to the origin's.
		{Active: func() bool { return m.HasRelationTrail() }, Action: escActionReturnToOrigin, Clear: func() {
			m.PopRelationReturn()
		}},
		// The search step is the one whose act depends on what it is standing over:
		// a query typed under a lens put that lens down, so this press puts it back
		// instead of clearing anything. SearchExitAction is the one answer the
		// footer, the guidance, and the empty-state sentence all read.
		{Active: func() bool { return m.HasSearchProjection() }, Action: m.SearchExitAction(), Clear: func() {
			m.ClearSearch()
		}},
		// A lens is one atomic context over the mode, facets, sort, query, and
		// population, so it unwinds before the narrowings taken inside it: leaving
		// it restores the focus, zoom, and scope the user had anyway, and stepping
		// through those first would unwind state the lens is about to replace.
		{Active: func() bool { return m.HasActiveLens() }, Action: escActionLeaveLens, Clear: func() {
			m.RestoreLensBaseline()
		}},
		{Active: func() bool { return len(m.Focus.Stack) > 0 }, Action: escActionLeaveFocus, Clear: func() {
			m.PopFocusLevel()
		}},
		{Active: func() bool { return len(m.Focus.CrossStack) > 0 }, Action: escActionLeaveFocus, Clear: func() {
			m.PopFocusLevel()
		}},
		// A zoom narrows one mode, so it unwinds after task focus and before the
		// project scope that owns the mode.
		{Active: func() bool { return m.Focus.Zoom.Active() }, Action: escActionUnzoom, Clear: func() {
			m.ExitSectionZoom()
		}},
		// Task focus and section zoom both unwind before project scope: Esc pops
		// one scope frame, restoring the previous scope's mode, filter, cursor,
		// scroll, and zoom.
		{Active: func() bool { return m.IsProjectScoped() }, Action: escActionLeaveScope, Clear: func() {
			m.PopProjectScope()
		}},
	}
}

// The cascade's vocabulary. Each is the imperative half of one hint, so the
// footer never spells an act and the cascade never carries an unnamed step.
// `leave lens` and `restore lens` are deliberately not one word: leaving puts an
// active lens down, restoring brings a suspended one back, and a single spelling
// for both would name opposite acts on the same key.
const (
	escActionClosePeek      = "close Peek"
	escActionFreezeRange    = "freeze range"
	escActionClearSelection = "clear selection"
	escActionReturnToOrigin = "return to origin"
	escActionLeaveLens      = "leave lens"
	escActionLeaveFocus     = "leave Focus"
	escActionUnzoom         = "unzoom"
	escActionLeaveScope     = "all projects"
)

// EscAction names what the next Esc press does to the standing state, or "" when
// the cascade is empty. Every Outliner guidance profile spells its Esc entry
// through this rather than through a literal of its own, which is what keeps a
// footer from promising an act the cascade would not run.
//
// It answers for the cascade alone. The modal claims above it — the relationship
// picker, the menu, the readout, the confirm, the inline editor, the search
// input, and the suggestion pane — each own the key outright before the cascade
// sees it, so each names its own act in its own profile.
func (m *OutlinerModel) EscAction() string {
	if m == nil {
		return ""
	}
	return listsurface.NextEscAction(m.EscSteps())
}

// EscHint is that action as a complete footer entry, empty when nothing stands.
func (m *OutlinerModel) EscHint() string {
	action := m.EscAction()
	if action == "" {
		return ""
	}
	return "Esc " + action
}

func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() ||
		m.SearchShouldCaptureEsc() ||
		m.MultiSelectMode ||
		m.QuickEdit.IsActive() ||
		m.QueryReadout.Expanded ||
		// The picker is drawn over the whole frame, so it is the innermost thing Esc
		// can put down and claims the key ahead of the menu below it.
		m.RelationPickerOpen() ||
		// An open menu is the innermost thing Esc can put down, so it has to claim
		// the key before the shell's back cascade spends it unwinding a scope the
		// user cannot even see the rows of.
		m.MenuBar.Open ||
		listsurface.AnyEscStepActive(m.EscSteps())
}
func (m *OutlinerModel) HasTextInput() bool {
	return m.DefaultHasTextInput() || m.QuickEdit.IsActive()
}

type outlinerEmptyCause uint8

const (
	outlinerEmptyNoTasks outlinerEmptyCause = iota
	outlinerEmptyNoProjects
	outlinerEmptyMode
	outlinerEmptyFiltered
	outlinerEmptyProjectFiltered
	outlinerEmptySearch
	OutlinerEmptySearchFiltered
	outlinerEmptySearchInvalid
	outlinerEmptyLens
	outlinerEmptyFocus
	OutlinerEmptySection
)

func (m *OutlinerModel) HandledKeys() []string {
	if m.OperationRegistry == nil {
		return nil
	}
	return m.OperationRegistry.HandledKeysForInputStrategy(operation.InputStrategySimplified)
}

// The Outliner has no GetHelpItems of its own: `?` is a core input operation, so
// AppModel.coreInputHelpItems is the one projection that answers it. A second
// copy here would be unreachable and would drift.

// ContextualGuidance selects one Outliner-specific profile before falling back
// to the ranked operation projection for an ordinary task row. Interaction and
// causal recovery controls are required so narrow terminals retain the action
// that exits the current state.
func (m *OutlinerModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m == nil {
		return chrome.GuidanceSpec{}, false
	}
	switch {
	case m.RelationPickerOpen():
		// First because the picker is drawn over the whole frame: every profile below
		// describes keys the rows it hid would have answered.
		return m.relationPickerGuidance(), true
	case m.MenuBar.Open:
		// First for the same reason the readout is first below it: while a menu is
		// open its own keymap is what the four navigation keys mean, and every
		// profile after this one would name acts they are not doing right now.
		return m.menuBarGuidance(), true
	case m.QueryReadout.Expanded:
		// First because the readout owns input outright: every other profile below
		// describes keys the stack is currently refusing.
		return m.queryReadoutGuidance(), true
	case m.Confirm.IsActive():
		return chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("y confirm", "n/Esc cancel"), Rows: 2}, true
	case m.QuickEdit.IsActive():
		hints := chrome.RequiredGuidanceHints("Enter save", "Esc cancel")
		if !m.QuickEdit.IsRenameOnly() {
			hints = append(hints, chrome.OptionalGuidanceHints(10, quickEditGrammarGuidance()...)...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 2}, true
	case m.IsSearchMode() && m.Search.historyPicker != nil:
		return chrome.RequiredExitGuidanceSpec("Enter use query", "Esc return to search", "↑/↓ select", "Ctrl+X forget query", "Type to filter history"), true
	case m.IsSearchMode() && m.Search.Autocomplete.Active():
		// Only while the pane is open: an Esc here closes suggestions rather than
		// the search, and saying so is the whole point of the swap.
		return m.searchAutocompleteGuidance(), true
	case m.IsSearchMode():
		hints := chrome.RequiredGuidanceHints("Enter apply search", m.SearchExitHint())
		if m.hasSearchHistory() {
			// Only once something has been submitted: an empty ring would advertise
			// a walk that does nothing on its first press.
			hints = append(hints, chrome.OptionalGuidanceHints(10, "↑/↓ recall query", "Ctrl+R search history")...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 2}, true
	// A focused projection with nothing in it still carries its project header,
	// so emptiness is asked of the content rather than of the row count. The
	// explanation and the way out of it are what the user needs there; the
	// header's own actions stay in `?` help and its action sheet.
	//
	// An open dock is the exception: it is the innermost thing on screen and the
	// one Esc puts down first, so naming the empty state's recovery key here would
	// promise an act the first press is not doing. Closing the dock restores this
	// branch and its cause unchanged.
	case !m.HasContentRows() && !m.PeekDockVisible():
		return m.emptyGuidance(), true
	case m.MultiSelectMode:
		return m.multiSelectGuidance(), true
	case m.PeekDockVisible():
		peekKey := m.peekOperationKey(operation.OperationCorePeek, peekDisplayKey)
		// One entry for two keys that do one thing. Peek is the cascade's first
		// step, so the act is read off the cascade rather than spelled here, and
		// the columns a second copy of the same sentence used to spend go to the
		// scrolling offers below it.
		hints := chrome.RequiredGuidanceHints(peekKey + "/" + m.EscHint())
		sizeAction := "expand Peek"
		if m.PeekExpanded {
			sizeAction = "compact Peek"
		}
		hints = append(hints, chrome.NewGuidanceHint(m.peekOperationKey(operation.OperationCorePeekSize, "B")+" "+sizeAction, 1, true))
		// The four scrolling keys are offered only in the direction the document
		// can actually move: a detail that fits its budget advertises no movement
		// at all. Their descriptors are applicable on a wider fact — a visible
		// Peek — because consuming a key at the end of a document is what keeps the
		// press off the list cursor underneath, and an inert offer is not the same
		// thing as an inert key.
		if scrolling := m.peekScrollHints(); len(scrolling) > 0 {
			hints = append(hints, scrolling...)
		}
		return chrome.GuidanceSpec{
			Hints: append(hints,
				chrome.OptionalGuidanceHints(10, "Space actions", "e edit", "? help")...),
			Rows: 2,
			// The paging hints are what wrap this footer, and the entry they push
			// onto a second row alone is not worth the row.
			DropOrphan: true,
		}, true
	case m.HasSearchProjection():
		return chrome.GuidanceSpec{
			Hints: append(chrome.RequiredGuidanceHints(m.EscHint()),
				chrome.OptionalGuidanceHints(10, "/ edit search", "a capture", "P projects", "? help")...),
			Rows: 1,
		}, true
	case m.HasActiveLens():
		// One press leaves the whole saved projection, so it is the required hint
		// here exactly as clearing search is above. `/` is named beside it because
		// under a lens that key suspends the lens for one scope-wide search rather
		// than narrowing it further, which nothing else on screen says.
		return chrome.GuidanceSpec{
			Hints: append(chrome.RequiredGuidanceHints(m.EscHint()),
				chrome.OptionalGuidanceHints(10, `/ search scope`, `\ lenses`, "a capture", "? help")...),
			Rows: 1,
		}, true
	case m.cursorNamesHeader():
		return m.headerGuidance(), true
	case m.OperationRegistry == nil:
		return chrome.GuidanceSpec{}, false
	}

	hints := m.OperationRegistry.GuidanceHintsForInputStrategy(
		operation.InputStrategySimplified,
		m.OperationCommandContext(),
	)
	// A Focus, a zoom, a scope, and a relationship hop all leave the ordinary row
	// footer standing, and none of them is a key the cursor names: the strategy's
	// ranks cannot answer what Esc does, because the answer is what is standing
	// rather than what is under the cursor. The cascade is asked instead, and the
	// entry is required so a narrow terminal keeps the way out of the state it is
	// in. An empty cascade adds nothing, so the browse footer stays as quiet as it
	// was.
	if esc := m.EscHint(); esc != "" {
		hints = append(hints, chrome.NewGuidanceHint(esc, 0, true))
	}
	if len(hints) == 0 {
		return chrome.GuidanceSpec{}, false
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}, true
}

// quickEditFooterEntries is the inline editor's footer spelling of each field
// it teaches: a short positive token and a plain noun that never repeats it.
// Clears and negations are absent on purpose; the `?` syntax guide states
// them, and a footer naming both halves of every field restated itself.
var quickEditFooterEntries = map[textgrammar.Field]string{
	textgrammar.FieldPriority:    "!1-3 priority",
	textgrammar.FieldTag:         "#tag tag",
	textgrammar.FieldContext:     "##ctx context",
	textgrammar.FieldDue:         "@date due",
	textgrammar.FieldDefer:       "@@date defer",
	textgrammar.FieldDestination: "$state status",
	textgrammar.FieldPinned:      "*pin pin",
	textgrammar.FieldReview:      "=review review",
	textgrammar.FieldProject:     "%slug project",
	textgrammar.FieldParent:      ">task parent",
	textgrammar.FieldChild:       "<task adopt",
	textgrammar.FieldDependency:  "~task depends",
}

// quickEditGrammarGuidance lists one entry per field the edit surface accepts,
// in teach order, so the footer reads in the order the syntax guide does and
// the two-row packer sheds from the end of what a user meets last.
func quickEditGrammarGuidance() []string {
	definitions := textgrammar.GuideDefinitions(textgrammar.SurfaceEdit)
	entries := make([]string, 0, len(quickEditFooterEntries))
	for _, definition := range definitions {
		if entry, ok := quickEditFooterEntries[definition.Field]; ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (m *OutlinerModel) multiSelectGuidance() chrome.GuidanceSpec {
	rangeAction := "V range"
	if m.IsRangeSelectionActive() {
		rangeAction = "V freeze range"
	}
	hints := []chrome.GuidanceHint{
		chrome.NewGuidanceHint("Space actions", 1, m.SelectedCount() > 0),
		chrome.NewGuidanceHint("v toggle", 2, false),
		chrome.NewGuidanceHint(rangeAction, 3, false),
		chrome.NewGuidanceHint("Ctrl+A select visible", 4, false),
		// A live range is its own cascade step above the selection, so the key
		// freezes the range here and clears the selection only once it is frozen.
		// The hint is read off the cascade for exactly that reason: spelling
		// "clear selection" at both moments would name an act one of them refuses.
		chrome.NewGuidanceHint(m.EscHint(), 0, true),
		chrome.NewGuidanceHint("? help", 10, false),
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) cursorNamesHeader() bool {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	return m.Rows[m.Cursor].IsHeaderRow()
}

// headerGuidance names only the keys the header under the cursor can act on. On
// the zoomed section header three of them cannot: Enter re-enters the zoom it is
// already in, and the zoom overrides that section's fold, so h/l and s refuse
// rather than write one. Esc is the act that remains, so it takes the fold
// hint's rank and is required.
//
// The two projected hints are re-ranked to 1 rather than kept at the rank the
// strategy declares. That rank orders them among a task row's many keys, where
// they are minor; on a header, opening what the row names is the act the row
// exists for, so it must outlive the mode and search chrome when the terminal
// narrows.
func (m *OutlinerModel) headerGuidance() chrome.GuidanceSpec {
	row := m.Rows[m.Cursor]
	zoomed := m.IsZoomedSectionRow(row)
	hints := make([]chrome.GuidanceHint, 0, 8)
	if m.OperationRegistry != nil {
		for _, hint := range m.OperationRegistry.GuidanceHintsForInputStrategy(
			operation.InputStrategySimplified,
			m.OperationCommandContext(),
		) {
			if (hint.Key == "Enter" && !zoomed) || hint.Key == peekDisplayKey {
				hint.Priority = 1
				hints = append(hints, hint)
			}
		}
	}
	escHint := m.EscHint()
	switch {
	case row.IsFocusedProjectHeader:
		// The one header with no fold advertises no fold key. Everything below it
		// is still foldable on its own row, so the outline loses nothing by this
		// row staying silent about h and l.
	case zoomed:
		// The way out takes the fold hint's place rather than the tail every other
		// header gives it, and names the cascade's own act: a zoom reached from
		// inside a Focus or a lens is left by those first, and "unzoom" would have
		// promised a press that does something else.
		hints = append(hints, chrome.NewGuidanceHint(escHint, 1, true))
		escHint = ""
	case m.IsSectionRowCollapsed(row):
		hints = append(hints, chrome.NewGuidanceHint("l/Right expand", 1, false))
	default:
		hints = append(hints, chrome.NewGuidanceHint("h/Left collapse", 1, false))
	}
	if row.IsProjectHeader {
		// All four sit at rank 2 and the stable sort keeps them in this order, so
		// the narrowest header footer that carries a project act carries the menu
		// first: it still reaches Archive and Delete, which no key beside it does.
		// The three keys that left the menu follow it, each labeled as the strategy
		// labels it.
		hints = append(hints,
			chrome.NewGuidanceHint("Space actions", 2, false),
			chrome.NewGuidanceHint("e project", 2, false),
			chrome.NewGuidanceHint("# project tags", 2, false),
			chrome.NewGuidanceHint("* project pin", 2, false),
		)
	} else if !zoomed {
		hints = append(hints, chrome.NewGuidanceHint("s sections", 2, false))
	}
	// A header row under a Focus, a scope, or a relationship hop owes the way out
	// as much as a task row does, and the zoomed header has already spent it above.
	if escHint != "" {
		hints = append(hints, chrome.NewGuidanceHint(escHint, 0, true))
	}
	hints = append(hints, chrome.OptionalGuidanceHints(3, "Tab next mode", "P projects", "/ search", "? help")...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) emptyGuidance() chrome.GuidanceSpec {
	var required string
	cause := m.ClassifyEmptyCause()
	switch cause {
	case outlinerEmptyNoProjects:
		required = "P projects"
	case outlinerEmptyMode:
		if _, hint := m.emptyModeNextStep(m.PrimaryMode); hint != "" {
			required = hint
		} else {
			required = "Tab next mode"
		}
	case outlinerEmptyFiltered:
		required = "fx clear filters"
	case outlinerEmptyProjectFiltered:
		required = "Fx clear project filters"
	case outlinerEmptySearch, outlinerEmptySearchInvalid:
		required = m.EscHint()
	case OutlinerEmptySearchFiltered:
		// The query is what the user is holding, so widening the facets is the act
		// that keeps it; leaving search is named beside it rather than in its place.
		required = "fx clear filters"
	case outlinerEmptyLens, outlinerEmptyFocus, OutlinerEmptySection:
		// All three are cascade steps, so all three name the press the cascade
		// would actually run: a lens standing over a Focus is left first, and a
		// profile spelling its own recovery here would say otherwise.
		required = m.EscHint()
	default:
		required = "a capture"
	}
	optional := []string{"P projects", "? help"}
	if strings.HasPrefix(required, "P ") {
		optional = []string{"? help"}
	}
	if cause == OutlinerEmptySearchFiltered {
		optional = append([]string{m.EscHint()}, optional...)
	}
	hints := append(chrome.RequiredGuidanceHints(required), chrome.OptionalGuidanceHints(10, optional...)...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

func (m *OutlinerModel) ClassifyEmptyCause() outlinerEmptyCause {
	if m.HasSearchProjection() && m.SearchQuery() != "" {
		if m.Search.Gate.Invalid() {
			return outlinerEmptySearchInvalid
		}
		// Facets are an AND gate applied before scoring, so the plain search
		// message would claim the query matched nothing when a facet is what hid
		// the result. Naming both exits is the difference.
		if query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
			return OutlinerEmptySearchFiltered
		}
		return outlinerEmptySearch
	}
	if m.Project == nil && len(m.Population.Projects) == 0 {
		if query.HasActiveProjectFilters(&m.ProjectFilter) {
			return outlinerEmptyProjectFiltered
		}
		return outlinerEmptyNoProjects
	}
	// A lens owns the mode, the whole narrowing, the sort, and the scope all at
	// once, so naming any one of them would send the user to a key that cannot
	// widen the list. It outranks zoom and focus for the same reason
	// the Esc cascade unwinds it first.
	if m.HasActiveLens() {
		return outlinerEmptyLens
	}
	// A zoom outranks the filter and focus causes below it: whatever else
	// narrowed the list, Esc is the press that widens it back, and naming a
	// filter would send the user to a key that leaves the section empty.
	if m.Focus.Zoom.Active() {
		return OutlinerEmptySection
	}
	if query.HasActiveCommonFilters(&m.Filter.CommonFilterState) {
		return outlinerEmptyFiltered
	}
	if m.Focus.Active() {
		return outlinerEmptyFocus
	}
	if m.PrimaryMode != chrome.ModeAll {
		return outlinerEmptyMode
	}
	return outlinerEmptyNoTasks
}

// decoratePendingChordHint adds effective filter state to the existing live
// registry entry. It never creates or reorders a key, so which-key and dispatch
// continue sharing the same metadata and applicability checks.
func (m *OutlinerModel) DecoratePendingChordHint(path []string, hint operation.ChordHint) operation.ChordHint {
	if m == nil || len(path) != 2 {
		return hint
	}
	switch path[0] {
	case "f":
		filter := &m.Filter.CommonFilterState
		switch path[1] {
		case "!":
			if filter.Priority > 0 {
				hint.Label = "priority:" + strings.ToLower(models.PriorityName(filter.Priority)) + "+"
			}
		case "w":
			if filter.HasStateFilter() {
				hint.Label = fmt.Sprintf("states:%d", len(filter.States.Values()))
			}
		case "a":
			if filter.HasCompletionAgeFilter() {
				hint.Label = "completed:all"
				if days := filter.CompletionAge.Days(); days > 0 {
					hint.Label = fmt.Sprintf("completed:%dd+", days)
				}
			}
		case "c":
			// The no-context row is one more admitted alternative, so it counts
			// alongside the names rather than as a facet of its own.
			count := len(filter.Contexts)
			if filter.NoContext {
				count++
			}
			if count > 0 {
				hint.Label = fmt.Sprintf("contexts:%d", count)
			}
		case "%":
			if len(filter.Projects) > 0 {
				hint.Label = fmt.Sprintf("projects:%d", len(filter.Projects))
			}
		case "@":
			// The label is the token the picker's row writes, so the hint, the chip,
			// and a typed query name this facet with the same characters.
			active := query.DueFilterClauses(filter.DueDate)
			if filter.Deferred {
				active = query.FilterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, "")
			}
			if len(active) > 0 {
				hint.Label = strings.Join(query.FilterClauseParts(active), " ")
			}
		case "#":
			if len(filter.Tags) > 0 {
				hint.Label = fmt.Sprintf("tags:%d", len(filter.Tags))
			}
		case "=":
			if filter.Review {
				hint.Label = "review:on"
			}
		case "h":
			if filter.Hierarchy != query.HierarchyAll {
				hint.Label = "hierarchy:" + HierarchyFilterLabel(filter.Hierarchy)
			}
		case "s":
			if filter.Stale {
				hint.Label = "stale:on"
			}
		case "x":
			if count := query.CommonFilterFacetCount(filter); count > 0 {
				hint.Label = fmt.Sprintf("clear:%d", count)
			}
		}
	case "F":
		return m.decorateProjectFilterChordHint(path[1], hint)
	}
	return hint
}

// decorateProjectFilterChordHint decorates one F continuation. Each population
// key only reports the mode it owns, so the other key of the same axis keeps
// its static affordance instead of both rows naming one state.
func (m *OutlinerModel) decorateProjectFilterChordHint(key string, hint operation.ChordHint) operation.ChordHint {
	filter := &m.ProjectFilter
	switch key {
	case "#":
		if selected := query.ProjectFilterSelection(filter); len(selected) > 0 {
			hint.Label = fmt.Sprintf("project tags:%d", len(selected))
		}
	case "p":
		if filter.ParkedMode == query.ProjectPopulationInclude {
			hint.Label = "parked:on"
		}
	case "P":
		if filter.ParkedMode == query.ProjectPopulationOnly {
			hint.Label = "parked:only"
		}
	case "a":
		if filter.ArchivedMode == query.ProjectPopulationInclude {
			hint.Label = "archived:on"
		}
	case "A":
		if filter.ArchivedMode == query.ProjectPopulationOnly {
			hint.Label = "archived:only"
		}
	case "x":
		if count := query.ProjectFilterFacetCount(filter); count > 0 {
			hint.Label = fmt.Sprintf("clear:%d", count)
		}
	}
	return hint
}
