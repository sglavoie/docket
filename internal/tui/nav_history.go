package tui

const maxNavHistoryEntries = 200

// NavEntry describes a view location for navigation history.
type NavEntry struct {
	ViewType  ViewType
	ProjectID string // empty for global views
	TaskID    string // empty for non-task views
}

// NavHistory tracks a linear history of visited views for back/forward navigation.
// It is orthogonal to the view stack (which handles hierarchical q/Esc navigation).
type NavHistory struct {
	entries []NavEntry
	cursor  int
	locked  bool
}

// NewNavHistory creates a new empty navigation history.
func NewNavHistory() *NavHistory {
	return &NavHistory{
		cursor: -1,
	}
}

// Record adds a new entry to the history. It deduplicates consecutive identical
// entries, truncates any forward history when the cursor is not at the end, and
// caps the total at maxNavHistoryEntries by evicting the oldest.
func (h *NavHistory) Record(entry NavEntry) {
	if h.locked {
		return
	}

	// Deduplicate consecutive identical entries
	if h.cursor >= 0 && h.cursor < len(h.entries) {
		prev := h.entries[h.cursor]
		if prev.ViewType == entry.ViewType && prev.ProjectID == entry.ProjectID && prev.TaskID == entry.TaskID {
			return
		}
	}

	// Truncate forward history
	h.entries = h.entries[:h.cursor+1]

	h.entries = append(h.entries, entry)
	h.cursor = len(h.entries) - 1

	// Evict oldest if over cap
	if len(h.entries) > maxNavHistoryEntries {
		excess := len(h.entries) - maxNavHistoryEntries
		h.entries = h.entries[excess:]
		h.cursor -= excess
	}
}

// Current returns the entry at the current cursor position without moving it.
// Returns false if the history is empty.
func (h *NavHistory) Current() (NavEntry, bool) {
	if h.cursor < 0 || h.cursor >= len(h.entries) {
		return NavEntry{}, false
	}
	return h.entries[h.cursor], true
}

// GoBack moves the cursor one step back and returns the entry, or false if at the beginning.
func (h *NavHistory) GoBack() (NavEntry, bool) {
	if !h.CanGoBack() {
		return NavEntry{}, false
	}
	h.cursor--
	return h.entries[h.cursor], true
}

// GoForward moves the cursor one step forward and returns the entry, or false if at the end.
func (h *NavHistory) GoForward() (NavEntry, bool) {
	if !h.CanGoForward() {
		return NavEntry{}, false
	}
	h.cursor++
	return h.entries[h.cursor], true
}

// CanGoBack returns true if there is history before the current position.
func (h *NavHistory) CanGoBack() bool {
	return h.cursor > 0
}

// CanGoForward returns true if there is history after the current position.
func (h *NavHistory) CanGoForward() bool {
	return h.cursor < len(h.entries)-1
}

// Lock prevents recording during history navigation.
func (h *NavHistory) Lock() {
	h.locked = true
}

// Unlock re-enables recording.
func (h *NavHistory) Unlock() {
	h.locked = false
}

// IsLocked returns whether recording is currently locked.
func (h *NavHistory) IsLocked() bool {
	return h.locked
}

// isTrackedView returns true if the given ViewType should be recorded in history.
// Pickers, editors, creators, modals, and overlays are excluded.
func isTrackedView(vt ViewType) bool {
	switch vt {
	case ViewDashboard,
		ViewProjectTree,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewFocusView,
		ViewProjectFocusView,
		ViewDueDateOverview,
		ViewPriorityView,
		ViewRecentActivity,
		ViewGlobalSearch,
		ViewGlobalBlockers,
		ViewGlobalNotes,
		ViewGlobalInboxProcessor,
		ViewGlobalTagManager,
		ViewNotes,
		ViewBlockers,
		ViewInboxProcessor,
		ViewSettings,
		ViewWorkflowRules,
		ViewTaskTemplates,
		ViewUndoHistory,
		ViewTutor,
		ViewScoreProfileManager,
		ViewTaskComments,
		ViewTaskChecklist,
		ViewTaskDetail,
		ViewTaskAttachments,
		ViewTaskActivity,
		ViewMetricsDashboard,
		ViewAllTasks,
		ViewOutliner,
		ViewProjectRecentActivity:
		return true
	default:
		return false
	}
}

// isTaskScopedView returns true if the view is scoped to a specific task.
func isTaskScopedView(vt ViewType) bool {
	switch vt {
	case ViewTaskComments, ViewTaskChecklist, ViewTaskDetail, ViewTaskAttachments, ViewTaskActivity:
		return true
	default:
		return false
	}
}
