package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskSectionCmd = &cobra.Command{
	Use:   "section <task-id> <section>",
	Short: "Assign a task to a section",
	Long: `Assign a task to a section for organization.

Use an empty string "" to clear the section assignment.

Examples:
  tt task section 5 "In Progress"
  tt task section 12 Review
  tt task section 3 ""`,
	Args: cobra.ExactArgs(2),
	RunE: runTaskSection,
}

func init() {
	taskCmd.AddCommand(taskSectionCmd)
}

func runTaskSection(cmd *cobra.Command, args []string) error {
	taskID := args[0]
	sectionName := args[1]

	var oldSection string

	task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
		// If section name is provided and non-empty, ensure it exists
		if sectionName != "" {
			project.EnsureSection(sectionName)
		}

		oldSection = task.Section
		task.Section = sectionName
		return nil
	})

	if err != nil {
		return err
	}

	// Output feedback
	if sectionName == "" {
		if oldSection != "" {
			fmt.Printf("Cleared section from task #%d (was: %s)\n", task.ShortID, oldSection)
		} else {
			fmt.Printf("Task #%d has no section\n", task.ShortID)
		}
	} else if oldSection == "" {
		fmt.Printf("Assigned task #%d to section: %s\n", task.ShortID, sectionName)
	} else {
		fmt.Printf("Moved task #%d from '%s' to '%s'\n", task.ShortID, oldSection, sectionName)
	}

	return nil
}
