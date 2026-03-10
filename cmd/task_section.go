package cmd

import (
	"fmt"
	"strings"

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

	var errs []string
	succeeded := 0
	for _, taskID := range taskIDs {
		var oldSection string

		task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
			if sectionName != "" {
				project.EnsureSection(sectionName)
			}

			oldSection = task.Section
			task.Section = sectionName
			return nil
		})

		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		succeeded++
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
	}

	if len(errs) > 0 {
		if succeeded > 0 {
			fmt.Printf("\n%d succeeded, %d failed:\n", succeeded, len(errs))
		}
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
