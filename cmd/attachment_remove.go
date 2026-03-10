package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var keepFile bool

var attachmentRemoveCmd = &cobra.Command{
	Use:     "remove <task-id> <attachment-index>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an attachment",
	Long: `Remove an attachment from a task.

By default, this also deletes the file from disk for local attachments.
Use --keep-file to only remove the reference without deleting the file.

Examples:
  tt attachment remove 5 0           # Remove and delete first attachment
  tt attachment rm 5 0 --keep-file   # Remove reference but keep file`,
	Args: cobra.ExactArgs(2),
	RunE: runAttachmentRemove,
}

func init() {
	attachmentCmd.AddCommand(attachmentRemoveCmd)
	attachmentRemoveCmd.Flags().BoolVar(&keepFile, "keep-file", false, "keep the file on disk")
}

func runAttachmentRemove(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	if len(task.Attachments) == 0 {
		return fmt.Errorf("task #%d has no attachments", task.ShortID)
	}

	index, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid attachment index: %s", args[1])
	}

	if index < 0 || index >= len(task.Attachments) {
		return fmt.Errorf("attachment index %d out of range (0-%d)", index, len(task.Attachments)-1)
	}

	att := task.Attachments[index]
	filename := att.Filename
	isURL := att.IsURL

	// Remove from task metadata
	removed, ok := task.RemoveAttachment(index)
	if !ok {
		return fmt.Errorf("failed to remove attachment")
	}

	// Delete file from disk if not a URL and not keeping file
	if !isURL && !keepFile {
		if err := storage.RemoveAttachmentFile(project, task, removed.Filename); err != nil {
			fmt.Printf("Warning: failed to delete file: %v\n", err)
		}
	}

	if err := SaveProject(project, nil); err != nil {
		return err
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
