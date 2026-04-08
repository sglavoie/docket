package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var keepFile bool
var attachmentRemoveForce bool

var attachmentRemoveCmd = &cobra.Command{
	Use:     "remove [task-id] [attachment-index]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an attachment",
	Long: `Remove an attachment from a task.

If no task ID is provided, an interactive selector is shown (filtered to tasks
that have attachments). The attachment index can be found via 'tt attachment list'.

By default, this also deletes the file from disk for local attachments.
Use --keep-file to only remove the reference without deleting the file.

Examples:
  tt attachment remove 5 0           # Remove and delete first attachment
  tt attachment rm 5 0 --keep-file   # Remove reference but keep file
  tt attachment remove               # Interactive task selection
  tt attachment remove 5 0 --force   # Skip confirmation prompt`,
	RunE: runAttachmentRemove,
}

func init() {
	attachmentCmd.AddCommand(attachmentRemoveCmd)
	attachmentRemoveCmd.Flags().BoolVar(&keepFile, "keep-file", false, "keep the file on disk")
	attachmentRemoveCmd.Flags().BoolVarP(&attachmentRemoveForce, "force", "f", false, "skip confirmation prompt")
	addOutputFlags(attachmentRemoveCmd)
}

func runAttachmentRemove(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Select the task: from args or interactively (only tasks with attachments)
	task, err := SelectTask(project, args,
		func(t *models.Task) bool { return len(t.Attachments) > 0 },
		"Select task to remove attachment from:", "No tasks with attachments found.")
	if err != nil {
		return err
	}

	if len(task.Attachments) == 0 {
		return fmt.Errorf("task #%d has no attachments; use 'tt attachment list' to check", task.ShortID)
	}

	// If no attachment index provided, list attachments and give a hint
	if len(args) < 2 {
		var b strings.Builder
		fmt.Fprintf(&b, "Task #%d has %d attachment(s):\n", task.ShortID, len(task.Attachments))
		for i, att := range task.Attachments {
			kind := "local"
			if att.IsURL {
				kind = "URL"
			}
			fmt.Fprintf(&b, "  %d: %s (%s)\n", i, att.Filename, kind)
		}
		fmt.Fprintf(&b, "Specify the index: tt attachment remove %d <index>", task.ShortID)
		return fmt.Errorf("%s", b.String())
	}

	index, err := parseItemIndex(args[1], len(task.Attachments), "attachment")
	if err != nil {
		return err
	}

	att := task.Attachments[index]
	filename := att.Filename
	isURL := att.IsURL

	prompt := fmt.Sprintf("Remove attachment '%s' from task #%d? [y/N] ", filename, task.ShortID)
	if !ConfirmYN(prompt, attachmentRemoveForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Remove from task metadata
	removed, ok := task.RemoveAttachment(index)
	if !ok {
		return fmt.Errorf("failed to remove attachment '%s' from task #%d", filename, task.ShortID)
	}

	// Delete file from disk if not a URL and not keeping file
	if !isURL && !keepFile {
		if err := storage.RemoveAttachmentFile(project, task, removed.Filename); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete file: %v\n", err)
		}
	}

	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Remove Attachment", UndoContext: filename,
	}); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if isURL {
		fmt.Printf("Removed URL from #%d: %s\n", task.ShortID, filename)
	} else if keepFile {
		fmt.Printf("Removed reference from #%d: %s (file kept in .attachments/)\n", task.ShortID, filename)
	} else {
		fmt.Printf("Deleted attachment copy from #%d: %s\n", task.ShortID, filename)
	}

	return nil
}
