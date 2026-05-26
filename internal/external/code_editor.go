package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/logging"
)

// editorPattern defines how an editor handles opening a file at a specific line.
type editorPattern int

const (
	// patternGoto uses --goto file:line (VS Code, Cursor)
	patternGoto editorPattern = iota
	// patternPlusLine uses +line file (vim, nvim, emacs, nano)
	patternPlusLine
	// patternColonLine uses file:line (Sublime, Zed)
	patternColonLine
	// patternLineFlag uses --line N file (JetBrains IDEs)
	patternLineFlag
	// patternUnknown uses file only (unknown editors)
	patternUnknown
)

// detectEditorPattern determines how to invoke the editor with a line number.
func detectEditorPattern(editor string) editorPattern {
	// Strip any flags (e.g. "nvim -u NORC") before base-name detection.
	exe, _ := SplitEditorString(editor)
	// Get base name of editor (handle paths like /usr/bin/vim)
	base := strings.ToLower(filepath.Base(exe))

	// VS Code and Cursor use --goto file:line
	if base == "code" || base == "cursor" ||
		strings.HasPrefix(base, "code-") || strings.HasPrefix(base, "cursor-") {
		return patternGoto
	}

	// Terminal editors use +line file
	switch base {
	case "vim", "nvim", "vi", "nano", "emacs", "emacsclient", "pico", "hx", "helix":
		return patternPlusLine
	}

	// Sublime and Zed use file:line
	if base == "subl" || base == "sublime" || base == "zed" {
		return patternColonLine
	}

	// JetBrains IDEs use --line N file
	if base == "idea" || base == "goland" || base == "pycharm" ||
		base == "webstorm" || base == "phpstorm" || base == "clion" ||
		base == "rider" || base == "rubymine" || base == "datagrip" {
		return patternLineFlag
	}

	return patternUnknown
}

// OpenCodeReference opens a file at a specific line in the configured editor.
func OpenCodeReference(editor, path string, line int) error {
	if editor == "" {
		return fmt.Errorf("no editor configured: set code_editor in ~/.tt.json")
	}
	exe, extraArgs := SplitEditorString(editor)
	if _, err := exec.LookPath(exe); err != nil {
		return fmt.Errorf("editor %q not found in PATH", exe)
	}
	pattern := detectEditorPattern(editor)

	var cmd *exec.Cmd
	switch pattern {
	case patternGoto:
		// code --goto file:line
		args := append([]string{}, extraArgs...)
		if line > 0 {
			args = append(args, "--goto", path+":"+strconv.Itoa(line))
		} else {
			args = append(args, path)
		}
		cmd = exec.Command(exe, args...)
	case patternPlusLine:
		// vim +line file
		args := append([]string{}, extraArgs...)
		if line > 0 {
			args = append(args, "+"+strconv.Itoa(line))
		}
		args = append(args, path)
		cmd = exec.Command(exe, args...)
	case patternColonLine:
		// subl file:line
		args := append([]string{}, extraArgs...)
		if line > 0 {
			args = append(args, path+":"+strconv.Itoa(line))
		} else {
			args = append(args, path)
		}
		cmd = exec.Command(exe, args...)
	case patternLineFlag:
		// idea --line N file
		args := append([]string{}, extraArgs...)
		if line > 0 {
			args = append(args, "--line", strconv.Itoa(line))
		}
		args = append(args, path)
		cmd = exec.Command(exe, args...)
	default:
		// Unknown editor: just open the file
		cmd = exec.Command(exe, append(extraArgs, path)...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Terminal editors need stdin and must block until exit. Unknown
	// editors are assumed terminal too — running them async without
	// stdin causes helix/hx and similar to fail silently.
	if pattern == patternPlusLine || pattern == patternUnknown {
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	// GUI editors run in the background; reap the child process in a
	// goroutine to prevent zombie processes.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening editor: %w", err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			logging.Warn("external tool exited with error: %v", err)
		}
	}()
	return nil
}
