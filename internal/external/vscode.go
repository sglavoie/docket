package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/util"
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

	tmpPath := f.Name()

	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpPath, nil
}

// SplitEditorString splits an editor command string that may contain flags
// (e.g. "nvim -u NORC" or "code --wait") into the executable and any
// pre-configured arguments. This handles the common pattern where $EDITOR,
// $VISUAL, or a config field contains both a binary and flags.
// Quoted segments and backslash escapes are handled correctly, so paths with
// spaces (e.g. '"/Applications/Visual Studio Code.app/.../code" --wait')
// and quoted flag values (e.g. 'nvim -u "/tmp/my vimrc"') work as expected.
func SplitEditorString(s string) (exe string, args []string) {
	fields := util.ShellTokenize(s)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// ResolveEditor returns the editor command to use, applying the fallback chain:
// configEditor → VISUAL env → EDITOR env → "code" → "vi".
// Each candidate is verified with exec.LookPath before being returned.
// Candidates may contain flags (e.g. "nvim -u NORC"); only the executable
// part is looked up, but the full string (including flags) is returned so
// callers can pass it to EditorCommand or SplitEditorString.
// Returns an error if no editor is found.
func ResolveEditor(configEditor string) (string, error) {
	candidates := []string{configEditor, os.Getenv("VISUAL"), os.Getenv("EDITOR"), "code"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		exe, _ := SplitEditorString(c)
		if _, err := exec.LookPath(exe); err == nil {
			return c, nil
		}
		fmt.Fprintf(os.Stderr, "warning: editor %q not found in PATH, trying next fallback\n", exe)
	}
	// Last resort: check if vi is available before returning it
	if _, err := exec.LookPath("vi"); err == nil {
		return "vi", nil
	}
	return "", fmt.Errorf("no editor found: set $VISUAL or $EDITOR, or install VS Code")
}

// stripWaitFlags removes blocking wait flags from a list of pre-configured
// editor arguments. This ensures OpenFolderCommand is always fire-and-forget,
// even when the editor is configured as e.g. "code --wait" or "subl -w".
// Only the unambiguous GUI-editor wait flags are stripped: --wait and -w.
func stripWaitFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if a == "--wait" || a == "-w" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// OpenFolderCommand builds an exec.Cmd for opening a folder in the given editor.
// Unlike EditorCommand, this never blocks: it does not add --wait and strips any
// blocking wait flags that may be present in the pre-configured editor string
// (e.g. "code --wait" → the folder opens fire-and-forget).
// Other pre-configured flags (e.g. "nvim -u NORC") are preserved.
func OpenFolderCommand(editor, path string) *exec.Cmd {
	exe, extraArgs := SplitEditorString(editor)
	args := extraArgs
	if needsWaitFlag(exe) {
		args = stripWaitFlags(extraArgs)
	}
	args = append(args, path)
	return exec.Command(exe, args...)
}

// EditorCommand builds an exec.Cmd for the given editor and file path,
// adding --wait for editors that need it (VS Code, Sublime Text, Zed).
// The editor string may contain pre-configured flags (e.g. "nvim -u NORC");
// these are prepended before the --wait flag and file path.
func EditorCommand(editor, path string) *exec.Cmd {
	exe, extraArgs := SplitEditorString(editor)
	if needsWaitFlag(exe) {
		args := append(extraArgs, "--wait", path)
		return exec.Command(exe, args...)
	}
	args := append(extraArgs, path)
	return exec.Command(exe, args...)
}

// needsWaitFlag returns true if the editor needs --wait to block until
// the file is closed. Checks the base name to handle full paths and
// variants like "code-insiders".
func needsWaitFlag(editor string) bool {
	// editor here is already the executable only (not a full command string).
	base := strings.ToLower(filepath.Base(editor))
	if base == "code" || base == "codium" || base == "cursor" || base == "subl" || base == "sublime" || base == "zed" {
		return true
	}
	if strings.HasPrefix(base, "code-") || strings.HasPrefix(base, "codium-") || strings.HasPrefix(base, "cursor-") {
		return true
	}
	return false
}

// EditInExternalEditor opens content in an external editor and returns the modified content.
func EditInExternalEditor(editor string, content string, extension string) (string, error) {
	resolvedEditor, resolveErr := ResolveEditor(editor)
	if resolveErr != nil {
		return "", resolveErr
	}
	editor = resolvedEditor

	tmpPath, err := CreateTempFileWithContent(content, "tt-edit-", extension)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpPath) }()

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
