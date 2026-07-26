package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestEvaluateCriterion_StateSet(t *testing.T) {
	project := &models.Project{Name: "test"}

	gtdTask := &models.Task{
		Title:         "GTD task",
		WorkflowState: models.StateNext,
	}
	activeTask := &models.Task{
		Title: "Active task",
	}
	bothTask := &models.Task{
		Title:         "Both task",
		WorkflowState: models.StateNext,
	}

	tests := []struct {
		name string
		task *models.Task
		op   string
		want bool
	}{
		{"in matches Next task", gtdTask, "in", true},
		{"in rejects Ready task", activeTask, "in", false},
		{"in matches another Next task", bothTask, "in", true},
		{"not_in rejects Next task", gtdTask, "not_in", false},
		{"not_in matches Ready task", activeTask, "not_in", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.SmartListCriterion{
				Field:     "state",
				Operation: tt.op,
				States:    testWorkflowStateSet(models.StateNext),
			}
			got := evaluateCriterion(tt.task, project, c, nil)
			if got != tt.want {
				t.Errorf("evaluateCriterion(state %s Next) = %v, want %v", tt.op, got, tt.want)
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
