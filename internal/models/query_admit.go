package models

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/util"
)

// Admits reports whether one task, read inside one project at one instant,
// survives the query's clause list. It mutates nothing and reads nothing but its
// three arguments, so the same triple always gives the same answer.
//
// The join law, stated once and applied here:
//
//   - across fields, AND;
//   - within a field, OR when the underlying task attribute is single-valued;
//   - within a field, AND when the attribute is multi-valued;
//   - negative clauses, always AND-of-nots, at any cardinality and on any field.
//
// Field cardinality is what decides, so repeated contexts union while repeated
// tags intersect. That is one rule applied twice, not two rules.
//
// A nil task or project is refused: a row that names no task, or a task read
// outside the project that owns it, is not something a predicate can answer. A
// zero now falls back to the wall clock so a caller that has no injected clock
// still gets the relative clauses evaluated rather than silently dropped.
func (q Query) Admits(task *Task, project *Project, now time.Time) bool {
	if task == nil || project == nil {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if len(q.Clauses) == 0 {
		return true
	}
	grouped := make(map[QueryField][]QueryClause, len(q.Clauses))
	for _, clause := range q.Clauses {
		grouped[clause.Field] = append(grouped[clause.Field], clause)
	}
	for field, clauses := range grouped {
		if !admitsQueryField(field, clauses, task, project, now) {
			return false
		}
	}
	return true
}

func admitsQueryField(field QueryField, clauses []QueryClause, task *Task, project *Project, now time.Time) bool {
	join, err := QueryFieldJoin(field)
	if err != nil {
		return false
	}
	positives := 0
	admitted := join == QueryJoinAND
	for _, clause := range clauses {
		matched := queryClauseMatches(clause, task, project, now)
		if clause.Polarity == QueryPolarityNegative {
			if matched {
				return false
			}
			continue
		}
		positives++
		switch join {
		case QueryJoinAND:
			admitted = admitted && matched
		default:
			admitted = admitted || matched
		}
	}
	if positives == 0 {
		return true
	}
	return admitted
}

func queryClauseMatches(clause QueryClause, task *Task, project *Project, now time.Time) bool {
	switch clause.Field {
	case QueryFieldText:
		return queryTextMatches(task, project, clause.Value)
	case QueryFieldDependency:
		return slices.ContainsFunc(task.Dependencies, func(reference TaskReference) bool {
			return reference.Locator() == clause.Value
		})
	case QueryFieldAttachment:
		if clause.Operator == QueryOperatorNone {
			return len(task.Attachments) == 0
		}
		return len(task.Attachments) > 0
	case QueryFieldPriority:
		return queryPriorityMatches(task, clause)
	case QueryFieldTag:
		return queryTagMatches(task, clause)
	case QueryFieldContext:
		return queryContextMatches(task, clause)
	case QueryFieldProject:
		return project.Slug != "" && strings.EqualFold(project.Slug, clause.Value)
	case QueryFieldDue:
		return queryDueMatches(task, clause, now)
	case QueryFieldDefer:
		return queryDeferDayMatches(task, clause.Value, now)
	case QueryFieldState:
		return string(task.State()) == clause.Value
	case QueryFieldPinned:
		return task.Pinned
	case QueryFieldReview:
		return task.Review
	case QueryFieldNeedsReview:
		return TaskOwesReviewAttention(project, task, now)
	case QueryFieldStale:
		return taskOwesStaleAttention(project, task, now)
	case QueryFieldDeferred:
		return task.IsDeferredAt(now)
	case QueryFieldCompleted:
		return queryCompletedMatches(task, clause, now)
	case QueryFieldParent:
		return queryOpenChildCount(project, task) > 0
	case QueryFieldChild:
		return task.ParentID != nil
	case QueryFieldLeaf:
		return queryOpenChildCount(project, task) == 0
	case QueryFieldBlocked:
		return len(task.Dependencies) > 0
	case QueryFieldBlocking:
		// Dependents live in other tasks, so no reading of this one can decide
		// them. The refusal is deliberate rather than a gap: a caller holding the
		// population's reference index lifts this field out before it ever gets
		// here, and a stray clause that reached this far names a fact the
		// predicate has no way to establish.
		return false
	case QueryFieldProjectTag, QueryFieldParked, QueryFieldArchived, QueryFieldHidden:
		return queryPopulationClauseMatches(clause, task, project)
	default:
		return false
	}
}

// queryPopulationClauseMatches answers the clauses whose field names a
// population — the project tags, parking, archiving, and hiding — rather than a
// task value.
func queryPopulationClauseMatches(clause QueryClause, task *Task, project *Project) bool {
	switch clause.Field {
	case QueryFieldProjectTag:
		if clause.Operator == QueryOperatorNone {
			return len(project.Tags) == 0
		}
		return slices.Contains(project.Tags, clause.Value)
	case QueryFieldParked:
		return queryPopulationMatches(clause.Value, ProjectTagsParked(project.Tags))
	case QueryFieldArchived:
		return queryPopulationMatches(clause.Value, project.Archived)
	case QueryFieldHidden:
		return queryPopulationMatches(clause.Value, TaskEffectivelyHidden(project, task))
	default:
		return false
	}
}

// queryPopulationMatches applies one population clause as a row predicate.
// Include narrows nothing: its whole effect is the reload obligation that put
// the population in front of the predicate in the first place. Only isolates,
// which is a narrowing and is evaluated here.
func queryPopulationMatches(value string, carries bool) bool {
	if value == QueryPopulationOnly {
		return carries
	}
	return true
}

// TaskOwesReviewAttention and taskOwesStaleAttention are the two attention
// questions every query surface asks. Hidden work is never owed attention, so
// neither admits it, even where a hidden clause has opened that population;
// the filter clause, the search ?review/?stale predicates, and the row's review
// badge share them so the readings cannot disagree. A nil project resolves
// inheritance through the task's Parent pointers.
func TaskOwesReviewAttention(project *Project, task *Task, now time.Time) bool {
	return TaskNeedsAttention(task, now) && !TaskEffectivelyHidden(project, task)
}

func taskOwesStaleAttention(project *Project, task *Task, now time.Time) bool {
	return QueryTaskIsStale(task, now) && !TaskEffectivelyHidden(project, task)
}

// QueryTaskIsStale reports whether ordinary open work has gone untouched for at
// least StaleTaskThresholdDays local calendar days. UpdatedAt is the touch
// clock; StateChangedAt deliberately does not participate, because a task can
// sit in one state for months while being actively worked.
func QueryTaskIsStale(task *Task, now time.Time) bool {
	if task == nil || task.IsTerminalState() || task.IsDeferredAt(now) {
		return false
	}
	today := util.StartOfDay(now)
	updated := util.StartOfDay(task.UpdatedAt.In(now.Location()))
	return util.DaysBetween(updated, today) >= StaleTaskThresholdDays
}

// queryPriorityMatches reads every priority through EffectivePriority, so an
// unset or corrupt stored value has one behavior across all three readings
// instead of one per operator.
func queryPriorityMatches(task *Task, clause QueryClause) bool {
	effective := EffectivePriority(task.Priority)
	if clause.Operator == QueryOperatorNone {
		return effective > PriorityLow
	}
	wanted, err := strconv.Atoi(clause.Value)
	if err != nil {
		return false
	}
	if clause.Operator == QueryOperatorAtLeast {
		return effective <= wanted
	}
	return effective == wanted
}

func queryTagMatches(task *Task, clause QueryClause) bool {
	if clause.Operator != QueryOperatorPrefix {
		return task.HasTag(clause.Value)
	}
	for _, existing := range task.Tags {
		if normalized, err := NormalizeTaskTag(existing); err == nil && strings.HasPrefix(normalized, clause.Value) {
			return true
		}
	}
	return false
}

func queryContextMatches(task *Task, clause QueryClause) bool {
	switch clause.Operator {
	case QueryOperatorNone:
		return task.Context == ""
	case QueryOperatorPrefix:
		normalized, err := NormalizeTaskContext(task.Context)
		return err == nil && strings.HasPrefix(normalized, clause.Value)
	default:
		return task.HasContext(clause.Value)
	}
}

// queryDueMatches reproduces the live due facet exactly: overdue is terminal
// aware and instant precise only when the due date carries a clock time, while
// every other reading compares local calendar days.
func queryDueMatches(task *Task, clause QueryClause, now time.Time) bool {
	if clause.Operator == QueryOperatorNone {
		return task.DueDate == nil
	}
	if task.DueDate == nil {
		return false
	}
	switch clause.Value {
	case QueryDueOverdue:
		if task.IsTerminalState() {
			return false
		}
		if task.DueHasTime() {
			return task.DueDate.Before(now)
		}
		return util.DaysBetween(now, *task.DueDate) < 0
	case QueryDueToday:
		return util.DaysBetween(now, *task.DueDate) == 0
	case QueryDueWeek:
		days := util.DaysBetween(now, *task.DueDate)
		return days >= 0 && days < 7
	default:
	}
	if strings.HasPrefix(clause.Value, "+") {
		return queryDueWindowMatches(task, clause.Value, now)
	}
	return queryDayMatches(*task.DueDate, clause.Value, now)
}

// queryDueWindowMatches reads a relative due value the way the typed `/`
// search does, so a lens saved from a search admits what the search showed: an
// hour window is everything due by its end, overdue work included, and a day or
// week offset is the one calendar day that far out, like any other date.
func queryDueWindowMatches(task *Task, value string, now time.Time) bool {
	count, hours, err := parseQueryWindow(value)
	if err != nil {
		return false
	}
	if hours {
		return !task.DueDate.After(now.Add(time.Duration(count) * time.Hour))
	}
	return util.DaysBetween(now, *task.DueDate) == count
}

func queryDeferDayMatches(task *Task, value string, now time.Time) bool {
	if task.DeferredUntil == nil {
		return false
	}
	return queryDayMatches(*task.DeferredUntil, value, now)
}

// queryDayMatches compares a stored instant to a calendar day in the caller's
// location, which is the same reading a date-only due date is stored under.
func queryDayMatches(stored time.Time, value string, now time.Time) bool {
	wanted, err := time.ParseInLocation(QueryDateLayout, value, now.Location())
	if err != nil {
		return false
	}
	return util.StartOfDay(stored.In(now.Location())).Equal(util.StartOfDay(wanted))
}

// queryCompletedMatches keeps the live cleanup facet's shape: a task with no
// usable completion timestamp is excluded even when the clause sets no age
// bound, so a malformed record can never reach a cleanup view.
func queryCompletedMatches(task *Task, clause QueryClause, now time.Time) bool {
	if clause.Operator != QueryOperatorAtLeast {
		return taskCompletedAtLeastDaysAgo(task, 0, now)
	}
	digits, _ := strings.CutSuffix(clause.Value, "d")
	threshold, err := strconv.Atoi(digits)
	if err != nil {
		return false
	}
	return taskCompletedAtLeastDaysAgo(task, threshold, now)
}

// queryOpenChildCount counts the open direct children the task carries as it was
// handed over. A projection that rewrites Children to express a promoted or
// flattened view is therefore answered about the tree it is actually showing,
// which is the same census the row's own child marker reads. A visible parent
// does not count a child storing the hidden flag, so parent and leaf describe
// what expanding it shows; a hidden parent's children are hidden with it and
// all count.
func queryOpenChildCount(project *Project, task *Task) int {
	parentHidden := TaskEffectivelyHidden(project, task)
	count := 0
	for _, child := range task.Children {
		if child != nil && !child.IsTerminalState() && (parentHidden || !child.Hidden) {
			count++
		}
	}
	return count
}

// queryTextMatches applies the one free-text matcher, case insensitively, over
// the whole task-side corpus. Notes are not part of it: a note is not reachable
// from a task, so admitting one would need a resolver this predicate does not
// take.
func queryTextMatches(task *Task, project *Project, value string) bool {
	needle := strings.ToLower(value)
	fields := []string{task.Title, task.Description, task.ResumeNote, task.Context, project.Name, project.Slug}
	fields = append(fields, task.Tags...)
	for _, item := range task.Checklist {
		fields = append(fields, item.Text)
	}
	for _, comment := range task.Comments {
		if comment != nil {
			fields = append(fields, comment.Text)
		}
	}
	for _, attachment := range task.Attachments {
		fields = append(fields, attachment.Filename, attachment.OriginalPath)
	}
	for _, field := range fields {
		if field != "" && strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}
