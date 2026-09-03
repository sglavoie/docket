package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// compileSearchQuery hands the Outliner's own carriers to the shared gate. The
// gate reads two things this model owns and nothing else does: the durable plan
// that decides Today and Next membership, and the canonical filter query whose
// facets open the terminal and deferred populations beside the sigils.
//
// A nil model still compiles: the sigils answer for themselves, and a view with
// no plan and no facets neither names a planned task nor opens a population.
func (m *OutlinerModel) compileSearchQuery(text string, projects []*models.Project) (string, *query.SearchPredicates) {
	if m == nil {
		return query.CompileSearchQuery(text, projects, nil, models.Query{})
	}
	return query.CompileSearchQuery(text, projects, outlinerPlanningMembership{outliner: m}, m.filterQuery)
}

// outlinerPlanningMembership is the Outliner's answer to the gate's planning
// question. It is a wrapper rather than a set of methods on the model itself so
// the gate's contract stays the two memberships it names, not whatever else the
// Outliner happens to expose.
type outlinerPlanningMembership struct {
	outliner *OutlinerModel
}

func (p outlinerPlanningMembership) TodayIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool {
	return p.outliner.resolveTodayIDs(projects)
}

func (p outlinerPlanningMembership) NextIdentities(projects []*models.Project) map[taskref.TaskIdentity]bool {
	return resolveSearchNextIDs(p.outliner, projects)
}

// resolveSearchNextIDs reads Next membership from the durable plan, because Next
// and Today both store Backlog tasks and a state compare cannot tell them apart.
func resolveSearchNextIDs(m *OutlinerModel, projects []*models.Project) map[taskref.TaskIdentity]bool {
	ids := make(map[taskref.TaskIdentity]bool)
	if m == nil || m.runtime == nil {
		return ids
	}
	for _, item := range planningNextOrder(m.runtime) {
		for _, project := range projects {
			if project == nil || project.ID != item.ProjectID {
				continue
			}
			if task := project.GetTaskByID(item.TaskID); task != nil {
				ids[taskref.For(task, project)] = true
			}
			break
		}
	}
	return ids
}
