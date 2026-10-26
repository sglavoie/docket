package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// TestCrossProjectsLoadedRebuildsOnlyForANewPopulation pins the republish
// rule: a read answering with exactly the population the rows were built from
// forces nothing, while new task data or a changed catalog field still does.
func TestCrossProjectsLoadedRebuildsOnlyForANewPopulation(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	generation := outlinerView.Population.LoadGen

	revision := outlinerView.Population.revision
	landCrossProjects(outlinerView, generation, alpha, beta)
	if outlinerView.Population.revision != revision {
		t.Fatal("republishing the population the rows were built from forced a rebuild")
	}

	retitled := models.CloneTaskDeep(alpha.Tasks[0])
	retitled.Title = "Wire the exporter"
	alpha.Tasks = append([]*models.Task{retitled}, alpha.Tasks[1:]...)
	alpha.BuildTaskTree()
	landCrossProjects(outlinerView, generation, alpha, beta)
	if outlinerView.Population.revision == revision {
		t.Fatal("a reloaded task slice did not force a rebuild")
	}
	row, ok := findPlanningTaskRow(outlinerView, retitled.ID)
	if !ok || row.Task.Title != "Wire the exporter" {
		t.Fatal("the rebuild did not draw the reloaded task")
	}

	revision = outlinerView.Population.revision
	beta.Name = "Beta renamed"
	landCrossProjects(outlinerView, generation, alpha, beta)
	if outlinerView.Population.revision == revision {
		t.Fatal("a renamed project over the same tasks did not force a rebuild")
	}

	revision = outlinerView.Population.revision
	beta.Tags = slices.Concat(beta.Tags, []string{"client"})
	landCrossProjects(outlinerView, generation, alpha, beta)
	if outlinerView.Population.revision == revision {
		t.Fatal("a retagged project over the same tasks did not force a rebuild")
	}
}
