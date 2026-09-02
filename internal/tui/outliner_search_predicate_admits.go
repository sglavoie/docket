package tui

import (
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

// admitsProject applies the one axis whose subject is the project rather than
// the row. A negated scope drops the whole project, which is what makes
// "%client -%client-archive" a narrowing rather than a per-row filter.
func (p *searchPredicates) admitsProject(project *models.Project) bool {
	if p == nil {
		return true
	}
	if p.projectPrefix == "" && len(p.excludedProjects) == 0 {
		return true
	}
	if project == nil {
		return false
	}
	for _, prefix := range p.excludedProjects {
		if projectCarriesPrefix(project, prefix) {
			return false
		}
	}
	return p.projectPrefix == "" || projectCarriesPrefix(project, p.projectPrefix)
}

func projectCarriesPrefix(project *models.Project, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(project.Slug), prefix) ||
		strings.HasPrefix(strings.ToLower(project.Name), prefix)
}

// admitsPopulation is the population half of the gate, kept apart from the
// narrowing half because it answers a different question: terminal and deferred
// work sit outside the base population every projection loads, and only a query
// naming them puts them back. Terminal is read first, so terminal work that also
// carries a live deferral is decided by the state it ended in.
func (p *searchPredicates) admitsPopulation(task *models.Task) bool {
	if p == nil || task == nil {
		return p == nil
	}
	if task.IsTerminalState() {
		return p.admitsTerminal
	}
	if task.IsDeferredAt(p.now) {
		return p.admitsDeferred
	}
	return true
}

func (p *searchPredicates) admits(task *models.Task, project *models.Project) bool {
	if p == nil {
		return true
	}
	if task == nil {
		return false
	}
	if !p.admitsPriority(task) || !p.admitsPinned(task) || !p.admitsReview(task) {
		return false
	}
	if !p.admitsTags(task) || !p.admitsContext(task) || !p.admitsDue(task) || !p.admitsDeferral(task) {
		return false
	}
	if !p.admitsDependencies(task) || !p.admitsDerived(task, project) {
		return false
	}
	return p.admitsDestination(task, project)
}

// admitsPriority reads every priority through EffectivePriority, so an unset or
// corrupt stored value answers the positive and the negative the same way.
func (p *searchPredicates) admitsPriority(task *models.Task) bool {
	if len(p.priorities) == 0 && len(p.excludedPriorities) == 0 {
		return true
	}
	effective := models.EffectivePriority(task.Priority)
	if slices.Contains(p.excludedPriorities, effective) {
		return false
	}
	return len(p.priorities) == 0 || slices.Contains(p.priorities, effective)
}

// admitsPinned refuses a task that both spellings claim. *pin and -*pin in one
// query is a contradiction, and a contradiction admits nothing rather than
// letting whichever token was typed last silently win.
func (p *searchPredicates) admitsPinned(task *models.Task) bool {
	if p.requirePinned && !task.Pinned {
		return false
	}
	return !p.refusePinned || !task.Pinned
}

// admitsReview answers the review axis the way admitsPinned answers the pin one,
// contradiction included: =review and -=review in one query admit nothing.
func (p *searchPredicates) admitsReview(task *models.Task) bool {
	if p.requireReview && !task.Review {
		return false
	}
	return !p.refuseReview || !task.Review
}

func (p *searchPredicates) admitsTags(task *models.Task) bool {
	for _, prefix := range p.tags {
		if !taskCarriesTagPrefix(task, prefix) {
			return false
		}
	}
	for _, prefix := range p.excludedTags {
		if taskCarriesTagPrefix(task, prefix) {
			return false
		}
	}
	return true
}

// admitsDependencies requires the task's stored set to contain every requested
// target and none of the excluded ones. Repeated predicates AND, matching every
// other repeatable sigil, and the negation is the AND-of-nots the join law asks
// for on any cardinality.
func (p *searchPredicates) admitsDependencies(task *models.Task) bool {
	for _, reference := range p.dependencies {
		if !slices.Contains(task.Dependencies, reference) {
			return false
		}
	}
	for _, reference := range p.excludedDeps {
		if slices.Contains(task.Dependencies, reference) {
			return false
		}
	}
	return true
}

func (p *searchPredicates) admitsContext(task *models.Task) bool {
	if len(p.contexts) == 0 && len(p.excludedContexts) == 0 {
		return true
	}
	context, err := models.NormalizeTaskContext(task.Context)
	if err != nil {
		// An unreadable context satisfies no positive prefix, and excluding it
		// would claim it is not something it cannot be read as either. Only a
		// positive query refuses it.
		return len(p.contexts) == 0
	}
	// Repeated contexts union. A task carries at most one context, so an
	// intersection of two names could never admit anything; the join law reads
	// cardinality, and a single-valued field joins with OR. This is the same
	// reading fc has always had, now spelled the same way in both places.
	if len(p.contexts) > 0 {
		matched := false
		for _, prefix := range p.contexts {
			if strings.HasPrefix(context, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, prefix := range p.excludedContexts {
		if context != "" && strings.HasPrefix(context, prefix) {
			return false
		}
	}
	return true
}

func (p *searchPredicates) admitsDue(task *models.Task) bool {
	if !matchesSearchDue(task, p.due, p.now) {
		return false
	}
	for _, excluded := range p.excludedDue {
		if matchesSearchDue(task, excluded, p.now) {
			return false
		}
	}
	return true
}

// matchesSearchDue answers one compiled due predicate. A zero predicate matches
// everything, which is what lets the positive and the negative share it: an
// unusable value compiled to nothing narrows nothing in either direction.
func matchesSearchDue(task *models.Task, predicate searchDuePredicate, now time.Time) bool {
	switch {
	case predicate.hasBucket:
		return dueBucketAdmits(task, predicate.bucket, now)
	case predicate.hasHourWindow:
		return task.DueDate != nil && !task.DueDate.After(predicate.window)
	case predicate.hasDay:
		return task.DueDate != nil && util.DaysBetween(predicate.day, *task.DueDate) == 0
	default:
		return true
	}
}

// admitsDeferral splits the population the way the deferred filter facet does:
// a lapsed deferral keeps its instant but is no longer deferred, so @@any names
// the tasks currently hidden and @@none everything else. The day form is
// deliberately literal instead, since a query naming a wake day is asking about
// the instant itself.
func (p *searchPredicates) admitsDeferral(task *models.Task) bool {
	if !matchesSearchDefer(task, p.deferral, p.now) {
		return false
	}
	for _, excluded := range p.excludedDeferral {
		if matchesSearchDefer(task, excluded, p.now) {
			return false
		}
	}
	return true
}

func matchesSearchDefer(task *models.Task, predicate searchDeferPredicate, now time.Time) bool {
	switch {
	case predicate.presence > 0:
		return task.IsDeferredAt(now)
	case predicate.presence < 0:
		return !task.IsDeferredAt(now)
	case predicate.hasDay:
		return task.DeferredUntil != nil && util.DaysBetween(predicate.day, *task.DeferredUntil) == 0
	default:
		return true
	}
}

func (p *searchPredicates) admitsDestination(task *models.Task, project *models.Project) bool {
	for _, excluded := range p.excludedDests {
		if excluded.matches(task, project) {
			return false
		}
	}
	if p.destination == nil {
		return true
	}
	return p.destination.matches(task, project)
}

// matches answers one compiled destination for one row.
func (d searchDestinationPredicate) matches(task *models.Task, project *models.Project) bool {
	identity := taskIdentityFor(task, project)
	switch d.destination {
	case textgrammar.DestinationToday:
		return d.todayIDs[identity]
	case textgrammar.DestinationNext:
		return d.nextIDs[identity]
	default:
		return task.State() == d.destination.WorkflowState()
	}
}

// admitsDerived answers the computed predicates. Each is a question the task
// does not store, so none of them can be a stored-field compare: two read a
// census built once per query, and the rest read the clock or the task's own
// reference list.
func (p *searchPredicates) admitsDerived(task *models.Task, project *models.Project) bool {
	for _, predicate := range p.derived {
		if !p.matchesDerived(predicate, task, project) {
			return false
		}
	}
	for _, predicate := range p.excludedDerived {
		if p.matchesDerived(predicate, task, project) {
			return false
		}
	}
	return true
}

// matchesDerived hands the question to the canonical answer, having supplied the
// two facts a task cannot answer alone. The census lookups are cheap map reads
// against tables compileDerivedContext built only if this query asked for them;
// an absent table reads as a zero, which no predicate reaching here can see,
// because the same list decided whether to build it.
func (p *searchPredicates) matchesDerived(
	predicate textgrammar.DerivedPredicate,
	task *models.Task,
	project *models.Project,
) bool {
	identity := taskIdentityFor(task, project)
	census := models.DerivedCensus{
		OpenChildren: p.derivedContext.openChildren[identity],
		HasDependents: p.derivedContext.dependedOn[models.TaskReference{
			ProjectID: identity.ProjectID,
			TaskID:    identity.TaskID,
		}],
	}
	return models.TaskMatchesDerived(predicate.Name, predicate.Days, task, census, p.now)
}

// taskCarriesTagPrefix matches over normalized tag names only. Search keeps the
// tag and context axes apart because it offers a separate sigil for each, and
// so does the filter grammar now that contexts answer to fc rather than f#.
func taskCarriesTagPrefix(task *models.Task, prefix string) bool {
	for _, raw := range task.Tags {
		tag, err := models.NormalizeTaskTag(raw)
		if err == nil && strings.HasPrefix(tag, prefix) {
			return true
		}
	}
	return false
}
