package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestEvaluateCriterion_SectionMatchesCustomSection(t *testing.T) {
	project := &models.Project{Name: "test"}

	gtdTask := &models.Task{
		Title:   "GTD task",
		Section: "Next",
	}
	customTask := &models.Task{
		Title:         "Custom task",
		CustomSection: "Design",
	}
	bothTask := &models.Task{
		Title:         "Both task",
		Section:       "Next",
		CustomSection: "Design",
	}

	tests := []struct {
		name string
		task *models.Task
		op   string
		val  string
		want bool
	}{
		{"GTD equals matches GTD task", gtdTask, "equals", "Next", true},
		{"GTD equals does not match custom task", customTask, "equals", "Next", false},
		{"custom equals matches custom task", customTask, "equals", "Design", true},
		{"custom equals does not match GTD task", gtdTask, "equals", "Design", false},
		{"custom equals matches task with both", bothTask, "equals", "Design", true},
		{"GTD equals matches task with both", bothTask, "equals", "Next", true},
		{"not_equals GTD on custom task", customTask, "not_equals", "Next", true},
		{"not_equals Design on custom task", customTask, "not_equals", "Design", false},
		{"contains partial custom section", customTask, "contains", "esig", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.SmartListCriterion{Field: "section", Operation: tt.op, Value: tt.val}
			got := evaluateCriterion(tt.task, project, c, nil)
			if got != tt.want {
				t.Errorf("evaluateCriterion(section %s %q) = %v, want %v", tt.op, tt.val, got, tt.want)
			}
		})
	}
}
