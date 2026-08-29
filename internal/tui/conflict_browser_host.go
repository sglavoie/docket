package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// The shell side of the conflict browser. The browser itself knows only its
// rows and its loader; reading the store and mounting the overlay live here, so
// the AppModel stays the one place overlays are created and torn down.
//
// The store is read directly rather than through an internal/app port, for the
// same reason the status strip's count is: nothing on this path validates,
// mutates, or commits, so there is no application-layer policy for a port to
// hold.

// conflictStore is the workspace's bundle directory. It is nil only for a model
// with no runtime, which is what the Hub's preview painter builds.
func (m *AppModel) conflictStore() *storage.ConflictStore {
	if m == nil || m.runtime == nil {
		return nil
	}
	return storage.NewConflictStore(config.ConflictsDir(m.runtime.DataDir()))
}

// openConflictBrowser lists the recorded bundles and mounts the overlay over the
// current route. An empty workspace refuses with a status rather than opening an
// empty box: the count in the strip and the key that reads it then agree about
// the same directory.
func (m *AppModel) openConflictBrowser() tea.Msg {
	store := m.conflictStore()
	if store == nil {
		return nil
	}
	records, err := store.List()
	if err != nil {
		m.err = err
		return commandMessage(defaultStatusTimeout())
	}
	// The strip and this overlay count the same directory, so one listing settles
	// both rather than leaving the strip a tick behind what the user just read.
	m.outstandingConflicts = len(records)
	if len(records) == 0 {
		m.setInfoStatus(conflictBrowserEmptyStatus)
		return commandMessage(defaultStatusTimeout())
	}
	browser, err := NewConflictBrowserModel(records, store.Load)
	if err != nil {
		m.err = err
		return commandMessage(defaultStatusTimeout())
	}
	m.conflictBrowser = browser
	return nil
}
