package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
)

func TestRoadmapCriticalPathRendersDuplicateTaskSlackByProject(t *testing.T) {
	alpha := &models.Project{ID: "alpha"}
	beta := &models.Project{ID: "beta"}
	alphaTask := &models.Task{ID: "shared-id", Title: "Alpha task", Project: alpha}
	betaTask := &models.Task{ID: "shared-id", Title: "Beta task", Project: beta}

	m := &RoadmapModel{
		cursor: -1,
		criticalPath: &planner.CriticalPathResult{
			Path: []*models.Task{alphaTask, betaTask},
			Slack: map[string]float64{
				planner.TaskKey(alphaTask): 4,
				planner.TaskKey(betaTask):  7,
			},
		},
	}

	rendered := m.renderCriticalPathTab(10)
	for _, want := range []string{"4.0d", "7.0d"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("critical path render missing qualified slack %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "0.0d") {
		t.Fatalf("duplicate task UUID fell back to zero slack and critical styling:\n%s", rendered)
	}
}
