package external

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/sglavoie/tt/internal/logging"
)

// ReferenceEditorClass describes whether an editor can be used for async
// reference opens from inside the running TUI.
type ReferenceEditorClass string

const (
	ReferenceEditorGUI      ReferenceEditorClass = "gui"
	ReferenceEditorTerminal ReferenceEditorClass = "terminal"
	ReferenceEditorUnknown  ReferenceEditorClass = "unknown"
)

// ReferenceOpenStatus is the stable status contract returned by
// OpenReferenceFile.
type ReferenceOpenStatus string

const (
	ReferenceOpenStatusOpened            ReferenceOpenStatus = "opened"
	ReferenceOpenStatusResolveFailed     ReferenceOpenStatus = "resolve_failed"
	ReferenceOpenStatusUnsupportedEditor ReferenceOpenStatus = "unsupported_editor"
	ReferenceOpenStatusLaunchFailed      ReferenceOpenStatus = "launch_failed"
)

// ErrReferenceEditorUnsupported marks terminal or unknown editors rejected for
// async reference mode. Callers can use errors.Is while still showing the
// returned ReferenceOpenStatus to users.
var ErrReferenceEditorUnsupported = errors.New("unsupported reference editor")

// ReferenceOpenResult carries the status contract needed by TUI callers.
type ReferenceOpenResult struct {
	Status ReferenceOpenStatus
	Class  ReferenceEditorClass
	Editor string
	Path   string
}

// ResolveReferenceEditor resolves the configured external editor for reference
// opens. Callers should pass the value from config.GetExternalEditor().
func ResolveReferenceEditor(configuredExternalEditor string) (string, error) {
	editor, err := ResolveEditor(configuredExternalEditor)
	if err != nil {
		return "", fmt.Errorf("resolve reference editor: %w", err)
	}
	return editor, nil
}

// ClassifyReferenceEditor reports whether the editor can run safely as an
// async reference opener. Terminal and unknown editors are intentionally
// rejected because they cannot share the active tt terminal fire-and-forget.
func ClassifyReferenceEditor(editor string) ReferenceEditorClass {
	switch detectEditorPattern(editor) {
	case patternGoto, patternColonLine, patternLineFlag:
		return ReferenceEditorGUI
	case patternPlusLine:
		return ReferenceEditorTerminal
	default:
		return ReferenceEditorUnknown
	}
}

// ReferenceEditorCommand builds a non-blocking command for opening path as a
// read-only-by-convention reference. It strips common GUI wait flags and
// rejects terminal or unknown editors.
func ReferenceEditorCommand(editor, path string) (*exec.Cmd, ReferenceEditorClass, error) {
	class := ClassifyReferenceEditor(editor)
	if class != ReferenceEditorGUI {
		return nil, class, unsupportedReferenceEditorError(editor, class)
	}

	exe, extraArgs := SplitEditorString(editor)
	args := stripWaitFlags(extraArgs)
	args = append(args, path)
	return exec.Command(exe, args...), class, nil
}

// OpenReferenceFile opens path in a GUI editor without waiting for edits to be
// made. The file is a reference copy; callers must not import changes back from
// this path.
func OpenReferenceFile(editor, path string) (ReferenceOpenResult, error) {
	result := ReferenceOpenResult{
		Path: path,
	}

	resolvedEditor, err := ResolveReferenceEditor(editor)
	if err != nil {
		result.Status = ReferenceOpenStatusResolveFailed
		return result, err
	}

	result.Editor = resolvedEditor
	cmd, class, err := ReferenceEditorCommand(resolvedEditor, path)
	result.Class = class
	if err != nil {
		result.Status = ReferenceOpenStatusUnsupportedEditor
		return result, err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		result.Status = ReferenceOpenStatusLaunchFailed
		return result, fmt.Errorf("opening reference editor: %w", err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logging.Warn("external tool exited with error: %v", err)
		}
	}()

	result.Status = ReferenceOpenStatusOpened
	return result, nil
}

func unsupportedReferenceEditorError(editor string, class ReferenceEditorClass) error {
	exe, _ := SplitEditorString(editor)
	if exe == "" {
		exe = editor
	}
	return fmt.Errorf("%w: %q is %s; configure external_editor to a GUI editor for async reference opens", ErrReferenceEditorUnsupported, exe, class)
}
