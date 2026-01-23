package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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
	Args: cobra.ExactArgs(1),
	RunE: runTaskDepends,
}

func init() {
	taskDependsCmd.Flags().StringVar(&dependsOnFlag, "on", "", "Add dependency on this task ID")
	taskDependsCmd.Flags().StringVar(&dependsRemoveFlag, "remove", "", "Remove dependency on this task ID")
	taskDependsCmd.Flags().BoolVar(&dependsClearFlag, "clear", false, "Clear all dependencies")

	taskCmd.AddCommand(taskDependsCmd)
}

func runTaskDepends(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return fmt.Errorf("task '%s' not found", args[0])
	}

	// Handle --clear
	if dependsClearFlag {
		if len(task.DependsOn) == 0 {
			fmt.Printf("Task #%d has no dependencies to clear\n", task.ShortID)
			return nil
		}
		task.ClearDependencies()
		if err := SaveProjectWithBackup(project); err != nil {
			return fmt.Errorf("failed to save: %w", err)
		}
		fmt.Printf("Cleared all dependencies for #%d: %s\n", task.ShortID, task.Title)
		return nil
	}

	// Handle --remove
	if dependsRemoveFlag != "" {
		blocker := FindTaskByIdentifier(project, dependsRemoveFlag)
		if blocker == nil {
			return fmt.Errorf("blocker task '%s' not found", dependsRemoveFlag)
		}
		if !task.HasDependency(blocker.ID) {
			return fmt.Errorf("task #%d does not depend on #%d", task.ShortID, blocker.ShortID)
		}
		task.RemoveDependency(blocker.ID)
		if err := SaveProjectWithBackup(project); err != nil {
			return fmt.Errorf("failed to save: %w", err)
		}
		fmt.Printf("Removed dependency: #%d no longer depends on #%d (%s)\n",
			task.ShortID, blocker.ShortID, blocker.Title)
		return nil
	}

	// Handle --on
	if dependsOnFlag != "" {
		blocker := FindTaskByIdentifier(project, dependsOnFlag)
		if blocker == nil {
			return fmt.Errorf("blocker task '%s' not found", dependsOnFlag)
		}
		if blocker.ID == task.ID {
			return fmt.Errorf("a task cannot depend on itself")
		}
		if task.HasDependency(blocker.ID) {
			return fmt.Errorf("task #%d already depends on #%d", task.ShortID, blocker.ShortID)
		}
		if project.HasCircularDependency(task.ID, blocker.ID) {
			return fmt.Errorf("cannot add dependency: would create circular dependency")
		}
		task.AddDependency(blocker.ID)
		if err := SaveProjectWithBackup(project); err != nil {
			return fmt.Errorf("failed to save: %w", err)
		}
		fmt.Printf("Added dependency: #%d now depends on #%d (%s)\n",
			task.ShortID, blocker.ShortID, blocker.Title)
		return nil
	}

	// Show dependencies (no flags)
	showTaskDependencies(project, task)
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
