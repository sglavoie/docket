package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskSectionCmd = &cobra.Command{
	Use:   "section <task-id>... <section>",
	Short: "Assign task(s) to a section",
	Long: `Assign one or more tasks to a section for organization.

The last argument is the section name. Use an empty string "" to clear.

Examples:
  tt task section 5 "In Progress"
  tt task section 1 2 3 Review
  tt task section 3 ""`,
	Args: cobra.MinimumNArgs(2),
	RunE: runTaskSection,
}

func init() {
	taskCmd.AddCommand(taskSectionCmd)
}

func runTaskSection(cmd *cobra.Command, args []string) error {
	// Last argument is the section name, everything before is task IDs
	taskIDs := args[:len(args)-1]
	sectionName := args[len(args)-1]

	// Validate section name upfront (not per-task)
	if sectionName != "" {
		if err := models.ValidateSectionName(sectionName); err != nil {
			return err
		}
	}

	// Pre-load project to check for no-ops before saving
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	var errs []string
	succeeded := 0
	skipped := 0
	for _, taskID := range taskIDs {
		// Check if the section is already the requested value
		preTask, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, findErr))
			continue
		}
		if preTask.Section == sectionName {
			skipped++
			if sectionName == "" {
				fmt.Printf("Task #%d already has no section.\n", preTask.ShortID)
			} else {
				fmt.Printf("Task #%d is already in section '%s'.\n", preTask.ShortID, sectionName)
			}
			continue
		}

		var oldSection string

		task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
			oldSection = task.Section

			if sectionName != "" {
				project.EnsureSection(sectionName)
			}

			task.Section = sectionName
			return nil
		})

		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		succeeded++
		if sectionName == "" {
			fmt.Printf("Cleared section from task #%d (was: %s)\n", task.ShortID, oldSection)
		} else if oldSection == "" {
			fmt.Printf("Assigned task #%d to section: %s\n", task.ShortID, sectionName)
		} else {
			fmt.Printf("Moved task #%d from '%s' to '%s'\n", task.ShortID, oldSection, sectionName)
		}
	}

	if skipped > 0 && succeeded > 0 {
		fmt.Printf("%d updated, %d already in the requested section\n", succeeded, skipped)
	}

	return ReportBatchErrors(succeeded, errs)
}
