package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

var workflowTestCmd = &cobra.Command{
	Use:   "test [task-id]",
	Short: "Test which rules would match a task",
	Long: `Test which workflow rules would match a specific task.

This is a dry-run that shows which rules would trigger and what
actions would be taken, without actually modifying the task.

If no task ID is provided, tests all tasks in the current project.

Examples:
  tt workflow test 5       # Test rules against task #5
  tt workflow test abc123  # Test rules against task with UUID prefix`,
	RunE: runWorkflowTest,
}

func init() {
	workflowCmd.AddCommand(workflowTestCmd)
}

func runWorkflowTest(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		fmt.Println("No enabled workflow rules to test.")
		return nil
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	var tasks []*models.Task
	if len(args) > 0 {
		task, findErr := FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
		tasks = []*models.Task{task}
	} else {
		tasks = project.Tasks
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks to test.")
		return nil
	}

	matchCount := 0
	for _, task := range tasks {
		var matchingRules []struct {
			rule    workflow.WorkflowRule
			actions []string
		}

		for _, rule := range rules {
			matches, actions := workflow.TestRule(task, rule)
			if matches {
				matchingRules = append(matchingRules, struct {
					rule    workflow.WorkflowRule
					actions []string
				}{rule, actions})
			}
		}

		if len(matchingRules) > 0 {
			matchCount++
			fmt.Printf("#%d %s\n", task.ShortID, task.Title)
			for _, mr := range matchingRules {
				fmt.Printf("  -> %s:\n", mr.rule.Name)
				for _, action := range mr.actions {
					fmt.Printf("      %s\n", action)
				}
			}
			fmt.Println()
		}
	}

	if matchCount == 0 {
		fmt.Println("No rules match any tasks.")
	} else {
		fmt.Printf("%d task(s) would be affected by workflow rules.\n", matchCount)
	}

	return nil
}
