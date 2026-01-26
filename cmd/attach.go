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
	Short: "Attach a file or URL to a task",
	Long: `Attach a file or URL reference to a task.

Files are copied to .attachments/<short-id>/ alongside tasks.md.
URLs are stored as references only (not downloaded).

Examples:
  tt attach 5 screenshot.png           # Attach a file
  tt attach 5 --url https://figma.com/xyz  # Attach a URL reference
  tt attach abc123 ~/docs/spec.pdf`,
	Args: func(cmd *cobra.Command, args []string) error {
		if attachURL != "" {
			// URL mode: only need task-id
			if len(args) < 1 {
				return fmt.Errorf("requires task-id argument")
			}
			return nil
		}
		// File mode: need task-id and file-path
		if len(args) < 2 {
			return fmt.Errorf("requires task-id and file-path arguments")
		}
		return nil
	},
	RunE: runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
	attachCmd.Flags().StringVarP(&attachURL, "url", "u", "", "attach a URL reference instead of a file")
}

func runAttach(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return tterrors.NewTaskNotFoundError(args[0])
	}

	// Handle URL attachment
	if attachURL != "" {
		url := strings.TrimSpace(attachURL)
		if url == "" {
			return fmt.Errorf("URL cannot be empty")
		}

		task.AddAttachment(url, true, url)

		if err := storage.BackupProject(project); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}

		if err := storage.SaveProject(project); err != nil {
			return fmt.Errorf("failed to save project: %w", err)
		}

		fmt.Printf("Attached URL to #%d: %s\n", task.ShortID, url)
		return nil
	}

	// Handle file attachment
	filePath := args[1]

	// Expand ~ to home directory
	if strings.HasPrefix(filePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home directory: %w", err)
		}
		filePath = filepath.Join(home, filePath[2:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", absPath)
	}

	// Copy file to attachments directory
	filename, err := storage.CopyAttachment(project, task, absPath)
	if err != nil {
		return fmt.Errorf("failed to copy attachment: %w", err)
	}

	// Add attachment to task metadata
	task.AddAttachment(filename, false, absPath)

	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if err := storage.SaveProject(project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	fmt.Printf("Attached file to #%d: %s\n", task.ShortID, filename)
	return nil
}
