package external

import (
	"fmt"
	"os"
	"os/exec"
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
// configEditor → EDITOR env → "code".
// Each candidate is verified with exec.LookPath before being returned.
func ResolveEditor(configEditor string) string {
	candidates := []string{configEditor, os.Getenv("EDITOR"), "code"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
		fmt.Fprintf(os.Stderr, "warning: editor %q not found in PATH, trying next fallback\n", c)
	}
	// Last resort: check if vi is available before returning it
	if _, err := exec.LookPath("vi"); err == nil {
		return "vi"
	}
	return "" // caller should handle empty editor
}

// EditorCommand builds an exec.Cmd for the given editor and file path,
// adding --wait for VS Code.
func EditorCommand(editor, path string) *exec.Cmd {
	if editor == "code" {
		return exec.Command(editor, "--wait", path)
	}
	return exec.Command(editor, path)
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
	editor = ResolveEditor(editor)
	if editor == "" {
		return "", fmt.Errorf("no editor found in PATH (tried config, $EDITOR, code, vi)")
	}
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
