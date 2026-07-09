package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestEvaluateCriterion_SectionMatchesGTDOnly(t *testing.T) {
	project := &models.Project{Name: "test"}

	gtdTask := &models.Task{
		Title:   "GTD task",
		Section: "Next",
	}
	activeTask := &models.Task{
		Title: "Active task",
	}
	bothTask := &models.Task{
		Title:   "Both task",
		Section: "Next",
	}

	tests := []struct {
		name string
		task *models.Task
		op   string
		val  string
		want bool
	}{
		{"GTD equals matches GTD task", gtdTask, "equals", "Next", true},
		{"GTD equals does not match empty task", activeTask, "equals", "Next", false},
		{"non-GTD equals does not match empty task", activeTask, "equals", "Design", false},
		{"non-GTD equals does not match GTD task", gtdTask, "equals", "Design", false},
		{"non-GTD equals does not match task with GTD section", bothTask, "equals", "Design", false},
		{"GTD equals matches task with both", bothTask, "equals", "Next", true},
		{"not_equals GTD on empty task", activeTask, "not_equals", "Next", true},
		{"not_equals Design on empty task", activeTask, "not_equals", "Design", true},
		{"contains partial non-GTD section does not match", activeTask, "contains", "esig", false},
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

func TestEvaluateCriterion_Parked(t *testing.T) {
	project := &models.Project{Name: "test"}
	parkedTask := &models.Task{Title: "Parked", ParkedNote: "resume after review"}
	activeTask := &models.Task{Title: "Active"}

	tests := []struct {
		name string
		task *models.Task
		val  string
		want bool
	}{
		{"parked true matches parked task", parkedTask, "true", true},
		{"parked true rejects active task", activeTask, "true", false},
		{"parked false rejects parked task", parkedTask, "false", false},
		{"parked false matches active task", activeTask, "false", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.SmartListCriterion{Field: "parked", Operation: "equals", Value: tt.val}
			got := evaluateCriterion(tt.task, project, c, nil)
			if got != tt.want {
				t.Errorf("evaluateCriterion(parked equals %q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}
