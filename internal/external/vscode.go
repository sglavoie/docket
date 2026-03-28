package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateTempFileWithContent creates a temporary file with the given content.
// Useful for editing task descriptions in external editors.
func CreateTempFileWithContent(content string, prefix string, suffix string) (string, error) {
	dir := os.TempDir()
	pattern := prefix + "*" + suffix
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	return f.Name(), nil
}

// ResolveEditor returns the editor command to use, applying the fallback chain:
// configEditor → EDITOR env → "code" → "vi".
// Each candidate is verified with exec.LookPath before being returned.
// Returns an error if no editor is found.
func ResolveEditor(configEditor string) (string, error) {
	candidates := []string{configEditor, os.Getenv("EDITOR"), "code"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := exec.LookPath(c); err == nil {
			return c, nil
		}
		fmt.Fprintf(os.Stderr, "warning: editor %q not found in PATH, trying next fallback\n", c)
	}
	// Last resort: check if vi is available before returning it
	if _, err := exec.LookPath("vi"); err == nil {
		return "vi", nil
	}
	return "", fmt.Errorf("no editor found: set $EDITOR or install VS Code")
}

// EditorCommand builds an exec.Cmd for the given editor and file path,
// adding --wait for editors that need it (VS Code, Sublime Text, Zed).
func EditorCommand(editor, path string) *exec.Cmd {
	if needsWaitFlag(editor) {
		return exec.Command(editor, "--wait", path)
	}
	return exec.Command(editor, path)
}

// needsWaitFlag returns true if the editor needs --wait to block until
// the file is closed. Checks the base name to handle full paths and
// variants like "code-insiders".
func needsWaitFlag(editor string) bool {
	base := strings.ToLower(filepath.Base(editor))
	if base == "code" || base == "subl" || base == "sublime" || base == "zed" {
		return true
	}
	if strings.HasPrefix(base, "code-") {
		return true
	}
	return false
}

// EditInExternalEditor opens content in an external editor and returns the modified content.
func EditInExternalEditor(editor string, content string, extension string) (string, error) {
	// Create temp file
	tmpPath, err := CreateTempFileWithContent(content, "tt-edit-", extension)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpPath) }()

	// Get editor command
	resolvedEditor, resolveErr := ResolveEditor(editor)
	if resolveErr != nil {
		return "", resolveErr
	}
	editor = resolvedEditor
	cmd := EditorCommand(editor, tmpPath)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if runErr := cmd.Run(); runErr != nil {
		return "", fmt.Errorf("editor failed: %w", runErr)
	}

	// Read modified content
	modified, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read modified file: %w", err)
	}

	return string(modified), nil
}
