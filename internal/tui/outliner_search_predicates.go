package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

// searchDuePredicate is either one relational bucket, one exact calendar day,
// or a rolling "due within N hours" window (@+Nh). The window is filter-only:
// hours never become a stored due value, but a search can ask for work coming
// due inside the next few hours now that a due date can carry a clock time.
type searchDuePredicate struct {
	bucket        DueDateFilter
	day           time.Time
	window        time.Time
	hasBucket     bool
	hasDay        bool
	hasHourWindow bool
}

// searchDeferPredicate is either one population split or one exact day. A
// deferral is an instant, so the day compare is what makes @@+2h and @@today
// name the same set.
type searchDeferPredicate struct {
	day      time.Time
	hasDay   bool
	presence int // 0 none, +1 @@any, -1 @@none
}

// searchDestinationPredicate is one compiled $destination. Today and Next are
// memberships rather than states, so each compiled destination carries the set
// it resolved; a negation needs the same resolution the positive did, because
// "not on Today" cannot be answered from a state compare either.
type searchDestinationPredicate struct {
	destination textgrammar.Destination
	todayIDs    map[TaskIdentity]bool
	nextIDs     map[TaskIdentity]bool
}

// searchDerivedContext holds the two censuses a derived predicate needs and a
// task cannot answer alone. Both are computed once per query, and only when a
// predicate that reads them is present, so an ordinary search walks no extra
// tree and scans no other project.
type searchDerivedContext struct {
	openChildren map[TaskIdentity]int
	dependedOn   map[models.TaskReference]bool
}

// searchPredicates is the compiled sigil gate for one search query. It is
// deliberately built without a fuzzy.Matcher: the case-relaxation retry re-runs
// the scoring pass, and a gate that read the matcher could admit a different set
// on the second pass than on the first.
//
// Every axis holds its positives and its negatives apart, because they join
// differently: positives narrow by their own field's rule, while a negative is
// always an AND-of-nots. Repeated negations therefore accumulate on every axis,
// including the ones whose positive form is a singleton — "not today and not
// tomorrow" is one coherent question about a single-valued field.
type searchPredicates struct {
	priorities         []int
	excludedPriorities []int
	tags               []string
	excludedTags       []string
	contexts           []string
	excludedContexts   []string
	due                searchDuePredicate
	excludedDue        []searchDuePredicate
	deferral           searchDeferPredicate
	excludedDeferral   []searchDeferPredicate
	dependencies       []models.TaskReference
	excludedDeps       []models.TaskReference
	destination        *searchDestinationPredicate
	excludedDests      []searchDestinationPredicate
	requirePinned      bool
	refusePinned       bool
	projectPrefix      string
	excludedProjects   []string
	derived            []textgrammar.DerivedPredicate
	excludedDerived    []textgrammar.DerivedPredicate
	derivedContext     searchDerivedContext
	admitsTerminal     bool
	now                time.Time
	guidance           string
	// sigils records that the query carried at least one sigil token. It is
	// read off the parse rather than off the compiled conjuncts so that a
	// destination or pin sigil, which compiles into a field rather than into a
	// slice, counts the same as a tag. A sigil the parser refuses outright
	// never reaches this flag; that query is already invalid().
	sigils bool
}

// compileSearchQuery splits one query into the sigil gate and the free-text
// remainder that keeps today's fuzzy ranking. The query arrives explicitly
// rather than being read off the search input, because a lens narrows with its
// own saved query and must compile it through this same grammar: one parser,
// one predicate set, one remainder.
func (m *OutlinerModel) compileSearchQuery(query string, projects []*models.Project) (string, *searchPredicates) {
	parsed := textgrammar.ParseSearch(strings.TrimSpace(query))
	predicates := &searchPredicates{now: time.Now(), sigils: len(parsed.Tokens) > 0}
	if guidance := textgrammar.SearchGuidance(parsed); guidance != "" {
		predicates.guidance = guidance
		return parsed.Text, predicates
	}

	for _, token := range parsed.Tokens {
		predicates.applyToken(token, projects, m)
	}
	predicates.compileDerivedContext(projects)
	return parsed.Text, predicates
}

// applyToken compiles one token onto the gate. A negation is never a separate
// compilation path: it reads the same value the positive would have, and only
// the slice it lands in differs, which is what keeps -@friday and @friday
// naming the same day.
func (p *searchPredicates) applyToken(token textgrammar.Token, projects []*models.Project, m *OutlinerModel) {
	switch token.Field { //nolint:exhaustive // The placement and retired fields refuse in SearchGuidance before this loop.
	case textgrammar.FieldPriority:
		if values, ok := textgrammar.ParseSearchPriority(token.Value); ok {
			p.priorities = values
		}
	case textgrammar.FieldRemovePriority:
		// "Not in A" and "not in B" is "not in A or B", so every excluded
		// threshold flattens into one set rather than one set per token.
		if values, ok := textgrammar.ParseSearchPriority(token.Value); ok {
			p.excludedPriorities = append(p.excludedPriorities, values...)
		}
	case textgrammar.FieldTag:
		p.tags = append(p.tags, normalizeSearchTagPrefix(token.Value))
	case textgrammar.FieldRemoveTag:
		p.excludedTags = append(p.excludedTags, normalizeSearchTagPrefix(token.Value))
	case textgrammar.FieldContext:
		p.contexts = append(p.contexts, normalizeSearchTagPrefix(token.Value))
	case textgrammar.FieldRemoveContext:
		p.excludedContexts = append(p.excludedContexts, normalizeSearchTagPrefix(token.Value))
	case textgrammar.FieldDue:
		p.due = compileSearchDue(token.Value, p.now)
	case textgrammar.FieldRemoveDue:
		p.excludedDue = append(p.excludedDue, compileSearchDue(token.Value, p.now))
	case textgrammar.FieldDefer:
		p.deferral = compileSearchDefer(token.Value, p.now)
	case textgrammar.FieldRemoveDefer:
		p.excludedDeferral = append(p.excludedDeferral, compileSearchDefer(token.Value, p.now))
	case textgrammar.FieldDestination:
		p.setDestination(token.Value, projects, m)
	case textgrammar.FieldRemoveDestination:
		if compiled, ok := compileSearchDestination(token.Value, projects, m); ok {
			p.excludedDests = append(p.excludedDests, compiled)
		}
	case textgrammar.FieldPinned:
		if token.Value == "pin" {
			p.requirePinned = true
			return
		}
		p.refusePinned = true
	case textgrammar.FieldRemovePinned:
		p.refusePinned = true
	case textgrammar.FieldProject:
		p.projectPrefix = strings.ToLower(strings.TrimSpace(token.Value))
	case textgrammar.FieldRemoveProject:
		p.excludedProjects = append(p.excludedProjects, strings.ToLower(strings.TrimSpace(token.Value)))
	case textgrammar.FieldDependency:
		p.addDependency(token.Value, projects, &p.dependencies)
	case textgrammar.FieldRemoveDependency:
		p.addDependency(token.Value, projects, &p.excludedDeps)
	case textgrammar.FieldDerived:
		p.addDerived(token.Value, &p.derived)
	case textgrammar.FieldRemoveDerived:
		p.addDerived(token.Value, &p.excludedDerived)
	}
}

// addDerived compiles one ? value. The parse cannot fail here: ParseSearch has
// already raised the diagnostic for a value the grammar recognized and cannot
// honor, and that guidance returned before this loop ran.
func (p *searchPredicates) addDerived(value string, into *[]textgrammar.DerivedPredicate) {
	predicate, err := textgrammar.ParseDerived(value)
	if err != nil {
		return
	}
	if models.DerivedOpensTerminalPopulation(predicate.Name) && into == &p.derived {
		// A completion question is asking about finished work, so it opens the
		// terminal population the way $done does. The negation deliberately does
		// not: "not completed recently" is a question about live work.
		p.admitsTerminal = true
	}
	*into = append(*into, predicate)
}

// compileDerivedContext builds the two censuses only some derived predicates
// need, once per query. Both are skipped entirely when nothing asks for them.
func (p *searchPredicates) compileDerivedContext(projects []*models.Project) {
	if p.needsDerived(models.DerivedNeedsChildCensus) {
		p.derivedContext.openChildren = openChildCounts(projects)
	}
	if p.needsDerived(models.DerivedNeedsDependentCensus) {
		p.derivedContext.dependedOn = collectDependencyTargets(projects)
	}
}

// needsDerived asks the canonical property list which census a query reads,
// rather than naming the properties here: the question "what does this predicate
// have to look at" is part of the predicate's meaning and belongs beside it.
func (p *searchPredicates) needsDerived(reads func(models.DerivedProperty) bool) bool {
	for _, predicate := range append(slices.Clone(p.derived), p.excludedDerived...) {
		if reads(predicate.Name) {
			return true
		}
	}
	return false
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
func (p *searchPredicates) addDependency(value string, projects []*models.Project, into *[]models.TaskReference) {
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
		ref, err := resolveDependencyReference(value, effective, projects)
		if err != nil {
			return models.TaskReference{}, err.Error()
		}
		if ref.locator.TaskID == "" {
			return models.TaskReference{}, dependencyNotFoundMessage(trigger, value)
		}
		return ref.locator.Reference(), ""
	}

	var matches []capturedDependencyRef
	for _, project := range projects {
		task, err := resolveStructuralReference(value, project, dependencyRelationship)
		if err != nil {
			return models.TaskReference{}, err.Error()
		}
		if task == nil {
			continue
		}
		matches = append(matches, dependencyRefFor(project, task))
	}
	switch len(matches) {
	case 0:
		return models.TaskReference{}, dependencyNotFoundMessage(trigger, value)
	case 1:
		return matches[0].locator.Reference(), ""
	}
	labels := make([]string, len(matches))
	for i, match := range matches {
		labels[i] = match.label()
	}
	return models.TaskReference{}, fmt.Sprintf(
		"dependency %q matches %s; qualify it as %sproject-slug:short-id",
		value, strings.Join(labels, ", "), trigger,
	)
}

func (p *searchPredicates) setDestination(value string, projects []*models.Project, m *OutlinerModel) {
	compiled, ok := compileSearchDestination(value, projects, m)
	if !ok {
		return
	}
	p.destination = &compiled
	// Only a positive destination opens the terminal population. -$done is a
	// question about live work and must not drag every completed task into the
	// candidate set just to reject it.
	p.admitsTerminal = compiled.destination.WorkflowState().IsTerminal()
}

// compileSearchDestination resolves one destination and, for the two planning
// memberships, the identity set that answers it. A negation compiles through
// here too: "not on Today" needs the same membership the positive did, because
// Today and Next both store Backlog tasks and no state compare separates them.
func compileSearchDestination(
	value string,
	projects []*models.Project,
	m *OutlinerModel,
) (searchDestinationPredicate, bool) {
	destination, ok := textgrammar.ParseDestination(value)
	if !ok {
		return searchDestinationPredicate{}, false
	}
	compiled := searchDestinationPredicate{destination: destination}
	switch destination { //nolint:exhaustive // Only the two planning destinations resolve a membership set; the rest gate on state.
	case textgrammar.DestinationToday:
		compiled.todayIDs = m.resolveTodayIDs(projects)
	case textgrammar.DestinationNext:
		compiled.nextIDs = resolveSearchNextIDs(m, projects)
	}
	return compiled, true
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

// resolveSearchNextIDs reads Next membership from the durable plan, because Next
// and Today both store Backlog tasks and a state compare cannot tell them apart.
func resolveSearchNextIDs(m *OutlinerModel, projects []*models.Project) map[TaskIdentity]bool {
	ids := make(map[TaskIdentity]bool)
	if m == nil || m.runtime == nil {
		return ids
	}
	for _, item := range planningNextOrder(m.runtime) {
		for _, project := range projects {
			if project == nil || project.ID != item.ProjectID {
				continue
			}
			if task := project.GetTaskByID(item.TaskID); task != nil {
				ids[taskIdentityFor(task, project)] = true
			}
			break
		}
	}
	return ids
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
	day, err := ParseDate(value)
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

func (p *searchPredicates) invalid() bool {
	return p != nil && p.guidance != ""
}

// hasAnySigil reports whether the query carried a sigil at all. Sigils compile
// to an AND gate applied before scoring, and a note satisfies no conjunct of
// that gate — it has no priority, no state, no tags, no due date. So a query
// carrying one excludes the notes tier whole rather than exempting notes from
// the gate, which is the one move that would break the invariant every other
// search consumer relies on.
func (p *searchPredicates) hasAnySigil() bool {
	return p != nil && p.sigils
}

func (p *searchPredicates) guidanceText() string {
	if p == nil {
		return ""
	}
	return p.guidance
}

func (p *searchPredicates) allowsTerminal() bool {
	return p != nil && p.admitsTerminal
}

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

func (p *searchPredicates) admits(task *models.Task, project *models.Project) bool {
	if p == nil {
		return true
	}
	if task == nil {
		return false
	}
	if !p.admitsPriority(task) || !p.admitsPinned(task) {
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
