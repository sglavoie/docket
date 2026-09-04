package external

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// ErrClipboardUnsupported is what CopyText returns on a platform with no
// clipboard tool it knows how to drive. Callers match it with errors.Is to tell
// "this machine cannot copy" apart from "the copy failed", because the first is
// a status line and the second is an error.
var ErrClipboardUnsupported = errors.New("clipboard not supported on this OS")

// CopyText writes arbitrary text to the system clipboard by shelling out to the
// platform tool. It is a package var so tests can inject clipboard success,
// unsupported, and error paths without running that tool.
//
// It sits beside CopyTaskTitle rather than sharing its implementation: those
// three helpers project a task through the atotto/clipboard library and report
// no unsupported-platform sentinel, while every text copy in the TUI is written
// against this one. Merging them would change what a caller on an unsupported
// platform sees, so they stay two spellings of the same intent until something
// decides which one wins.
var CopyText = func(text string) error {
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		clipCmd = exec.Command("clip")
	default:
		return ErrClipboardUnsupported
	}

	clipCmd.Stdin = strings.NewReader(text)
	return clipCmd.Run()
}
