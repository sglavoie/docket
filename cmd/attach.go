package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// validateAttachURL ensures the input looks like a real URL reference with an
// explicit scheme. Without this check, callers can attach arbitrary strings
// (e.g. "example.com" or "not a url"), which later fail with a confusing
// "file not found" when the OS opener interprets them as paths.
func validateAttachURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("URL must include a scheme (e.g. https://): %s", raw)
	}
	// Reject obvious garbage: scheme-only with no opaque/host (e.g. "https://").
	if parsed.Host == "" && parsed.Opaque == "" && parsed.Path == "" {
		return fmt.Errorf("URL must include a host or path: %s", raw)
	}
	return nil
}

var attachURL string

var attachCmd = &cobra.Command{
	Use:   "attach <task-id> <file-path>",
	Short: "Attach a file, URL, or code reference to a task",
	Long: `Attach a file, URL reference, or code reference to a task.

Files are copied into the project's .attachments/ tree alongside tasks.md.
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

	project, workspace, errLoad := LoadRuntimeCurrentProject()
	if errLoad != nil {
		return errLoad
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	svc := newWorkspaceAppService(workspace)

	// Handle URL attachment
	if attachURL != "" {
		url := strings.TrimSpace(attachURL)
		if url == "" {
			return fmt.Errorf("URL cannot be empty")
		}
		if len(url) > 2000 {
			return fmt.Errorf("URL exceeds maximum length of 2000 characters (%d)", len(url))
		}
		if validationErr := validateAttachURL(url); validationErr != nil {
			return validationErr
		}

		res, attachErr := withTaskRetry(func() (app.TaskResult, error) {
			return svc.AttachURL(task.ID, url)
		})
		if attachErr != nil {
			return translateAppError(attachErr)
		}

		if out.Quiet {
			return nil
		}
		if out.JSON {
			return printJSON(struct {
				TaskShortID int    `json:"task_short_id"`
				Attachment  string `json:"attachment"`
				Type        string `json:"type"`
			}{TaskShortID: res.ShortID, Attachment: url, Type: "url"})
		}
		fmt.Printf("Attached URL to #%d: %s\n", res.ShortID, url)
		return nil
	}

	// Handle file or code reference attachment. Join any trailing args so a path
	// pasted across multiple tokens survives, then normalize pasted shell-style
	// input (quotes or backslash-escaped spaces) down to the underlying path.
	filePath := util.NormalizePastedPathInput(strings.Join(args[1:], " "))

	// Check if it's a code reference (file:line format)
	codePath, lineNum, isCodeRef := storage.ParseCodeReference(filePath)
	if isCodeRef {
		// Store code reference without copying.
		res, attachErr := withTaskRetry(func() (app.TaskResult, error) {
			return svc.AttachCodeReference(task.ID, codePath, lineNum)
		})
		if attachErr != nil {
			return translateAppError(attachErr)
		}

		if out.Quiet {
			return nil
		}
		if out.JSON {
			return printJSON(struct {
				TaskShortID int    `json:"task_short_id"`
				Attachment  string `json:"attachment"`
				Type        string `json:"type"`
			}{TaskShortID: res.ShortID, Attachment: fmt.Sprintf("%s:%d", codePath, lineNum), Type: "code_ref"})
		}
		// Check if file exists and show info message if not
		if !storage.CodeReferenceFileExists(codePath) {
			fmt.Printf("Attached code reference to #%d: %s:%d\n", res.ShortID, codePath, lineNum)
			fmt.Println("Note: referenced file does not currently exist")
		} else {
			fmt.Printf("Attached code reference to #%d: %s:%d\n", res.ShortID, codePath, lineNum)
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

	// Check if file exists and is a regular file.
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("file not found: %s", absPath)
		}
		return fmt.Errorf("failed to access %s: %w", absPath, statErr)
	}
	// Directories cannot be attached; without this guard the copy fails with an
	// opaque "is a directory" io error later in the pipeline.
	if info.IsDir() {
		return fmt.Errorf("cannot attach a directory: %s", absPath)
	}

	// Route the file attachment through the application layer: the metadata write
	// is validated first, then the source bytes ride the SAME transaction as an
	// AttachmentCopy side effect. A validation failure aborts before any byte is
	// written and a commit failure leaves no orphan, so the legacy copy-then-clean-up
	// compensation is gone.
	result, attachErr := withResultRetry(func() (app.AttachFileResult, error) {
		return svc.AttachFile(task.ID, absPath)
	})
	if attachErr != nil {
		return translateAppError(attachErr)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			TaskShortID int    `json:"task_short_id"`
			Attachment  string `json:"attachment"`
			Type        string `json:"type"`
		}{TaskShortID: result.Task.ShortID, Attachment: result.Filename, Type: "file"})
	}
	fmt.Printf("Attached file to #%d: %s\n", result.Task.ShortID, result.Filename)
	return nil
}
