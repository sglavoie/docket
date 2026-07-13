package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

var keepFile bool
var attachmentRemoveForce bool

var attachmentRemoveCmd = &cobra.Command{
	Use:     "remove [task-id] [attachment-index]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an attachment",
	Long: `Remove an attachment from a task.

If no task ID is provided, an interactive selector is shown (filtered to tasks
that have attachments). The attachment index can be found via 'tt attachments'.

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

	project, cfg, _, err := LoadCurrentProject()
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
		return fmt.Errorf("task #%d has no attachments; use 'tt attachments' to check", task.ShortID)
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

	filename := task.Attachments[index].Filename

	prompt := fmt.Sprintf("Remove attachment '%s' from task #%d? [y/N] ", filename, task.ShortID)
	if !ConfirmYN(prompt, attachmentRemoveForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Route the removal through the application layer: the metadata removal and,
	// when the attachment is a copied local file and --keep-file is not set, the
	// byte delete commit as ONE transaction. This closes the legacy window where a
	// crash between the metadata save and the file delete left the two out of sync.
	svc := newAppService(cfg)
	res, removeErr := withResultRetry(func() (app.RemoveAttachmentResult, error) {
		return svc.RemoveAttachment(task.ID, index, !keepFile)
	})
	if removeErr != nil {
		return translateAppError(removeErr)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return emitMutation(out, "", res.Task)
	}
	switch {
	case res.IsURL:
		fmt.Printf("Removed URL from #%d: %s\n", task.ShortID, res.Filename)
	case res.IsCodeRef:
		fmt.Printf("Removed code reference from #%d: %s\n", task.ShortID, res.Filename)
	case keepFile:
		fmt.Printf("Removed reference from #%d: %s (file kept in .attachments/)\n", task.ShortID, res.Filename)
	default:
		fmt.Printf("Deleted attachment copy from #%d: %s\n", task.ShortID, res.Filename)
	}

	return nil
}
