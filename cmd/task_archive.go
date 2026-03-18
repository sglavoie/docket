package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var archiveForce bool
var archiveAll bool

var taskArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive completed tasks",
	Long: `Move completed tasks from tasks.md to completed.md.

Completed tasks are appended to the archive file and removed from the main tasks file.
When the archive file exceeds 5MB, it is rotated (completed.md -> completed.1.md, etc.).

Use --force to skip the confirmation prompt.

Examples:
  tt task archive          # Archive with confirmation
  tt task archive --force  # Archive without confirmation
  tt task archive --all    # Archive completed tasks across all projects`,
	RunE: runTaskArchive,
}

func init() {
	taskCmd.AddCommand(taskArchiveCmd)
	taskArchiveCmd.Flags().BoolVarP(&archiveForce, "force", "f", false, "Skip confirmation prompt")
	taskArchiveCmd.Flags().BoolVar(&archiveAll, "all", false, "Archive completed tasks across all projects")
}

func runTaskArchive(cmd *cobra.Command, args []string) error {
	if archiveAll {
		return runTaskArchiveAll()
	}

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		fmt.Println("No completed tasks to archive.")
		return nil
	}

	// Confirmation unless --force
	if !ConfirmYN(fmt.Sprintf("Archive %s to completed.md? [y/N] ", Pluralize(len(completedTasks), "completed task", "completed tasks")), archiveForce) {
		fmt.Println("Canceled")
		return nil
	}

	// Record undo snapshot before archiving (this is destructive and important to undo)
	if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
		if snapshotErr := storage.RecordUndoSnapshot(project, "Archive Tasks",
			fmt.Sprintf("%d tasks", len(completedTasks)), *cfg.UndoHistoryDepth); snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot: %v\n", snapshotErr)
		}
	}

	// Create backup before archiving
	if backupErr := storage.BackupProject(project); backupErr != nil {
		return fmt.Errorf("failed to create backup: %w", backupErr)
	}

	// Archive the tasks
	count, err := storage.ArchiveTasks(project)
	if err != nil {
		return fmt.Errorf("failed to archive tasks: %w", err)
	}

	fmt.Printf("Archived %s to completed.md\n", Pluralize(count, "task", "tasks"))
	return nil
}

func runTaskArchiveAll() error {
	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	projects := cfg.GetActiveProjects()
	totalArchived := 0
	var failures []string

	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			failures = append(failures, fmt.Sprintf("[%s] load failed: %v", project.Name, err))
			continue
		}

		completedTasks := project.GetCompletedTasks()
		if len(completedTasks) == 0 {
			continue
		}

		if !ConfirmYN(fmt.Sprintf("[%s] Archive %s? [y/N] ", project.Name, Pluralize(len(completedTasks), "completed task", "completed tasks")), archiveForce) {
			fmt.Printf("[%s] Skipped.\n", project.Name)
			continue
		}

		if backupErr := storage.BackupProject(project); backupErr != nil {
			failures = append(failures, fmt.Sprintf("[%s] backup failed: %v", project.Name, backupErr))
			continue
		}

		count, err := storage.ArchiveTasks(project)
		if err != nil {
			failures = append(failures, fmt.Sprintf("[%s] archive failed: %v", project.Name, err))
			continue
		}

		fmt.Printf("[%s] Archived %s\n", project.Name, Pluralize(count, "task", "tasks"))
		totalArchived += count
	}

	fmt.Printf("\nTotal: archived %s across all projects\n", Pluralize(totalArchived, "task", "tasks"))
	if len(failures) > 0 {
		fmt.Printf("\n%d project(s) had errors:\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  %s\n", f)
		}
	}
	return nil
}
