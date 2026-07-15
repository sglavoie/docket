package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tterrors"
)

var noteLinkRemove bool

var noteLinkCmd = &cobra.Command{
	Use:   "link <note> <task-id>",
	Short: "Link or unlink a note to a task",
	Long: `Add or remove a task link from a note.

Examples:
  tt note link meeting 5
  tt note link meeting abc123
  tt note link --remove meeting 5`,
	RunE: runNoteLink,
}

func init() {
	noteCmd.AddCommand(noteLinkCmd)
	noteLinkCmd.Flags().BoolVar(&noteLinkRemove, "remove", false, "remove the link instead of adding")
}

func runNoteLink(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("note ID and task ID")
	}

	// Reject --global explicitly: a link requires a project-scoped task, so
	// global notes cannot be linked. Without this guard the persistent flag
	// is silently ignored and the lookup falls through to the project notes
	// directory, producing a misleading "note not found" error when the
	// user's global note actually exists.
	if noteGlobal {
		return fmt.Errorf("--global cannot be used with 'note link': links require a project task")
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	ctx := &NoteContext{Location: app.ProjectNoteLocation(project.ID), Project: project, Workspace: workspace}
	note, err := findNoteContext(ctx, args[0])
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, args[1])
	if findErr != nil {
		return findErr
	}

	if noteLinkRemove {
		note.RemoveLink(task.ID)
	} else {
		note.AddLink(task.ID)
	}

	if _, err := updateNoteContext(ctx, note); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	if noteLinkRemove {
		fmt.Printf("Unlinked note '%s' from task #%d '%s'\n", note.Title, task.ShortID, task.Title)
	} else {
		fmt.Printf("Linked note '%s' to task #%d '%s'\n", note.Title, task.ShortID, task.Title)
	}

	return nil
}
