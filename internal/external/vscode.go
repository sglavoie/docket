package external

import (
	"fmt"
	"os"
	"os/exec"
)

// VSCodeAvailable checks if VS Code is installed and available.
func VSCodeAvailable() bool {
	_, err := CachedCodePath()
	return err == nil
}

// OpenFile opens a file in VS Code.
func OpenFile(path string) error {
	cmd := exec.Command("code", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

// OpenFileAtLine opens a file in VS Code at a specific line.
func OpenFileAtLine(path string, line int) error {
	// VS Code uses --goto flag for line:column
	arg := fmt.Sprintf("%s:%d", path, line)
	cmd := exec.Command("code", "--goto", arg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

// OpenFolder opens a folder in VS Code.
func OpenFolder(path string) error {
	cmd := exec.Command("code", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

// OpenNewWindow opens a path in a new VS Code window.
func OpenNewWindow(path string) error {
	cmd := exec.Command("code", "--new-window", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

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
func ResolveEditor(configEditor string) string {
	if configEditor != "" {
		return configEditor
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env
	}
	return "code"
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
