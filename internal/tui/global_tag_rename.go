package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// renameGlobalTag renames a tag across every project that uses it through the
// application-layer workspace tag-rename use case. That use case renames the
// normalized tag across each affected project's catalog, pinned set, and task
// usages atomically, committing every touched project as one transaction and
// recording a single grouped undo/redo entry.
func (m *GlobalTagManagerModel) renameGlobalTag(oldName, newName string) tea.Cmd {
	// Normalize and no-op on an empty/unchanged rename, matching the legacy
	// synchronous guard before any work is dispatched.
	oldName = models.NormalizeTag(oldName)
	newName = models.NormalizeTag(newName)
	if newName == "" || newName == oldName {
		return nil
	}

	coordinator := m.coordinator
	return func() tea.Msg {
		res, err := coordinator.RenameWorkspace(oldName, newName)
		if err != nil {
			return GlobalTagManagerErrorMsg{Err: globalTagRenameError(err)}
		}
		return GlobalTagRenamedMsg{
			OldName:      res.OldTag,
			NewName:      res.NewTag,
			ProjectCount: res.ProjectCount,
			TaskCount:    res.TaskCount,
		}
	}
}

// globalTagRenameError unwraps an application-layer invalid-input error to its
// bare reason so the surfaced message reads as a clean validation error rather
// than the app-layer envelope.
func globalTagRenameError(err error) error {
	var invalid *app.InvalidInputError
	if errors.As(err, &invalid) {
		return errors.New(invalid.Reason)
	}
	return err
}
