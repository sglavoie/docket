package status

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

// IsExhaustedHistory reports a restore refused only because the ledger holds
// nothing further in that direction, which is an answer rather than a failure.
func IsExhaustedHistory(err error) bool {
	return errors.Is(err, storage.ErrNoUndoHistory) || errors.Is(err, storage.ErrNoRedoHistory)
}

// NothingToRestoreCmd is that answer, spoken quietly on every surface.
func NothingToRestoreCmd(redo bool) tea.Cmd {
	text := "Nothing to undo"
	if redo {
		text = "Nothing to redo"
	}
	return func() tea.Msg { return InfoStatusMsg(text) }
}

func FormatUndoRedoRestoreError(label string, err error) error {
	if err == nil {
		return nil
	}
	return errors.New(formatUndoRedoRestoreErrorMessage(label, err))
}

func formatUndoRedoRestoreErrorMessage(label string, err error) string {
	action := undoRedoRestoreActionLabel(label)
	detail := strings.ToLower(err.Error())

	switch {
	case IsExhaustedHistory(err):
		return fmt.Sprintf("%s failed: no %s history is available for this workspace", action, label)
	case strings.Contains(detail, "diverged") || strings.Contains(detail, "does not match"):
		return fmt.Sprintf("%s failed: grouped history diverged; restore aborted: %s", action, err)
	case strings.Contains(detail, "missing project") ||
		strings.Contains(detail, "missing "+label+" history") ||
		strings.Contains(detail, "failed to read "+label+" snapshot") ||
		strings.Contains(detail, "failed to load "+label+" metadata"):
		return fmt.Sprintf("%s failed: %s history is partially unavailable; restore aborted: %s", action, label, err)
	default:
		return fmt.Sprintf("%s failed: %s", action, err)
	}
}

func undoRedoRestoreActionLabel(label string) string {
	switch label {
	case "undo":
		return "Undo"
	case "redo":
		return "Redo"
	default:
		if label == "" {
			return "Restore"
		}
		return strings.ToUpper(label[:1]) + label[1:]
	}
}
