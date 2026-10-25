package engine

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/util"
)

// recentWindowDays is how far back the Recent mode reads activity.
const recentWindowDays = 30

// QueryItem is one admitted task, qualified by the project that owns it, with
// the planning memberships a frontend needs to label it. The query result types
// carry the bridge contract's snake_case field names, since a headless
// frontend receives them serialized.
type QueryItem struct {
	Project *models.Project `json:"project"`
	Task    *models.Task    `json:"task"`
	Today   bool            `json:"today"`
	Next    bool            `json:"next"`
}

// Locator is the item's project-qualified identity, the form every mutation
// addresses a task by.
func (i QueryItem) Locator() app.TaskLocator {
	return app.TaskLocator{ProjectID: i.Project.ID, TaskID: i.Task.ID}
}

// QueryDiagnostic names a project the population holds but could not read. The
// query still answers for every other project, as a mutation snapshot does.
type QueryDiagnostic struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	Detail      string `json:"detail"`
}

// QueryResult is a query's answer: the canonical query it evaluated, the
// admitted tasks in order, and the projects it could not read.
type QueryResult struct {
	Query       models.Query      `json:"query"`
	Items       []QueryItem       `json:"items"`
	Diagnostics []QueryDiagnostic `json:"diagnostics"`
}

// Query evaluates one canonical query against the workspace as it stands on
// disk. It answers every half of the query rather than the per-task clauses
// alone:
//
//   - population: the base task workspace is non-archived and not parked; a
//     positive archived or parked clause loads that population too, and Scope
//     narrows loading to one project;
//   - lifecycle: done and canceled work stays out unless the query opens the
//     terminal population, and undeferred work unless it opens the deferred one
//     or the task carries its manual review flag, and effectively hidden work
//     (the task or an ancestor flagged) unless a positive hidden clause opens
//     it, in every mode including Recent;
//   - census: parent, leaf, child, blocked, and blocking are answered against
//     the loaded population's census, never a single task;
//   - mode: Today and Next are the planning collections, Inbox and Backlog the
//     lifecycle lanes (Backlog leaving Today's members to Today), Due the dated
//     work, Recent everything touched in the last 30 days including terminal and
//     deferred work.
//
// The order is the mode's: priority first everywhere but Due (deadline first)
// and Recent (newest touch first), with planning order or stored order breaking
// ties. Manual sort uses those orders without the primary key. A named field
// Sort replaces the primary key. Sections, hierarchy, and folds
// are presentation and stay with the frontend.
func (r *Runtime) Query(query models.Query) (QueryResult, error) {
	if err := query.Validate(); err != nil {
		return QueryResult{}, err
	}
	query = query.Canonical()
	if err := r.Refresh(); err != nil {
		return QueryResult{}, fmt.Errorf("reload workspace domains: %w", err)
	}
	now := r.Now()
	population, err := r.queryPopulation(query)
	if err != nil {
		return QueryResult{}, err
	}
	result := QueryResult{Query: query}
	loaded := make([]*models.Project, 0, len(population))
	for _, project := range population {
		if loadErr := storage.LoadProject(project); loadErr != nil {
			result.Diagnostics = append(result.Diagnostics, QueryDiagnostic{
				ProjectID: project.ID, ProjectName: project.Name, Detail: loadErr.Error(),
			})
			continue
		}
		loaded = append(loaded, project)
	}

	census := models.NewQueryCensus(loaded)
	today := planningRanks(r.state.Planning().GetToday())
	next := planningRanks(r.state.NextOrder())
	recent := query.Mode == models.QueryModeRecent
	admitsTerminal := recent || query.OpensTerminalPopulation()
	admitsDeferred := recent || query.OpensDeferredPopulation()
	admitsHidden := query.OpensHiddenPopulation()

	type candidate struct {
		item  QueryItem
		order int
	}
	var candidates []candidate
	for _, project := range loaded {
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			key := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			_, inToday := today[key]
			_, inNext := next[key]
			if task.State().IsTerminal() && !admitsTerminal {
				continue
			}
			if task.IsDeferredAt(now) && !admitsDeferred && !task.Review {
				continue
			}
			if !admitsHidden && models.TaskEffectivelyHidden(project, task) {
				continue
			}
			if !modeAdmits(query.Mode, task, inToday, inNext, now) {
				continue
			}
			if !query.AdmitsInPopulation(task, project, census, now) {
				continue
			}
			candidates = append(candidates, candidate{
				item:  QueryItem{Project: project, Task: task, Today: inToday, Next: inNext},
				order: len(candidates),
			})
		}
	}

	slices.SortStableFunc(candidates, func(left, right candidate) int {
		if query.Sort.Field != models.QuerySortDefault {
			if c := models.CompareTasksBySortField(left.item.Task, right.item.Task, query.Sort.Field); c != 0 {
				if query.Sort.Ascending {
					return c
				}
				return -c
			}
		} else if c := modeLead(query.Mode, left.item.Task, right.item.Task); c != 0 {
			return c
		}
		if c := cmp.Compare(planningTieRank(query.Mode, left.item, today, next), planningTieRank(query.Mode, right.item, today, next)); c != 0 {
			return c
		}
		return cmp.Compare(left.order, right.order)
	})
	result.Items = make([]QueryItem, len(candidates))
	for index, candidate := range candidates {
		result.Items[index] = candidate.item
	}
	return result, nil
}

// queryPopulation is the set of projects the query loads, in catalog order.
func (r *Runtime) queryPopulation(query models.Query) ([]*models.Project, error) {
	includeArchived := opensPopulation(query, models.QueryFieldArchived)
	includeParked := opensPopulation(query, models.QueryFieldParked)
	projects := r.state.AvailableProjects()
	if includeArchived {
		projects = append(projects, r.state.ArchivedProjects()...)
	}
	population := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		if !includeParked && models.ProjectTagsParked(project.Tags) {
			continue
		}
		if query.Scope != "" && project.ID != query.Scope {
			continue
		}
		population = append(population, project)
	}
	if query.Scope != "" && len(population) == 0 {
		return nil, fmt.Errorf("%w: %s", app.ErrProjectNotFound, query.Scope)
	}
	return population, nil
}

// opensPopulation reports whether a positive population clause names field.
// Include and only both load it; only is then a row narrowing Admits applies.
func opensPopulation(query models.Query, field models.QueryField) bool {
	return slices.ContainsFunc(query.Clauses, func(clause models.QueryClause) bool {
		return clause.Field == field && clause.Polarity == models.QueryPolarityPositive
	})
}

func planningRanks(items []config.ProjectTaskLocator) map[app.TaskLocator]int {
	ranks := make(map[app.TaskLocator]int, len(items))
	for index, item := range items {
		key := app.TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		if _, seen := ranks[key]; !seen {
			ranks[key] = index
		}
	}
	return ranks
}

func modeAdmits(mode models.QueryMode, task *models.Task, inToday, inNext bool, now time.Time) bool {
	switch mode {
	case models.QueryModeInbox:
		return task.IsInState(models.StateInbox)
	case models.QueryModeBacklog:
		return task.IsInState(models.StateBacklog) && !inToday
	case models.QueryModeToday:
		return inToday
	case models.QueryModeNext:
		return inNext
	case models.QueryModeDue:
		return task.DueDate != nil
	case models.QueryModeRecent:
		return util.DaysBetween(task.UpdatedAt.In(now.Location()), now) <= recentWindowDays
	default:
		return true
	}
}

// modeLead is the mode's primary key when the query names no sort.
func modeLead(mode models.QueryMode, left, right *models.Task) int {
	switch mode {
	case models.QueryModeDue:
		if c := models.CompareTasksBySortField(left, right, models.QuerySortDue); c != 0 {
			return c
		}
	case models.QueryModeRecent:
		if c := right.UpdatedAt.Compare(left.UpdatedAt); c != 0 {
			return c
		}
	default:
	}
	return models.CompareTasksBySortField(left, right, models.QuerySortPriority)
}

// planningTieRank is the planning order that breaks a tie in Today and Next.
// Every other mode breaks ties by stored order alone.
func planningTieRank(mode models.QueryMode, item QueryItem, today, next map[app.TaskLocator]int) int {
	switch mode {
	case models.QueryModeToday:
		return today[item.Locator()]
	case models.QueryModeNext:
		return next[item.Locator()]
	default:
		return 0
	}
}
