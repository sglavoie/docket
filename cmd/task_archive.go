package cmd

import (
	"encoding/json"
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
	addOutputFlags(taskArchiveCmd)
}

func runTaskArchive(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if archiveAll {
		return runTaskArchiveAll(out)
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		fmt.Println("No completed tasks to archive.")
		return nil
	}

	// Confirmation unless --force
	if !ConfirmYN(fmt.Sprintf("Archive %s to completed.md? [y/N]: ", Pluralize(len(completedTasks), "completed task", "completed tasks")), archiveForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Record undo snapshot before archiving (this is destructive and important to undo)
	if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
		if snapshotErr := storage.RecordUndoSnapshot(project, "Archive Tasks",
			fmt.Sprintf("%d tasks", len(completedTasks)), *cfg.UndoHistoryDepth, nil); snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot: %v\n", snapshotErr)
		}
	}

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	// for all tasks being archived.
	archivedIDs := make(map[string]bool, len(completedTasks))
	for _, t := range completedTasks {
		archivedIDs[t.ID] = true
	}
	if cfg.RemoveTaskRefs(archivedIDs) > 0 {
		if saveErr := configManager.Save(cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save config after cleaning task refs: %v\n", saveErr)
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

	if out.JSON {
		entry := struct {
			ArchivedCount int    `json:"archived_count"`
			Project       string `json:"project"`
		}{ArchivedCount: count, Project: project.Name}
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	if !out.Quiet {
		fmt.Printf("Archived %s to completed.md\n", Pluralize(count, "task", "tasks"))
	}
	return nil
}

func runTaskArchiveAll(out MutateOutput) error {
	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	type projectResult struct {
		Name     string `json:"name"`
		Archived int    `json:"archived"`
	}

	projects := cfg.GetActiveProjects()
	totalArchived := 0
	configDirty := false
	var failures []string
	var results []projectResult

	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			failures = append(failures, fmt.Sprintf("[%s] load failed: %v", project.Name, err))
			continue
		}

		completedTasks := project.GetCompletedTasks()
		if len(completedTasks) == 0 {
			continue
		}

		if !ConfirmYN(fmt.Sprintf("[%s] Archive %s? [y/N]: ", project.Name, Pluralize(len(completedTasks), "completed task", "completed tasks")), archiveForce) {
			if !out.Quiet && !out.JSON {
				fmt.Printf("[%s] Skipped.\n", project.Name)
			}
			continue
		}

		// Record undo snapshot before archiving
		if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
			if snapshotErr := storage.RecordUndoSnapshot(project, "Archive Tasks",
				fmt.Sprintf("%d tasks", len(completedTasks)), *cfg.UndoHistoryDepth, nil); snapshotErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot for %s: %v\n", project.Name, snapshotErr)
			}
		}

		// Clean up config references for archived tasks
		archivedIDs := make(map[string]bool, len(completedTasks))
		for _, t := range completedTasks {
			archivedIDs[t.ID] = true
		}
		if cfg.RemoveTaskRefs(archivedIDs) > 0 {
			configDirty = true
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

		results = append(results, projectResult{Name: project.Name, Archived: count})
		if !out.Quiet && !out.JSON {
			fmt.Printf("[%s] Archived %s\n", project.Name, Pluralize(count, "task", "tasks"))
		}
		totalArchived += count
	}

	// Save config once after all projects are processed
	if configDirty {
		if saveErr := configManager.Save(cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save config after cleaning task refs: %v\n", saveErr)
		}
	}

	if out.JSON {
		if results == nil {
			results = []projectResult{}
		}
		summary := struct {
			TotalArchived int             `json:"total_archived"`
			Projects      []projectResult `json:"projects"`
		}{TotalArchived: totalArchived, Projects: results}
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if !out.Quiet {
		fmt.Printf("\nTotal: archived %s across all projects\n", Pluralize(totalArchived, "task", "tasks"))
	}
	if len(failures) > 0 {
		fmt.Printf("\n%d project(s) had errors:\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  %s\n", f)
		}
		return fmt.Errorf("%s had errors during archiving", Pluralize(len(failures), "project", "projects"))
	}
	return nil
}
