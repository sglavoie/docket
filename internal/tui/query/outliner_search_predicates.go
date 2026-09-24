package query

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
	todayIDs    map[taskref.TaskIdentity]bool
	nextIDs     map[taskref.TaskIdentity]bool
}

// searchDerivedContext holds the two censuses a derived predicate needs and a
// task cannot answer alone. Both are computed once per query, and only when a
// predicate that reads them is present, so an ordinary search walks no extra
// tree and scans no other project.
type searchDerivedContext struct {
	openChildren map[taskref.TaskIdentity]int
	dependedOn   map[models.TaskReference]bool
}

// SearchPredicates is the compiled sigil gate for one search query. It is
// deliberately built without a fuzzy.Matcher: the case-relaxation retry re-runs
// the scoring pass, and a gate that read the matcher could admit a different set
// on the second pass than on the first.
//
// Every axis holds its positives and its negatives apart, because they join
// differently: positives narrow by their own field's rule, while a negative is
// always an AND-of-nots. Repeated negations therefore accumulate on every axis,
// including the ones whose positive form is a singleton — "not today and not
// tomorrow" is one coherent question about a single-valued field.
type SearchPredicates struct {
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
	requireReview      bool
	refuseReview       bool
	requireAttachments bool
	refuseAttachments  bool
	projectPrefixes    []string
	excludedProjects   []string
	derived            []textgrammar.DerivedPredicate
	excludedDerived    []textgrammar.DerivedPredicate
	derivedContext     searchDerivedContext
	// admitsTerminal and admitsDeferred are the two population answers, decided
	// once by compileSearchQuery from the one query-says-so rule and never
	// patched afterwards by the carrier that compiled the query.
	admitsTerminal bool
	admitsDeferred bool
	now            time.Time
	guidance       string
	// sigils records that the query carried at least one sigil token. It is
	// read off the parse rather than off the compiled conjuncts so that a
	// destination or pin sigil, which compiles into a field rather than into a
	// slice, counts the same as a tag. A sigil the parser refuses outright
	// never reaches this flag; that query is already invalid().
	sigils bool
}

// PlanningMembership answers, for one loaded population, which tasks each of the
// two planning destinations names. Today and Next both store Backlog tasks, so
// no state compare separates them and a compiled $today or $next has to read the
// durable plan. The plan belongs to the projection above this gate, so the gate
// asks it the question rather than holding the model that answers it.
type PlanningMembership interface {
	TodayIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool
	NextIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool
}

// compileSearchQuery splits one query into the sigil gate and the free-text
// remainder that keeps today's fuzzy ranking. The query arrives explicitly
// rather than being read off the search input, because a lens narrows with its
// own saved query and must compile it through this same grammar: one parser,
// one predicate set, one remainder.
//
// Terminal and deferred admission are settled here and nowhere else. The sigils
// answer for themselves as they compile, the facets arrive as the caller's
// canonical filter query, and the two halves are ORed once: a population is on
// screen when the query says so, whichever carrier spelled it. No caller may
// reopen a population by writing to the compiled gate afterwards.
func CompileSearchQuery(
	text string,
	projects []*models.Project,
	plan PlanningMembership,
	facets models.Query,
) (string, *SearchPredicates) {
	parsed := textgrammar.ParseSearch(strings.TrimSpace(text))
	predicates := &SearchPredicates{now: time.Now(), sigils: len(parsed.Tokens) > 0}
	if guidance := textgrammar.SearchGuidance(parsed); guidance != "" {
		predicates.guidance = guidance
		return parsed.Text, predicates
	}

	for _, token := range parsed.Tokens {
		predicates.applyToken(token, projects, plan)
	}
	predicates.compileDerivedContext(projects)
	predicates.admitsTerminal = predicates.admitsTerminal || facets.OpensTerminalPopulation()
	predicates.admitsDeferred = predicates.admitsDeferred || facets.OpensDeferredPopulation()
	return parsed.Text, predicates
}

// applyToken compiles one token onto the gate. A negation is never a separate
// compilation path: it reads the same value the positive would have, and only
// the slice it lands in differs, which is what keeps -@friday and @friday
// naming the same day.
func (p *SearchPredicates) applyToken(token textgrammar.Token, projects []*models.Project, plan PlanningMembership) {
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
		// @@any and @@day ask about work the base population hides behind a live
		// deferral, so they open it the way $done opens the terminal one. @@none
		// is a question about work already on screen and opens nothing.
		p.admitsDeferred = p.deferral.presence > 0 || p.deferral.hasDay
	case textgrammar.FieldRemoveDefer:
		p.excludedDeferral = append(p.excludedDeferral, compileSearchDefer(token.Value, p.now))
	case textgrammar.FieldDestination:
		p.setDestination(token.Value, projects, plan)
	case textgrammar.FieldRemoveDestination:
		if compiled, ok := compileSearchDestination(token.Value, projects, plan); ok {
			p.excludedDests = append(p.excludedDests, compiled)
		}
	case textgrammar.FieldProject:
		p.projectPrefixes = append(p.projectPrefixes, strings.ToLower(strings.TrimSpace(token.Value)))
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
	default:
		p.applyFlagToken(token)
	}
}

// applyFlagToken compiles the three stored boolean axes — pinned, review, and
// attachment presence — each of which either requires or refuses the flag.
func (p *SearchPredicates) applyFlagToken(token textgrammar.Token) {
	switch token.Field { //nolint:exhaustive // applyToken answers every other field.
	case textgrammar.FieldPinned:
		if token.Value == "pin" {
			p.requirePinned = true
			return
		}
		p.refusePinned = true
	case textgrammar.FieldRemovePinned:
		p.refusePinned = true
	case textgrammar.FieldReview:
		if token.Value == "review" {
			p.requireReview = true
			return
		}
		p.refuseReview = true
	case textgrammar.FieldRemoveReview:
		p.refuseReview = true
	case textgrammar.FieldAttachment, textgrammar.FieldRemoveAttachment:
		// -& reads its twin's pair inverted, so -&none asks what &any does.
		present := strings.EqualFold(strings.TrimSpace(token.Value), "any")
		if token.Field == textgrammar.FieldRemoveAttachment {
			present = !present
		}
		if present {
			p.requireAttachments = true
			return
		}
		p.refuseAttachments = true
	}
}

func (p *SearchPredicates) Invalid() bool {
	return p != nil && p.guidance != ""
}

// HasAnySigil reports whether the query carried a sigil at all. Sigils compile
// to an AND gate applied before scoring, and a note satisfies no conjunct of
// that gate — it has no priority, no state, no tags, no due date. So a query
// carrying one excludes the notes tier whole rather than exempting notes from
// the gate, which is the one move that would break the invariant every other
// search consumer relies on.
func (p *SearchPredicates) HasAnySigil() bool {
	return p != nil && p.sigils
}

func (p *SearchPredicates) GuidanceText() string {
	if p == nil {
		return ""
	}
	return p.guidance
}

func (p *SearchPredicates) AllowsTerminal() bool {
	return p != nil && p.admitsTerminal
}

func (p *SearchPredicates) AllowsDeferred() bool {
	return p != nil && p.admitsDeferred
}

// CompiledDestination names the one positive $destination the query compiled.
// The second result is what distinguishes "no destination sigil" from one whose
// zero value happens to be a legal destination, so a caller never has to read a
// destination it was not given.
//
// It exists for the cross-surface parity check: capture, edit and search all
// read the same $ vocabulary, and the only way to assert they agree is to ask
// each surface what it compiled.
func (p *SearchPredicates) CompiledDestination() (textgrammar.Destination, bool) {
	if p == nil || p.destination == nil {
		return "", false
	}
	return p.destination.destination, true
}

// DestinationResolvedMembership reports whether the compiled destination read a
// planning membership set rather than answering from the task's own state.
// Today and Next are the two destinations that do; every other one is a state,
// so a true here beside any other destination would be the bug.
func (p *SearchPredicates) DestinationResolvedMembership() bool {
	if p == nil || p.destination == nil {
		return false
	}
	return p.destination.todayIDs != nil || p.destination.nextIDs != nil
}
