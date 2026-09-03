package query

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// addDerived compiles one ? value. The parse cannot fail here: ParseSearch has
// already raised the diagnostic for a value the grammar recognized and cannot
// honor, and that guidance returned before this loop ran.
func (p *SearchPredicates) addDerived(value string, into *[]textgrammar.DerivedPredicate) {
	predicate, err := textgrammar.ParseDerived(value)
	if err != nil {
		return
	}
	if into == &p.derived {
		// A completion question is asking about finished work, so it opens the
		// terminal population the way $done does, and a deferral question opens
		// the deferred one. The negations deliberately do not: "not completed
		// recently" is a question about live work.
		p.admitsTerminal = p.admitsTerminal || models.DerivedOpensTerminalPopulation(predicate.Name)
		p.admitsDeferred = p.admitsDeferred || models.DerivedOpensDeferredPopulation(predicate.Name)
	}
	*into = append(*into, predicate)
}

// compileDerivedContext builds the two censuses only some derived predicates
// need, once per query. Both are skipped entirely when nothing asks for them.
func (p *SearchPredicates) compileDerivedContext(projects []*models.Project) {
	if p.needsDerived(models.DerivedNeedsChildCensus) {
		p.derivedContext.openChildren = OpenChildCounts(projects)
	}
	if p.needsDerived(models.DerivedNeedsDependentCensus) {
		p.derivedContext.dependedOn = collectDependencyTargets(projects)
	}
}

// needsDerived asks the canonical property list which census a query reads,
// rather than naming the properties here: the question "what does this predicate
// have to look at" is part of the predicate's meaning and belongs beside it.
func (p *SearchPredicates) needsDerived(reads func(models.DerivedProperty) bool) bool {
	for _, predicate := range append(slices.Clone(p.derived), p.excludedDerived...) {
		if reads(predicate.Name) {
			return true
		}
	}
	return false
}

// OpenChildCounts censuses each task's open direct children across the scope.
// It reads the authoritative flat project.Tasks, which already holds every
// nested child, because a row's own task can be a filter-pipeline clone whose
// Children were rewritten — the marker would then mean one thing in All and
// another in a flat mode.
//
// It sits beside the reference census rather than beside the mode wiring that
// also reads it, because the derived predicates compile against both and cannot
// reach a projection above them for either one.
func OpenChildCounts(projects []*models.Project) map[taskref.TaskIdentity]int {
	counts := make(map[taskref.TaskIdentity]int)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			open := len(task.IncompleteChildren())
			if open == 0 {
				continue
			}
			identity := taskref.For(task, project)
			if !identity.Valid() {
				continue
			}
			counts[identity] = open
		}
	}
	return counts
}

// collectDependencyTargets names every task some loaded task points at. It is
// the same honestly partial census incomingDependencyDetails walks: dependents
// are computed, never stored, so a reference from an unloaded project is not
// knowable and no project is opened to answer a search.
func collectDependencyTargets(projects []*models.Project) map[models.TaskReference]bool {
	targets := make(map[models.TaskReference]bool)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			for _, reference := range task.Dependencies {
				targets[reference] = true
			}
		}
	}
	return targets
}

// addDependency compiles one ~target predicate into the exact stored locator a
// task's own outgoing set must contain. The search is direct: it never walks the
// graph past that one hop, so a task depending on something that depends on the
// target does not match, and there is no negative form to invert it with.
//
// An unresolvable or ambiguous target becomes guidance rather than a predicate
// that quietly matches nothing, because "no results" and "I could not tell which
// task you meant" are different answers.
func (p *SearchPredicates) addDependency(value string, projects []*models.Project, into *[]models.TaskReference) {
	if p.guidance != "" {
		return
	}
	reference, guidance := compileSearchDependency(value, projects)
	if guidance != "" {
		p.guidance = guidance
		return
	}
	if slices.Contains(*into, reference) {
		return
	}
	*into = append(*into, reference)
}

// compileSearchDependency resolves one search target to a durable locator.
//
// A qualified alias resolves against the loaded population the way it does on
// every other surface. A bare alias is read in the scope project when the search
// is scoped to one, and otherwise has to name exactly one task across the loaded
// projects: picking a project on the user's behalf would answer a question they
// did not ask, so ambiguity is reported instead.
func compileSearchDependency(value string, projects []*models.Project) (models.TaskReference, string) {
	value = strings.TrimSpace(value)
	definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldDependency)
	trigger := definition.Trigger
	if _, _, qualified := strings.Cut(value, models.TaskReferenceSeparator); qualified || len(projects) == 1 {
		var effective *models.Project
		if len(projects) == 1 {
			effective = projects[0]
		}
		ref, err := grammar.ResolveDependencyReference(value, effective, projects)
		if err != nil {
			return models.TaskReference{}, err.Error()
		}
		if ref.Locator.TaskID == "" {
			return models.TaskReference{}, grammar.DependencyNotFoundMessage(trigger, value)
		}
		return ref.Locator.Reference(), ""
	}

	var matches []grammar.CapturedDependencyRef
	for _, project := range projects {
		task, err := grammar.ResolveStructuralReference(value, project, grammar.DependencyRelationship)
		if err != nil {
			return models.TaskReference{}, err.Error()
		}
		if task == nil {
			continue
		}
		matches = append(matches, grammar.DependencyRefFor(project, task))
	}
	switch len(matches) {
	case 0:
		return models.TaskReference{}, grammar.DependencyNotFoundMessage(trigger, value)
	case 1:
		return matches[0].Locator.Reference(), ""
	}
	labels := make([]string, len(matches))
	for i, match := range matches {
		labels[i] = match.Label()
	}
	return models.TaskReference{}, fmt.Sprintf(
		"dependency %q matches %s; qualify it as %sproject-slug:short-id",
		value, strings.Join(labels, ", "), trigger,
	)
}

func (p *SearchPredicates) setDestination(value string, projects []*models.Project, plan PlanningMembership) {
	compiled, ok := compileSearchDestination(value, projects, plan)
	if !ok {
		return
	}
	p.destination = &compiled
	// Only a positive destination opens the terminal population. -$done is a
	// question about live work and must not drag every completed task into the
	// candidate set just to reject it.
	p.admitsTerminal = p.admitsTerminal || compiled.destination.WorkflowState().IsTerminal()
}

// compileSearchDestination resolves one destination and, for the two planning
// memberships, the identity set that answers it. A negation compiles through
// here too: "not on Today" needs the same membership the positive did, because
// Today and Next both store Backlog tasks and no state compare separates them.
//
// A caller with no plan to read compiles a membership destination to the empty
// set rather than refusing it: $today from a view that holds no plan names no
// task, which is the same answer an empty plan gives.
func compileSearchDestination(
	value string,
	projects []*models.Project,
	plan PlanningMembership,
) (searchDestinationPredicate, bool) {
	destination, ok := textgrammar.ParseDestination(value)
	if !ok {
		return searchDestinationPredicate{}, false
	}
	compiled := searchDestinationPredicate{destination: destination}
	if plan == nil {
		return compiled, true
	}
	switch destination { //nolint:exhaustive // Only the two planning destinations resolve a membership set; the rest gate on state.
	case textgrammar.DestinationToday:
		compiled.todayIDs = plan.TodayIdentities(projects)
	case textgrammar.DestinationNext:
		compiled.nextIDs = plan.NextIdentities(projects)
	}
	return compiled, true
}

func compileSearchDue(value string, now time.Time) searchDuePredicate {
	if window, matched, err := util.ParseHourWindowAt(value, now); matched {
		if err != nil {
			return searchDuePredicate{}
		}
		return searchDuePredicate{window: window, hasHourWindow: true}
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "overdue":
		return searchDuePredicate{bucket: DueDateOverdue, hasBucket: true}
	case "today":
		return searchDuePredicate{bucket: DueDateToday, hasBucket: true}
	case "week":
		return searchDuePredicate{bucket: DueDateThisWeek, hasBucket: true}
	case "none":
		return searchDuePredicate{bucket: DueDateNone, hasBucket: true}
	}
	day, err := grammar.ParseDate(value)
	if err != nil {
		return searchDuePredicate{}
	}
	return searchDuePredicate{day: day, hasDay: true}
}

func compileSearchDefer(value string, now time.Time) searchDeferPredicate {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "any":
		return searchDeferPredicate{presence: 1}
	case "none":
		return searchDeferPredicate{presence: -1}
	}
	instant, err := util.ParseDeferInstantAt(value, now)
	if err != nil {
		return searchDeferPredicate{}
	}
	return searchDeferPredicate{day: instant, hasDay: true}
}

// normalizeSearchTagPrefix runs the prefix through the same kebab normalization
// the stored tag went through, so #source:github matches the tag persisted as
// source-github rather than nothing.
func normalizeSearchTagPrefix(value string) string {
	return models.NormalizeTag(value)
}
