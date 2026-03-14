package external

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	// Get base name of editor (handle paths like /usr/bin/vim)
	base := strings.ToLower(filepath.Base(editor))

	// VS Code and Cursor use --goto file:line
	if base == "code" || base == "cursor" || strings.HasPrefix(base, "code-") {
		return patternGoto
	}

	// Terminal editors use +line file
	switch base {
	case "vim", "nvim", "vi", "nano", "emacs", "emacsclient", "pico":
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
	pattern := detectEditorPattern(editor)

	var cmd *exec.Cmd
	switch pattern {
	case patternGoto:
		// code --goto file:line
		arg := path + ":" + strconv.Itoa(line)
		cmd = exec.Command(editor, "--goto", arg)
	case patternPlusLine:
		// vim +line file
		cmd = exec.Command(editor, "+"+strconv.Itoa(line), path)
	case patternColonLine:
		// subl file:line
		cmd = exec.Command(editor, path+":"+strconv.Itoa(line))
	case patternLineFlag:
		// idea --line N file
		cmd = exec.Command(editor, "--line", strconv.Itoa(line), path)
	default:
		// Unknown editor: just open the file
		cmd = exec.Command(editor, path)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Terminal editors need stdin and must block until exit
	if pattern == patternPlusLine {
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	// GUI editors run in the background; reap the child process in a
	// goroutine to prevent zombie processes.
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
