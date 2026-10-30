package outliner

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/status"
)

// handleKeyMsg processes only the inline controls owned by the Outliner. Root
// operations are dispatched by OperationRegistry before the model sees them.
func (m *OutlinerModel) HandleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A deliberate navigation or edit supersedes a transient message's window.
	m.feedbackBudget = feedbackBudgetAnchor{}
	key := msg.String()
	if m.PeekFullscreen {
		return m, m.handleFullscreenPeekKey(key)
	}

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
		return m, m.takeSearchCommand()
	}

	// Up and Down walk the session's submitted queries, below the pane's claim
	// above so an open pane keeps them for its own cursor. The walk is confined to
	// the input: no root key is bound for it, and the shell routes every key here
	// while search mode owns the input.
	if m.recallSearchHistory(key) {
		return m, m.takeSearchCommand()
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
	var searchCmd tea.Cmd
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		// Typing ends the walk, so the next Up starts from the newest query again
		// rather than from wherever the abandoned walk had reached.
		m.Search.history.resetRecall()
		searchCmd = m.rebuildLiveSearch()
	}); handled {
		// Recompute unconditionally rather than on value change alone: a bare caret
		// move leaves the query identical while putting a different token under it.
		m.RefreshSearchAutocomplete()
		return model, tea.Batch(cmd, historyCmd, searchCmd)
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

// escSteps defines the Esc dismissal cascade for Outliner, narrowest to widest:
// Peek → active range → frozen selection → search query → active lens →
// Focus stack → cross-project Focus stack → section zoom → project scope.
func (m *OutlinerModel) EscSteps() []listsurface.EscStep {
	return []listsurface.EscStep{
		{Active: func() bool { return m != nil && m.PeekFullscreen }, Action: "return to list", Clear: m.closeFullscreenPeek},
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

func (m *OutlinerModel) HandledKeys() []string {
	if m.OperationRegistry == nil {
		return nil
	}
	return m.OperationRegistry.HandledKeysForInputStrategy(operation.InputStrategySimplified)
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
