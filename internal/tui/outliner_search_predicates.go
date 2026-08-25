package tui

import (
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

// searchPredicates is the compiled sigil gate for one search query. It is
// deliberately built without a fuzzy.Matcher: the case-relaxation retry re-runs
// the scoring pass, and a gate that read the matcher could admit a different set
// on the second pass than on the first.
type searchPredicates struct {
	priority       int
	tags           []string
	excludedTags   []string
	contexts       []string
	due            searchDuePredicate
	deferral       searchDeferPredicate
	destination    textgrammar.Destination
	hasDestination bool
	pinned         int // 0 none, +1 *pin, -1 *unpin
	projectPrefix  string
	admitsTerminal bool
	todayIDs       map[TaskIdentity]bool
	nextIDs        map[TaskIdentity]bool
	now            time.Time
	guidance       string
}

// compileSearchQuery splits the live query into the sigil gate and the free-text
// remainder that keeps today's fuzzy ranking.
func (m *OutlinerModel) compileSearchQuery(projects []*models.Project) (string, *searchPredicates) {
	parsed := textgrammar.ParseSearch(strings.TrimSpace(m.SearchQuery()))
	predicates := &searchPredicates{now: time.Now()}
	if guidance := textgrammar.SearchGuidance(parsed); guidance != "" {
		predicates.guidance = guidance
		return parsed.Text, predicates
	}

	for _, token := range parsed.Tokens {
		switch token.Field { //nolint:exhaustive // The placement and retired fields refuse in SearchGuidance before this loop.
		case textgrammar.FieldPriority:
			if values, ok := textgrammar.ParsePriority(token.Value, false); ok && len(values) == 1 {
				predicates.priority = values[0]
			}
		case textgrammar.FieldTag:
			predicates.tags = append(predicates.tags, normalizeSearchTagPrefix(token.Value))
		case textgrammar.FieldRemoveTag:
			predicates.excludedTags = append(predicates.excludedTags, normalizeSearchTagPrefix(token.Value))
		case textgrammar.FieldContext:
			predicates.contexts = append(predicates.contexts, normalizeSearchTagPrefix(token.Value))
		case textgrammar.FieldDue:
			predicates.due = compileSearchDue(token.Value, predicates.now)
		case textgrammar.FieldDefer:
			predicates.deferral = compileSearchDefer(token.Value, predicates.now)
		case textgrammar.FieldDestination:
			predicates.setDestination(token.Value, projects, m)
		case textgrammar.FieldPinned:
			predicates.pinned = 1
			if token.Value != "pin" {
				predicates.pinned = -1
			}
		case textgrammar.FieldProject:
			predicates.projectPrefix = strings.ToLower(strings.TrimSpace(token.Value))
		}
	}
	return parsed.Text, predicates
}

func (p *searchPredicates) setDestination(value string, projects []*models.Project, m *OutlinerModel) {
	destination, ok := textgrammar.ParseDestination(value)
	if !ok {
		return
	}
	p.destination = destination
	p.hasDestination = true
	p.admitsTerminal = destination.WorkflowState().IsTerminal()
	switch destination { //nolint:exhaustive // Only the two planning destinations resolve a membership set; the rest gate on state.
	case textgrammar.DestinationToday:
		p.todayIDs = m.resolveTodayIDs(projects)
	case textgrammar.DestinationNext:
		p.nextIDs = resolveSearchNextIDs(m, projects)
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

func (p *searchPredicates) guidanceText() string {
	if p == nil {
		return ""
	}
	return p.guidance
}

func (p *searchPredicates) allowsTerminal() bool {
	return p != nil && p.admitsTerminal
}

func (p *searchPredicates) admitsProject(project *models.Project) bool {
	if p == nil || p.projectPrefix == "" {
		return true
	}
	if project == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(project.Slug), p.projectPrefix) ||
		strings.HasPrefix(strings.ToLower(project.Name), p.projectPrefix)
}

func (p *searchPredicates) admits(task *models.Task, project *models.Project) bool {
	if p == nil {
		return true
	}
	if task == nil {
		return false
	}
	if p.priority > 0 && models.EffectivePriority(task.Priority) != p.priority {
		return false
	}
	if p.pinned > 0 && !task.Pinned {
		return false
	}
	if p.pinned < 0 && task.Pinned {
		return false
	}
	if !p.admitsTags(task) || !p.admitsContext(task) || !p.admitsDue(task) || !p.admitsDeferral(task) {
		return false
	}
	return p.admitsDestination(task, project)
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

func (p *searchPredicates) admitsContext(task *models.Task) bool {
	if len(p.contexts) == 0 {
		return true
	}
	context, err := models.NormalizeTaskContext(task.Context)
	if err != nil {
		return false
	}
	for _, prefix := range p.contexts {
		if !strings.HasPrefix(context, prefix) {
			return false
		}
	}
	return true
}

func (p *searchPredicates) admitsDue(task *models.Task) bool {
	switch {
	case p.due.hasBucket:
		return passesDueDateFilterAt(task, p.due.bucket, p.now)
	case p.due.hasHourWindow:
		return task.DueDate != nil && !task.DueDate.After(p.due.window)
	case p.due.hasDay:
		return task.DueDate != nil && util.DaysBetween(p.due.day, *task.DueDate) == 0
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
	switch {
	case p.deferral.presence > 0:
		return task.IsDeferredAt(p.now)
	case p.deferral.presence < 0:
		return !task.IsDeferredAt(p.now)
	case p.deferral.hasDay:
		return task.DeferredUntil != nil && util.DaysBetween(p.deferral.day, *task.DeferredUntil) == 0
	default:
		return true
	}
}

func (p *searchPredicates) admitsDestination(task *models.Task, project *models.Project) bool {
	if !p.hasDestination {
		return true
	}
	identity := taskIdentityFor(task, project)
	switch p.destination {
	case textgrammar.DestinationToday:
		return p.todayIDs[identity]
	case textgrammar.DestinationNext:
		return p.nextIDs[identity]
	default:
		return task.State() == p.destination.WorkflowState()
	}
}

// taskCarriesTagPrefix matches over normalized tag names only. The f# facet's
// taskMatchesTagFacet deliberately conflates tags and contexts; search keeps the
// two axes apart because it offers a separate sigil for each.
func taskCarriesTagPrefix(task *models.Task, prefix string) bool {
	for _, raw := range task.Tags {
		tag, err := models.NormalizeTaskTag(raw)
		if err == nil && strings.HasPrefix(tag, prefix) {
			return true
		}
	}
	return false
}
