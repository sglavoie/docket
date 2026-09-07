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
// It is the only clipboard write in the product. Task text is projected by the
// application layer and arrives here already assembled, which is what lets one
// call carry a whole selection: nothing else may write the clipboard, so no
// caller can put half a selection on it.
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
