package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/util"
)

// CreateTempFileWithContent creates a temporary file with the given content.
// Useful for editing task descriptions in external editors.
func CreateTempFileWithContent(content string, prefix string, suffix string) (string, error) {
	return createTempFileWithContentInDir(os.TempDir(), content, prefix, suffix)
}

func createTempFileWithContentInDir(dir string, content string, prefix string, suffix string) (string, error) {
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

// ResolveNoteEditor resolves the deliberately environment-only editor contract
// used by the combined Notes tool. Notes do not consult machine settings or
// silently fall back to an installed editor: the first available executable in
// DOCKET_EDITOR, VISUAL, then EDITOR order wins.
func ResolveNoteEditor() (string, error) {
	type candidate struct {
		name  string
		value string
	}
	candidates := []candidate{
		{name: "DOCKET_EDITOR", value: os.Getenv("DOCKET_EDITOR")},
		{name: "VISUAL", value: os.Getenv("VISUAL")},
		{name: "EDITOR", value: os.Getenv("EDITOR")},
	}
	var unavailable []string
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.value) == "" {
			continue
		}
		exe, _ := SplitEditorString(candidate.value)
		if exe == "" {
			continue
		}
		if _, err := exec.LookPath(exe); err == nil {
			return candidate.value, nil
		}
		unavailable = append(unavailable, fmt.Sprintf("$%s (%s)", candidate.name, exe))
	}
	if len(unavailable) > 0 {
		return "", fmt.Errorf(
			"no configured notes editor is executable: checked %s; set DOCKET_EDITOR, VISUAL, or EDITOR to an editor command available in PATH",
			strings.Join(unavailable, ", "),
		)
	}
	return "", fmt.Errorf("no notes editor configured: set DOCKET_EDITOR, VISUAL, or EDITOR to an editor command")
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
