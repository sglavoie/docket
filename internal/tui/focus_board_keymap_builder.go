package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildFocusBoardKeyMap constructs the declarative KeyMap for FocusBoardModel.
func buildFocusBoardKeyMap(m *FocusBoardModel) *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasItem := func() bool { return m.getRowAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.skipGroupHeaderForward()
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
				m.skipGroupHeaderBackward()
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filtered), -HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filtered), HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)

	// --- Goto group: g prefix (gg = top, gp = go to project) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return FocusBoardSelectMsg{Task: task, Project: project} }
		},
	})...)

	// --- f prefix: fold/collapse operations + preview ---
	isTodayTab := func() bool { return m.tabs.Active == boardTabToday }
	setDepth := func(depth int, label string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			m.childDepthLimit = depth
			clear(m.parentOverrides)
			m.rebuildTodayChildren()
			m.setStatus(label)
			return m, nil
		}
	}
	entries = append(entries, keymap.Prefix("f", "Folds", "View", []keymap.Entry{
		keymap.Bind("a", "Toggle fold", "Folds", func() (tea.Model, tea.Cmd) {
			m.toggleFoldAtCursor()
			return m, nil
		}, keymap.WithGate(isTodayTab)),
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		}),
		keymap.Bind("1", "Collapse to root only", "Folds", setDepth(0, "Fold: root items only"), keymap.WithGate(isTodayTab)),
		keymap.Bind("2", "Expand to depth 2", "Folds", setDepth(1, "Fold: depth 2"), keymap.WithGate(isTodayTab)),
		keymap.Bind("3", "Expand to depth 3", "Folds", setDepth(2, "Fold: depth 3"), keymap.WithGate(isTodayTab)),
	}))

	// --- Standard task action groups (Status, Priority, DueDate, Snooze, Move,
	//     LinkedTask, TaskPrefix, Stage, Blocker, CommonTaskAction) ---
	entries = append(entries, buildStandardTaskActionEntries(m.buildTaskKeymapDeps(hasItem, stages))...)

	// Override RenameTask in TaskPrefixGroup is handled by the builder using v.getTaskRef();
	// FocusBoard needs inline edit mode instead. We patch it via a separate direct binding.
	// (The builder's RenameTask sends an EditTaskMsg; the focus board overrides via "E" binding below.)

	// --- Filter group: "F" prefix, no gate (always active) ---
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Ft", key)
			}
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Fs", key)
			}
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Fd", key)
			}
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix(prefix, key)
			}
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewFocusBoard} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("FS", key)
			}
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("F.", key)
			}
		},
		StageFilterNames: stages,
	})...)

	// --- Sort group: 1-5, 0 ---
	doSort := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			handleSortKey(key, &m.sort, func() {
				m.applyFilter()
				m.clampCursor()
				m.setStatus("Sort reset")
			}, func(field SortField) {
				m.applyFilter()
				m.clampCursor()
				dir := "↑"
				if !m.sort.Ascending {
					dir = "↓"
				}
				m.setStatus(fmt.Sprintf("Sort: %s %s", field, dir))
			})
			return m, nil
		}
	}
	entries = append(entries, keymap.SortGroup(keymap.SortDeps{
		SortByPriority: doSort("1"),
		SortByDueDate:  doSort("2"),
		SortByCreated:  doSort("3"),
		SortByStatus:   doSort("4"),
		SortByScore:    doSort("5"),
		ResetSort:      doSort("0"),
	})...)

	// --- View-specific direct bindings ---

	// Tab navigation
	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
	)

	// Search
	entries = append(entries,
		keymap.Bind("/", "Search", "Filtering", func() (tea.Model, tea.Cmd) {
			m.searchInput.SetValue(m.searchQuery)
			return m, m.ActivateSearch()
		}),
	)

	// Quick-add / select-all
	entries = append(entries,
		keymap.Bind("a", "Quick-add board item", "Actions", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.selectAllVisible()
				return m, nil
			}
			m.addMode = true
			m.addInput.SetValue("")
			m.addInput.Focus()
			return m, textinput.Blink
		}),
	)

	// Multi-select
	entries = append(entries,
		keymap.Bind("v", "Toggle multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection (multi-select) / hold to peek", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.toggleTaskSelection()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("A", "Deselect all", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		}, keymap.AsHidden()),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Navigate to task / open URL", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleEnter()
		}),
		keymap.Bind("x", "Mark item done", "Actions", func() (tea.Model, tea.Cmd) {
			return m.markDone()
		}),
		keymap.Bind("X", "Dismiss item", "Actions", func() (tea.Model, tea.Cmd) {
			return m.dismiss()
		}),
		keymap.Bind("p", "Set priority", "Actions", func() (tea.Model, tea.Cmd) {
			row := m.getRowAtCursor()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowPriorityPickerMsg{Task: row.Task, Project: row.Project} }
		}),
		keymap.Bind("E", "Quick-edit title", "Actions", func() (tea.Model, tea.Cmd) {
			row := m.getRowAtCursor()
			if row == nil || !row.isTask() {
				return m, nil
			}
			m.editMode = true
			m.editIdx = m.cursor
			m.editInput.SetValue(row.Task.Title)
			m.editInput.Focus()
			return m, textinput.Blink
		}),
	)

	// Daily plan reorder
	entries = append(entries,
		keymap.Bind("J", "Move item down in today plan", "Daily Plan", func() (tea.Model, tea.Cmd) {
			return m.handleDailyPlanReorder(1)
		}),
		keymap.Bind("K", "Move item up in today plan", "Daily Plan", func() (tea.Model, tea.Cmd) {
			return m.handleDailyPlanReorder(-1)
		}),
	)

	// Focus mode cycle: List → Tunnel → Zen → List
	entries = append(entries,
		keymap.Bind("ctrl+f", "Cycle focus mode (List/Tunnel/Zen)", "View", func() (tea.Model, tea.Cmd) {
			m.focusMode = (m.focusMode + 1) % 3
			return m, nil
		}),
	)

	// Refresh
	entries = append(entries,
		keymap.Bind("r", "Refresh", "View", func() (tea.Model, tea.Cmd) {
			m.loadItems()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// Parent navigation
	entries = append(entries,
		keymap.Bind("n", "Next parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToNext(m.parentRowIndices())
		}),
		keymap.Bind("N", "Previous parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToPrev(m.parentRowIndices())
		}),
	)

	// Perspective lens: "P" prefix (P0–P9, PP next, PN prev)
	entries = append(entries, BuildPerspectivePrefixEntry(PerspectivePrefixDeps{
		SelectByKey: func(perspKey int) (tea.Model, tea.Cmd) {
			m.activePerspective = perspKey
			m.applyFilter()
			m.clampCursor()
			if name := PerspectiveLensName(perspKey); name != "" {
				m.setStatus(fmt.Sprintf("Perspective: %s", name))
			}
			return m, nil
		},
		CycleNext: func() (tea.Model, tea.Cmd) {
			m.activePerspective = CyclePerspectiveLens(m.activePerspective, 1)
			m.applyFilter()
			m.clampCursor()
			m.setStatus(fmt.Sprintf("Perspective: %s", PerspectiveLensName(m.activePerspective)))
			return m, nil
		},
		CyclePrev: func() (tea.Model, tea.Cmd) {
			m.activePerspective = CyclePerspectiveLens(m.activePerspective, -1)
			m.applyFilter()
			m.clampCursor()
			m.setStatus(fmt.Sprintf("Perspective: %s", PerspectiveLensName(m.activePerspective)))
			return m, nil
		},
	}))

	// ctrl+t: switch to Backlog / Plan mode
	entries = append(entries,
		keymap.Bind("ctrl+t", "Switch to Backlog (Plan mode)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusViewMsg{} }
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Today",
		Entries:  entries,
	}
}
