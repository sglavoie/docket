package outliner

import (
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
