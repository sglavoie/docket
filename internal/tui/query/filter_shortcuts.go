package query

import (
	"strings"

	"github.com/sglavoie/docket/internal/models"
)

// FilterShortcut is one preset row of a facet whose vocabulary is closed: the
// f@ five and the f~ pair. The row is a shortcut for a token, not a vocabulary
// of its own: its value is the canonical clause text the same narrowing is typed
// with, and the clauses beside it are what that text means. Keeping both here is
// what lets a picker seed itself by comparing canonical spellings rather than by
// matching a private enum against the filter cache.
type FilterShortcut struct {
	ID        string
	Label     string
	Secondary string
	Clauses   []models.QueryClause
}

// Token is the canonical spelling the row writes, and the value the picker hands
// back. Two rows can never collide on it: each names a different field or a
// different bucket.
func (shortcut FilterShortcut) Token() string {
	return strings.Join(FilterClauseParts(shortcut.Clauses), " ")
}

// DueFilterShortcuts is the f@ row set: the four due buckets plus deferral.
// Deferral is a scheduling instant like a due date, so it is a row here rather
// than a chord of its own, and it writes the deferred field the other four
// leave alone.
func DueFilterShortcuts() []FilterShortcut {
	return []FilterShortcut{
		{ID: "overdue", Label: "Overdue", Secondary: "before today",
			Clauses: DueFilterClauses(DueDateOverdue)},
		{ID: "today", Label: "Today", Secondary: "due today",
			Clauses: DueFilterClauses(DueDateToday)},
		{ID: "week", Label: "This week", Secondary: "within seven days",
			Clauses: DueFilterClauses(DueDateThisWeek)},
		{ID: "none", Label: "No due date", Secondary: "unscheduled tasks",
			Clauses: DueFilterClauses(DueDateNone)},
		{ID: "deferred", Label: "Deferred only", Secondary: "waking later",
			Clauses: FilterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, "")},
	}
}

// DueFilterShortcutForToken resolves the value one row handed back. An unknown
// token is a caller that did not come from the row set.
func DueFilterShortcutForToken(token string) (FilterShortcut, bool) {
	for _, shortcut := range DueFilterShortcuts() {
		if shortcut.Token() == token {
			return shortcut, true
		}
	}
	return FilterShortcut{}, false
}

// DependencyFilterShortcuts is the f~ row set: the two directions of a soft
// reference. Each row is the canonical token it writes, exactly as the f@ rows
// are, so the picker mints no spelling the grammar does not already carry and a
// row typed into search means the same thing it means here.
//
// Neither row is a readiness verdict. The first names other work and the second
// is named by other work; a soft reference gates nothing, so both are structural
// facts and neither reorders, ranks, or delays anything. Each row is labeled
// with its sigil-qualified token rather than the bare word, which is what keeps
// it from reading as the lifecycle state a reader already knows.
func DependencyFilterShortcuts() []FilterShortcut {
	return []FilterShortcut{
		{ID: "?blocked", Label: "?blocked", Secondary: "names other work",
			Clauses: FilterClauseList(models.QueryFieldBlocked, models.QueryOperatorPresent, "")},
		{ID: "?blocking", Label: "?blocking", Secondary: "named by other work",
			Clauses: FilterClauseList(models.QueryFieldBlocking, models.QueryOperatorPresent, "")},
	}
}

// DependencyFilterShortcutForToken resolves the value one f~ row handed back.
func DependencyFilterShortcutForToken(token string) (FilterShortcut, bool) {
	for _, shortcut := range DependencyFilterShortcuts() {
		if shortcut.Token() == token {
			return shortcut, true
		}
	}
	return FilterShortcut{}, false
}
