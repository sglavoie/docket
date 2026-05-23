package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var attachURL string

var attachCmd = &cobra.Command{
	Use:   "attach <task-id> <file-path>",
	Short: "Attach a file, URL, or code reference to a task",
	Long: `Attach a file, URL reference, or code reference to a task.

Files are copied to .attachments/<short-id>/ alongside tasks.md.
URLs are stored as references only (not downloaded).
Code references use the file:line format and open in your code editor.

Examples:
  tt attach 5 screenshot.png               # Attach a file (copied)
  tt attach 5 --url https://figma.com/xyz  # Attach a URL reference
  tt attach abc123 ~/docs/spec.pdf         # Attach a file (copied)
  tt attach 5 src/main.go:42               # Code reference (opens in IDE)`,
	Args: func(cmd *cobra.Command, args []string) error {
		if attachURL != "" {
			// URL mode: only need task-id
			if len(args) < 1 {
				return tterrors.NewRequiredFieldError("task-id argument")
			}
			return nil
		}
		// File mode: need task-id and file-path
		if len(args) < 2 {
			return tterrors.NewRequiredFieldError("task-id and file-path arguments")
		}
		return nil
	},
	RunE: runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
	attachCmd.Flags().StringVarP(&attachURL, "url", "u", "", "attach a URL reference instead of a file")
	addOutputFlags(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error { //nolint:gocyclo
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, cfg, configManager, errLoad := LoadCurrentProject()
	if errLoad != nil {
		return errLoad
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	// Handle URL attachment
	if attachURL != "" {
		url := strings.TrimSpace(attachURL)
		if url == "" {
			return fmt.Errorf("URL cannot be empty")
		}
		if len(url) > 2000 {
			return fmt.Errorf("URL exceeds maximum length of 2000 characters (%d)", len(url))
		}

		err = task.AddAttachment(url, true, url)
		if err != nil {
			return err
		}

		err = SaveProject(project, &SaveOptions{
			Config: cfg, ConfigManager: configManager,
			UndoAction: "Attach URL", UndoContext: url,
		})
		if err != nil {
			return err
		}

		if out.Quiet {
			return nil
		}
		if out.JSON {
			return printJSON(struct {
				TaskShortID int    `json:"task_short_id"`
				Attachment  string `json:"attachment"`
				Type        string `json:"type"`
			}{TaskShortID: task.ShortID, Attachment: url, Type: "url"})
		}
		fmt.Printf("Attached URL to #%d: %s\n", task.ShortID, url)
		return nil
	}

	// Handle file or code reference attachment
	filePath := args[1]

	// Strip surrounding single quotes (e.g. from drag-and-drop paths with spaces)
	if len(filePath) >= 2 && filePath[0] == '\'' && filePath[len(filePath)-1] == '\'' {
		filePath = filePath[1 : len(filePath)-1]
	}

	// Check if it's a code reference (file:line format)
	codePath, lineNum, isCodeRef := storage.ParseCodeReference(filePath)
	if isCodeRef {
		// Store code reference without copying
		err = task.AddCodeReference(codePath, lineNum)
		if err != nil {
			return err
		}

		err = SaveProject(project, &SaveOptions{
			Config: cfg, ConfigManager: configManager,
			UndoAction: "Attach Code Ref", UndoContext: fmt.Sprintf("%s:%d", codePath, lineNum),
		})
		if err != nil {
			return err
		}

		if out.Quiet {
			return nil
		}
		if out.JSON {
			return printJSON(struct {
				TaskShortID int    `json:"task_short_id"`
				Attachment  string `json:"attachment"`
				Type        string `json:"type"`
			}{TaskShortID: task.ShortID, Attachment: fmt.Sprintf("%s:%d", codePath, lineNum), Type: "code_ref"})
		}
		// Check if file exists and show info message if not
		if !storage.CodeReferenceFileExists(codePath) {
			fmt.Printf("Attached code reference to #%d: %s:%d\n", task.ShortID, codePath, lineNum)
			fmt.Println("Note: referenced file does not currently exist")
		} else {
			fmt.Printf("Attached code reference to #%d: %s:%d\n", task.ShortID, codePath, lineNum)
		}
		return nil
	}

	// Handle regular file attachment
	// Expand ~ to home directory
	if strings.HasPrefix(filePath, "~/") {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return fmt.Errorf("failed to expand home directory: %w", homeErr)
		}
		filePath = filepath.Join(home, filePath[2:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		return fmt.Errorf("file not found: %s", absPath)
	}

	// Copy file to attachments directory
	filename, err := storage.CopyAttachment(project, task, absPath)
	if err != nil {
		return fmt.Errorf("failed to copy attachment: %w", err)
	}

	// Add attachment to task metadata. Remove the just-copied file on failure
	// (validation or save) so we don't leave orphaned bytes in .attachments/.
	cleanupCopiedFile := func() {
		if rmErr := storage.RemoveAttachmentFile(project, task, filename); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean up copied attachment: %v\n", rmErr)
		}
	}

	if err = task.AddAttachment(filename, false, absPath); err != nil {
		cleanupCopiedFile()
		return err
	}

	if err = SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Attach File", UndoContext: filename,
	}); err != nil {
		cleanupCopiedFile()
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			TaskShortID int    `json:"task_short_id"`
			Attachment  string `json:"attachment"`
			Type        string `json:"type"`
		}{TaskShortID: task.ShortID, Attachment: filename, Type: "file"})
	}
	fmt.Printf("Attached file to #%d: %s\n", task.ShortID, filename)
	return nil
}
