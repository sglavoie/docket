package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/spf13/cobra"
)

var archiveForce bool

var taskArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive completed tasks",
	Long: `Move completed tasks from tasks.md to completed.md.

Completed tasks are appended to the archive file and removed from the main tasks file.
When the archive file exceeds 5MB, it is rotated (completed.md -> completed.1.md, etc.).

Use --force to skip the confirmation prompt.

Examples:
  tt task archive          # Archive with confirmation
  tt task archive --force  # Archive without confirmation`,
	RunE: runTaskArchive,
}

func init() {
	taskCmd.AddCommand(taskArchiveCmd)
	taskArchiveCmd.Flags().BoolVarP(&archiveForce, "force", "f", false, "Skip confirmation prompt")
}

func runTaskArchive(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		fmt.Println("No completed tasks to archive.")
		return nil
	}

	// Confirmation unless --force
	if !archiveForce {
		fmt.Printf("Archive %d completed task(s) to completed.md?\n", len(completedTasks))
		fmt.Print("Type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Create backup before archiving
	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Archive the tasks
	count, err := storage.ArchiveTasks(project)
	if err != nil {
		return fmt.Errorf("failed to archive tasks: %w", err)
	}

	fmt.Printf("Archived %d task(s) to completed.md\n", count)
	return nil
}
