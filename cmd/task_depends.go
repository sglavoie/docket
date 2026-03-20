package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	dependsOnFlag     string
	dependsRemoveFlag string
	dependsClearFlag  bool
)

var taskDependsCmd = &cobra.Command{
	Use:   "depends <id>",
	Short: "Manage task dependencies",
	Long: `View or manage dependencies for a task.

Without flags, shows the current dependencies.

Examples:
  tt task depends 5                    # Show dependencies for task 5
  tt task depends 5 --on 3             # Task 5 now depends on task 3
  tt task depends 5 --remove 3         # Remove dependency on task 3
  tt task depends 5 --clear            # Remove all dependencies`,
	RunE: runTaskDepends,
}

func init() {
	taskDependsCmd.Flags().StringVar(&dependsOnFlag, "on", "", "Add dependency on this task ID")
	taskDependsCmd.Flags().StringVar(&dependsRemoveFlag, "remove", "", "Remove dependency on this task ID")
	taskDependsCmd.Flags().BoolVar(&dependsClearFlag, "clear", false, "Clear all dependencies")

	taskCmd.AddCommand(taskDependsCmd)
}

func runTaskDepends(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("task ID")
	}

	taskID := args[0]

	// Mutual exclusion: only one of --clear, --remove, --on may be set
	flagCount := 0
	if dependsClearFlag {
		flagCount++
	}
	if dependsRemoveFlag != "" {
		flagCount++
	}
	if dependsOnFlag != "" {
		flagCount++
	}
	if flagCount > 1 {
		return fmt.Errorf("only one of --clear, --remove, --on can be used at a time")
	}

	// If no flags provided, just show dependencies (read-only)
	if !dependsClearFlag && dependsRemoveFlag == "" && dependsOnFlag == "" {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		showTaskDependencies(project, task)
		return nil
	}

	// Handle --clear
	if dependsClearFlag {
		// Pre-check: skip the save if there are no dependencies to clear
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}
		preTask, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		if len(preTask.DependsOn) == 0 {
			fmt.Printf("Task #%d has no dependencies to clear.\n", preTask.ShortID)
			return nil
		}

		task, err := WithTaskUpdate(taskID, func(t *models.Task) error {
			t.ClearDependencies()
			return nil
		})
		if err != nil {
			return err
		}
		fmt.Printf("Cleared all dependencies for #%d: %s\n", task.ShortID, task.Title)
		return nil
	}

	// Handle --remove
	if dependsRemoveFlag != "" {
		task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, t *models.Task) error {
			blocker, blockerErr := FindTaskByIdentifier(project, dependsRemoveFlag)
			if blockerErr != nil {
				return blockerErr
			}
			if !t.HasDependency(blocker.ID) {
				return fmt.Errorf("task #%d does not depend on #%d", t.ShortID, blocker.ShortID)
			}
			t.RemoveDependency(blocker.ID)
			fmt.Printf("Removed dependency: #%d no longer depends on #%d (%s)\n",
				t.ShortID, blocker.ShortID, blocker.Title)
			return nil
		})
		if err != nil {
			return err
		}
		_ = task // used by inner function
		return nil
	}

	// Handle --on
	if dependsOnFlag != "" {
		task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, t *models.Task) error {
			blocker, blockerErr := FindTaskByIdentifier(project, dependsOnFlag)
			if blockerErr != nil {
				return blockerErr
			}
			if blocker.ID == t.ID {
				return tterrors.ErrSelfDependency
			}
			if t.HasDependency(blocker.ID) {
				return fmt.Errorf("%w: #%d already depends on #%d", tterrors.ErrDuplicateDep, t.ShortID, blocker.ShortID)
			}
			if project.HasCircularDependency(t.ID, blocker.ID) {
				return fmt.Errorf("cannot add dependency: %w", tterrors.ErrCircularDep)
			}
			if err := t.AddDependency(blocker.ID); err != nil {
				return err
			}
			fmt.Printf("Added dependency: #%d now depends on #%d (%s)\n",
				t.ShortID, blocker.ShortID, blocker.Title)
			return nil
		})
		if err != nil {
			return err
		}
		_ = task // used by inner function
		return nil
	}

	return nil
}

func showTaskDependencies(project *models.Project, task *models.Task) {
	fmt.Printf("#%d: %s\n", task.ShortID, task.Title)
	fmt.Println(strings.Repeat("-", 40))

	// Show what this task depends on
	blockers := project.GetBlockingTasks(task)
	if len(blockers) == 0 {
		fmt.Println("Depends on: (none)")
	} else {
		fmt.Println("Depends on:")
		for _, b := range blockers {
			status := b.Status.Icon()
			fmt.Printf("  %s #%d: %s\n", status, b.ShortID, b.Title)
		}
	}

	// Show what depends on this task
	blocked := project.GetBlockedByTasks(task)
	if len(blocked) == 0 {
		fmt.Println("Blocks: (none)")
	} else {
		fmt.Println("Blocks:")
		for _, t := range blocked {
			status := t.Status.Icon()
			fmt.Printf("  %s #%d: %s\n", status, t.ShortID, t.Title)
		}
	}
}
