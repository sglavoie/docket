package outliner

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// adoptFilterCarriers installs the predicate a test's hand-authored carriers
// spell. Production authors the query and derives the carriers from it, so this
// direction exists only here: it is how a test that wrote a carrier field gets
// the model into the state a facet key would have left it in.
//
// It is deliberately not a model method. The lift cannot express a clause's
// polarity or hold both dependency directions, so a production caller would be a
// second, lossy way to author a predicate — which is exactly what a restoration
// point stopped doing.
func adoptFilterCarriers(m *OutlinerModel) {
	if m == nil {
		return
	}
	clauses := query.CommonFilterClauses(&m.Filter.CommonFilterState)
	clauses = append(clauses, query.ProjectTagClauses(&m.ProjectFilter)...)
	clauses = append(clauses, query.PopulationClauses(models.QueryFieldParked, m.ProjectFilter.ParkedMode)...)
	clauses = append(clauses, query.PopulationClauses(models.QueryFieldArchived, m.ProjectFilter.ArchivedMode)...)
	m.SetFilterQuery(models.Query{Clauses: clauses})
}

func newCrossOutlinerFromEnv(env *crossProjectTestEnv) *OutlinerModel {
	runtime := newTestConfigAt(env.dataDir)
	setTestProjects(runtime, env.projects)
	outliner := &OutlinerModel{
		Runtime:   testRuntimeAt(runtime.DataDir(), runtime),
		QuickEdit: grammar.NewQuickEditComponent(),
		Folds: foldState{
			CollapsedSections: make(map[string]bool),
			ExpandedNodes:     make(map[taskref.TaskIdentity]bool),
		},
		MultiSelectState: listsurface.MultiSelectState{
			Selected: make(map[taskref.TaskIdentity]bool),
		},
		Population: populationState{Projects: env.projects},
	}
	outliner.taskItem = env.taskItem
	return outliner
}

func countTaskRows(outliner *OutlinerModel, taskID string) int {
	count := 0
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			count++
		}
	}
	return count
}

func searchMatchTaskIDs(matches []outlinerSearchMatch) []string {
	result := make([]string, len(matches))
	for i, match := range matches {
		result[i] = match.task.ID
	}
	return result
}
