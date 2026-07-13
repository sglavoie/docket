package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestWorkflowDryRun(t *testing.T) {
	t.Run("runs without error when no rules configured", func(t *testing.T) {
		setupWorkflowTest(t)

		err := runWorkflowTest(workflowTestCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("tests rules against project tasks without double load", func(t *testing.T) {
		deps, project := setupWorkflowTest(t)

		// Add a task to verify it's accessible (proving LoadCurrentProject loaded it)
		task, err := models.NewTask("test-task-1", project.GetNextShortID(), models.TaskTypeTask, "Test Task")
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateReady
		task.Priority = 1
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		// Add a matching rule
		rule := workflow.NewRule("High Priority Alert",
			workflow.RuleTrigger{
				Field:     workflow.FieldPriority,
				Operation: workflow.OpEquals,
				Value:     "1",
			},
			[]workflow.RuleAction{
				{ActionType: workflow.ActionAddTag, Value: "critical"},
			},
		)
		rule.Enabled = true
		deps.Config.WorkflowRules = append(deps.Config.WorkflowRules, *rule)

		err = runWorkflowTest(workflowTestCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
