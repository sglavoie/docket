package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
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
	taskID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	sectionName := args[1]

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := project.GetTaskByShortID(taskID)
	if task == nil {
		return fmt.Errorf("task #%d not found", taskID)
	}

	// If section name is provided and non-empty, ensure it exists
	if sectionName != "" {
		project.EnsureSection(sectionName)
	}

	oldSection := task.Section
	task.Section = sectionName
	task.UpdatedAt = time.Now()

	if err := SaveProjectWithRules(project, cfg); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	if sectionName == "" {
		if oldSection != "" {
			fmt.Printf("Cleared section from task #%d (was: %s)\n", taskID, oldSection)
		} else {
			fmt.Printf("Task #%d has no section\n", taskID)
		}
	} else if oldSection == "" {
		fmt.Printf("Assigned task #%d to section: %s\n", taskID, sectionName)
	} else {
		fmt.Printf("Moved task #%d from '%s' to '%s'\n", taskID, oldSection, sectionName)
	}

	return nil
}
