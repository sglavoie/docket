package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var attachmentOpenCmd = &cobra.Command{
	Use:   "open <task-id> <attachment-index>",
	Short: "Open an attachment",
	Long: `Open an attachment with the system default application.

For files, opens with the OS default app.
For URLs, opens in the default browser.

Examples:
  tt attachment open 5 0      # Open first attachment on task 5
  tt attachment open abc123 1 # Open second attachment`,
	Args: cobra.ExactArgs(2),
	RunE: runAttachmentOpen,
}

func init() {
	attachmentCmd.AddCommand(attachmentOpenCmd)
}

func runAttachmentOpen(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return tterrors.NewTaskNotFoundError(args[0])
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

	var target string
	if att.IsURL {
		target = att.Filename
	} else {
		target = storage.GetAttachmentPath(project, task, att.Filename)
		if !storage.AttachmentExists(project, task, att.Filename) {
			return fmt.Errorf("attachment file not found: %s", att.Filename)
		}
	}

	// Open with system default application
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", target)
	case "linux":
		openCmd = exec.Command("xdg-open", target)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", target)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	if err := openCmd.Start(); err != nil {
		return fmt.Errorf("failed to open attachment: %w", err)
	}

	if att.IsURL {
		fmt.Printf("Opening URL: %s\n", att.Filename)
	} else {
		fmt.Printf("Opening file: %s\n", att.Filename)
	}

	return nil
}
