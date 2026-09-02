package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

const (
	operationFilterPriority      OperationID = "tui.filter.priority"
	operationFilterStates        OperationID = "tui.filter.states"
	operationFilterTags          OperationID = "tui.filter.tags"
	operationFilterContexts      OperationID = "tui.filter.contexts"
	operationFilterReview        OperationID = "tui.filter.review"
	operationFilterDue           OperationID = "tui.filter.due"
	operationFilterCompletionAge OperationID = "tui.filter.completion-age"
	operationFilterHierarchy     OperationID = "tui.filter.hierarchy"
	operationFilterDependency    OperationID = "tui.filter.dependency"
	operationFilterStale         OperationID = "tui.filter.stale"
	operationFilterPinned        OperationID = "tui.filter.pinned"
	operationFilterClear         OperationID = "tui.filter.clear"
)

// filterShortcut is one preset row of a facet whose vocabulary is closed: the
// f@ five and the f~ pair. The row is a shortcut for a token, not a vocabulary
// of its own: its value is the canonical clause text the same narrowing is typed
// with, and the clauses beside it are what that text means. Keeping both here is
// what lets a picker seed itself by comparing canonical spellings rather than by
// matching a private enum against the filter cache.
type filterShortcut struct {
	id        string
	label     string
	secondary string
	clauses   []models.QueryClause
}

// token is the canonical spelling the row writes, and the value the picker hands
// back. Two rows can never collide on it: each names a different field or a
// different bucket.
func (shortcut filterShortcut) token() string {
	return strings.Join(filterClauseParts(shortcut.clauses), " ")
}

// dueFilterShortcuts is the f@ row set: the four due buckets plus deferral.
// Deferral is a scheduling instant like a due date, so it is a row here rather
// than a chord of its own, and it writes the deferred field the other four
// leave alone.
func dueFilterShortcuts() []filterShortcut {
	return []filterShortcut{
		{id: "overdue", label: "Overdue", secondary: "before today",
			clauses: dueFilterClauses(DueDateOverdue)},
		{id: "today", label: "Today", secondary: "due today",
			clauses: dueFilterClauses(DueDateToday)},
		{id: "week", label: "This week", secondary: "within seven days",
			clauses: dueFilterClauses(DueDateThisWeek)},
		{id: "none", label: "No due date", secondary: "unscheduled tasks",
			clauses: dueFilterClauses(DueDateNone)},
		{id: "deferred", label: "Deferred only", secondary: "waking later",
			clauses: filterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, "")},
	}
}

// dueFilterShortcutForToken resolves the value one row handed back. An unknown
// token is a caller that did not come from the row set.
func dueFilterShortcutForToken(token string) (filterShortcut, bool) {
	for _, shortcut := range dueFilterShortcuts() {
		if shortcut.token() == token {
			return shortcut, true
		}
	}
	return filterShortcut{}, false
}

// dependencyFilterShortcuts is the f~ row set: the two directions of a soft
// reference. Each row is the canonical token it writes, exactly as the f@ rows
// are, so the picker mints no spelling the grammar does not already carry and a
// row typed into search means the same thing it means here.
//
// Neither row is a readiness verdict. The first names other work and the second
// is named by other work; a soft reference gates nothing, so both are structural
// facts and neither reorders, ranks, or delays anything. Each row is labeled
// with its sigil-qualified token rather than the bare word, which is what keeps
// it from reading as the lifecycle state a reader already knows.
func dependencyFilterShortcuts() []filterShortcut {
	return []filterShortcut{
		{id: "?blocked", label: "?blocked", secondary: "names other work",
			clauses: filterClauseList(models.QueryFieldBlocked, models.QueryOperatorPresent, "")},
		{id: "?blocking", label: "?blocking", secondary: "named by other work",
			clauses: filterClauseList(models.QueryFieldBlocking, models.QueryOperatorPresent, "")},
	}
}

// dependencyFilterShortcutForToken resolves the value one f~ row handed back.
func dependencyFilterShortcutForToken(token string) (filterShortcut, bool) {
	for _, shortcut := range dependencyFilterShortcuts() {
		if shortcut.token() == token {
			return shortcut, true
		}
	}
	return filterShortcut{}, false
}
