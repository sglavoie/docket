package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
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
	RunE: runTaskSection,
}

func init() {
	taskCmd.AddCommand(taskSectionCmd)
	addOutputFlags(taskSectionCmd)
	taskSectionCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 1 {
			project, _, _, err := LoadCurrentProject()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return project.GetSections(), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTaskSection(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and section name")
	}

	// Last argument is the section name, everything before is task IDs
	taskIDs := args[:len(args)-1]
	sectionName := args[len(args)-1]

	// Validate section name upfront (not per-task)
	if sectionName != "" {
		err = models.ValidateSectionName(sectionName)
		if err != nil {
			return err
		}
	}

	var errs []string
	succeeded := 0
	skipped := 0
	for _, taskID := range taskIDs {
		var oldSection string
		var skippedShortID int
		alreadyInSection := false

		task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
			// Check with fresh data whether the section already matches
			if task.Section == sectionName {
				alreadyInSection = true
				skippedShortID = task.ShortID
				return fmt.Errorf("no change needed")
			}
			oldSection = task.Section

			if sectionName != "" {
				project.EnsureSection(sectionName)
			}

			task.Section = sectionName
			return nil
		})

		if alreadyInSection {
			skipped++
			if !out.Quiet {
				if sectionName == "" {
					fmt.Printf("Task #%d already has no section.\n", skippedShortID)
				} else {
					fmt.Printf("Task #%d is already in section '%s'.\n", skippedShortID, sectionName)
				}
			}
			continue
		}

		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		succeeded++
		if out.JSON {
			if err := outputSingleTaskJSON(task); err != nil {
				return err
			}
		} else if !out.Quiet {
			if sectionName == "" {
				fmt.Printf("Cleared section from task #%d (was: %s)\n", task.ShortID, oldSection)
			} else if oldSection == "" {
				fmt.Printf("Assigned task #%d to section: %s\n", task.ShortID, sectionName)
			} else {
				fmt.Printf("Moved task #%d from '%s' to '%s'\n", task.ShortID, oldSection, sectionName)
			}
		}
	}

	if !out.Quiet && !out.JSON && skipped > 0 && succeeded > 0 {
		fmt.Printf("%d updated, %d already in the requested section\n", succeeded, skipped)
	}

	return ReportBatchErrors(succeeded, errs)
}
