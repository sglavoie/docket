package cmd

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var attachmentsCmd = &cobra.Command{
	Use:     "attachments <task-id>",
	Aliases: []string{"att"},
	Short:   "List attachments on a task",
	Long: `List all attachments on a task.

Shows filename/URL, size (for local files), and when it was added.

Examples:
  tt attachments 5
  tt att abc123`,
	RunE: runAttachmentsList,
}

func init() {
	rootCmd.AddCommand(attachmentsCmd)
}

func runAttachmentsList(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("task ID")
	}

	return WithTaskRead(args[0], func(project *models.Project, task *models.Task) error {
		return displayAttachmentsList(project, task)
	})
}

func displayAttachmentsList(project *models.Project, task *models.Task) error {
	fmt.Printf("#%d: %s\n", task.ShortID, task.Title)
	fmt.Println(strings.Repeat("-", 40))

	if len(task.Attachments) == 0 {
		fmt.Println("No attachments.")
		fmt.Println("\nAttach files with: tt attach <task-id> <file-path>")
		return nil
	}

	fmt.Printf("%d attachment(s):\n\n", len(task.Attachments))

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"#", "Type", "Name", "Size", "Added"})

	for i, att := range task.Attachments {
		attType := "File"
		name := att.Filename
		size := ""

		if att.IsURL {
			attType = "URL"
			// Truncate long URLs
			if len(name) > 50 {
				name = name[:47] + "..."
			}
		} else {
			attSize := storage.GetAttachmentSize(project, task, &att)
			if attSize > 0 {
				size = storage.FormatFileSize(attSize)
			} else {
				size = "(missing)"
			}
		}

		added := storage.FormatRelativeTime(att.AddedAt)

		t.AppendRow(table.Row{i, attType, name, size, added})
	}

	fmt.Println(t.Render())

	return nil
}
