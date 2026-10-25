package outliner

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/query"
)

// searchACOutliner builds a scoped Outliner already in search mode, which is the
// only state the suggestion pane can exist in.
func searchACOutliner(projects ...*models.Project) *OutlinerModel {
	outliner := scopeTestOutliner(projects...)
	outliner.SetSize(120, 40)
	outliner.ActivateSearch()
	return outliner
}

// typeSearchAC puts a whole query in the box with the caret at its end, which is
// the ordinary typing case.
func typeSearchAC(outliner *OutlinerModel, value string) {
	typeSearchACAt(outliner, value, utf8.RuneCountInString(value))
}

func searchACValues(outliner *OutlinerModel) []string {
	values := make([]string, 0, len(outliner.Search.Autocomplete.Ac.Items))
	for _, item := range outliner.Search.Autocomplete.Ac.Items {
		values = append(values, item.Value)
	}
	return values
}

func searchACTask(id string, tags []string, context string) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.Title = id
	task.Tags = tags
	task.Context = context
	return task
}

// typeSearchACAt is the mid-query case: the caret lands on a rune index inside a
// settled query rather than after it.
func typeSearchACAt(outliner *OutlinerModel, value string, runeCursor int) {
	outliner.SetSearchInputValue(value)
	outliner.SetSearchInputCursor(runeCursor)
	outliner.RefreshSearchAutocomplete()
}

// cueTask is a bland task: nothing in it renders a `·` of its own, so any middle
// dot in the row came from the match cue.
func cueTask() *models.Task {
	return &models.Task{
		ID:            "cue",
		ShortID:       1,
		Title:         "Untitled row",
		WorkflowState: models.StateBacklog,
	}
}

// searchGateProjects builds one two-project scope whose task order is the source
// order a sigil-only query must preserve.
func searchGateProjects() (*models.Project, *models.Project) {
	today := time.Now()
	later := today.AddDate(0, 0, 3)

	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Title = "Pin candidate"
	pinned.Pinned = true
	pinned.Tags = []string{"backend"}
	priorityOne := primaryRowsTask("priority-one", models.StateBacklog)
	priorityOne.Title = "Ship auth"
	priorityOne.Priority = models.PriorityHigh
	priorityOne.DueDate = &today
	contexted := primaryRowsTask("contexted", models.StateBacklog)
	contexted.Title = "Client work"
	contexted.Context = "client-acme"
	contexted.DueDate = &later
	plannedToday := primaryRowsTask("planned-today", models.StateBacklog)
	plannedToday.Title = "Planned today"
	plannedNext := primaryRowsTask("planned-next", models.StateBacklog)
	plannedNext.Title = "Planned next"
	finished := primaryRowsTask("finished", models.StateDone)
	finished.Title = "Old auth work"
	alpha := primaryRowsProject("alpha", pinned, priorityOne, contexted, plannedToday, plannedNext, finished)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta auth"
	betaTask.Tags = []string{"backend"}
	beta := primaryRowsProject("beta", betaTask)

	return alpha, beta
}

func searchGateOutliner() *OutlinerModel {
	alpha, beta := searchGateProjects()
	outliner := scopeTestOutliner(alpha, beta)
	outliner.Runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-today"},
	})
	outliner.Runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-next"},
	})
	return outliner
}

// searchHistoryOutliner builds a scoped Outliner with the slash input open,
// which is the only state the walk exists in.
func searchHistoryOutliner() *OutlinerModel {
	outliner := scopeTestOutliner(scopeTestProject("work", "Work", "alpha", "beta"))
	outliner.SetSize(120, 40)
	outliner.ActivateSearch()
	return outliner
}

// searchRankedList holds a query and shows it as the flat ranked list. The tier,
// score, and seating assertions are all about that presentation; a query alone
// narrows the mode's own lanes instead, which the D7 tests cover separately.
func searchRankedList(outliner *OutlinerModel, query string) {
	outliner.SetSearchQuery(query)
	if !outliner.ShowsRankedList() {
		outliner.ToggleRankedList()
	}
}

func searchVisibleTaskIDs(outliner *OutlinerModel) []string {
	result := make([]string, 0)
	for _, identity := range qualifiedVisibleTaskIDs(outliner) {
		result = append(result, identity.TaskID)
	}
	return result
}

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
// It resets by replacement rather than field by field: the armed markers behind
// the state and completion-age facets are the carrier's own, so a fresh carrier
// is the only spelling of "no facet at all" that cannot leave one of them set.

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
// It resets by replacement rather than field by field: the armed markers behind
// the state and completion-age facets are the carrier's own, so a fresh carrier
// is the only spelling of "no facet at all" that cannot leave one of them set.
func clearCommonFilters(f *query.CommonFilterState) {
	*f = query.NewCommonFilterState()
}

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.
func passesCommonFiltersAt(task *models.Task, filter *query.CommonFilterState, now time.Time) bool {
	predicate := query.WithoutFields(
		models.Query{Clauses: query.CommonFilterClauses(filter)},
		query.FilterQueryHierarchyFields...,
	)
	return predicate.Admits(task, query.EmptyFilterQueryProject, now)
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func grammarLiteralsInNode(node ast.Node, triggers, destinations map[string]bool) (map[string]bool, map[string]bool) {
	foundTriggers := make(map[string]bool)
	foundDestinations := make(map[string]bool)
	ast.Inspect(node, func(child ast.Node) bool {
		literal, ok := child.(*ast.BasicLit)
		if !ok || (literal.Kind != token.STRING && literal.Kind != token.CHAR) {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		if triggers[value] {
			foundTriggers[value] = true
		}
		if destinations[value] {
			foundDestinations[value] = true
		}
		return true
	})
	return foundTriggers, foundDestinations
}

func inlineFormTriggerCount(literal string, triggers map[string]bool) int {
	found := make(map[string]bool)
	for _, word := range strings.Fields(literal) {
		normalized := strings.Trim(word, "\t\n\r ,.;:!?()[]{}\"'")
		longest := ""
		for trigger := range triggers {
			if trigger == "%" && isFormattingDirective(normalized) {
				continue
			}
			if len(normalized) > len(trigger) && len(trigger) > len(longest) && strings.HasPrefix(normalized, trigger) {
				longest = trigger
			}
		}
		if longest != "" {
			found[longest] = true
		}
	}
	return len(found)
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}
