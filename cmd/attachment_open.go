package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var attachmentOpenCmd = &cobra.Command{
	Use:   "open <task-id> <attachment-index>",
	Short: "Open an attachment",
	Long: `Open an attachment with the appropriate application.

For files, opens with the OS default app (or configured attachment editor).
For URLs, opens in the default browser.
For code references, opens in the configured code editor at the specified line.

Examples:
  tt attachment open 5 0      # Open first attachment on task 5
  tt attachment open abc123 1 # Open second attachment`,
	RunE: runAttachmentOpen,
}

func init() {
	attachmentCmd.AddCommand(attachmentOpenCmd)
}

// buildAttachmentEditorCmd builds an exec.Cmd to open target with the
// configured attachment editor. The editor string may contain flags
// (e.g. "vlc --fullscreen"), so it is split before constructing the command.
func buildAttachmentEditorCmd(editor, target string) *exec.Cmd {
	exe, editorArgs := external.SplitEditorString(editor)
	return exec.Command(exe, append(editorArgs, target)...)
}

func runAttachmentOpen(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and attachment name")
	}

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	index, err := parseItemIndex(args[1], len(task.Attachments), "attachment")
	if err != nil {
		return err
	}

	att := task.Attachments[index]

	// Handle code references
	if att.IsCodeRef {
		codeEditor := cfg.GetCodeEditor()
		if !storage.CodeReferenceFileExists(att.OriginalPath) {
			return fmt.Errorf("code reference file not found: %s", att.OriginalPath)
		}
		if err := external.OpenCodeReference(codeEditor, att.OriginalPath, att.LineNumber); err != nil {
			return fmt.Errorf("failed to open code reference: %w", err)
		}
		fmt.Printf("Opening code reference: %s:%d\n", att.OriginalPath, att.LineNumber)
		return nil
	}

	var target string
	if att.IsURL {
		target = att.Filename
	} else {
		target = storage.GetAttachmentPath(project, task, att.Filename)
		if !storage.AttachmentExists(project, task, att.Filename) {
			return fmt.Errorf("attachment file not found: %s", att.Filename)
		}
	}

	// Check if we should use a configured attachment editor (for non-URL files)
	if !att.IsURL && cfg.GetAttachmentEditor() != "" {
		openCmd := buildAttachmentEditorCmd(cfg.GetAttachmentEditor(), target)
		if err := openCmd.Start(); err != nil {
			return fmt.Errorf("failed to open attachment: %w", err)
		}
		fmt.Printf("Opening file: %s\n", att.Filename)
		return nil
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
