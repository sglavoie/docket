package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
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
	Args: cobra.ExactArgs(2),
	RunE: runNoteLink,
}

func init() {
	noteCmd.AddCommand(noteLinkCmd)
	noteLinkCmd.Flags().BoolVar(&noteLinkRemove, "remove", false, "remove the link instead of adding")
}

func runNoteLink(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	note, err := findNote(project, args[0])
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[1])
	if task == nil {
		return fmt.Errorf("task not found: %s", args[1])
	}

	if noteLinkRemove {
		note.RemoveLink(task.ID)
		fmt.Printf("Unlinked note '%s' from task #%d '%s'\n", note.Title, task.ShortID, task.Title)
	} else {
		note.AddLink(task.ID)
		fmt.Printf("Linked note '%s' to task #%d '%s'\n", note.Title, task.ShortID, task.Title)
	}

	if err := storage.SaveNote(project, note); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	return nil
}
