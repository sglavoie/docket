package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/tui/status"
)

// countedSyncSession is an enrolled session whose passes and sync status
// reads are counted.
type countedSyncSession struct {
	model  *AppModel
	passes int
	reads  int
}

func newCountedSyncSession(t *testing.T, rt *engine.Runtime) *countedSyncSession {
	t.Helper()
	model, err := NewAppModel(rt)
	if err != nil {
		t.Fatal(err)
	}
	session := &countedSyncSession{model: model}
	exchange, read := model.automaticSync.exchange, model.domain.syncStatus
	if exchange == nil || read == nil {
		t.Fatal("an enrolled session has no exchange worker or no sync status port")
	}
	model.automaticSync.exchange = func() (*syncexchange.FolderReport, error) {
		session.passes++
		return exchange()
	}
	model.domain.syncStatus = func() (*engine.SyncStatus, error) {
		session.reads++
		return read()
	}
	runSession2AppCommand(t, model, model.Init())
	return session
}

// pass runs one timer-driven exchange to completion and returns how many
// passes ran and how often the sync status was read meanwhile.
func (s *countedSyncSession) pass(t *testing.T) (passes, reads int) {
	t.Helper()
	s.passes, s.reads = 0, 0
	_, cmd := s.model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, s.model, cmd)
	return s.passes, s.reads
}

func TestAutomaticSyncReadsNoStatusAfterAnIdlePass(t *testing.T) {
	session := newCountedSyncSession(t, newEnrolledPair(t).desk)
	session.pass(t) // Whatever the launch left to settle.
	for range 3 {
		if passes, reads := session.pass(t); passes != 1 || reads != 0 {
			t.Fatalf("an idle pass ran %d exchanges and read the sync status %d times, want 1 and 0", passes, reads)
		}
	}
	session.reads = 0
	if handled, _ := session.model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if session.reads != 0 {
		t.Fatalf("the periodic tick read the sync status %d times in an enrolled session", session.reads)
	}
}

func TestAutomaticSyncSurfacesAnImportedConflict(t *testing.T) {
	pair := newEnrolledPair(t)
	session := newCountedSyncSession(t, pair.desk)
	session.pass(t)
	model := session.model
	if model.feedback.outstandingConflictCount() != 0 {
		t.Fatalf("conflict count before any conflict = %d", model.feedback.outstandingConflictCount())
	}

	retitleOnDisk(t, pair.desk, "Desk title")
	retitleOnDisk(t, pair.laptop, "Laptop title")
	exchangeOnce(t, pair.laptop)
	if _, reads := session.pass(t); reads == 0 {
		t.Fatal("a pass that imported a conflict did not read the sync status")
	}
	if model.feedback.outstandingConflictCount() != 1 || len(model.feedback.syncConflicts) != 1 {
		t.Fatalf("conflict count = %d with %d sync conflicts, want the one imported conflict",
			model.feedback.outstandingConflictCount(), len(model.feedback.syncConflicts))
	}
	if strip := model.crossOutliner.StatusStripBody(160); !strings.Contains(strip, "1 unresolved merge conflict — gc to review") {
		t.Fatalf("status strip does not count the imported conflict:\n%s", strip)
	}
	model.openConflictBrowser()
	if model.conflictBrowser == nil {
		t.Fatal("gc did not open the browser over the imported conflict")
	}
	if rendered := model.conflictBrowser.RenderWithin(160, 40); !strings.Contains(rendered, "sync conflict · 2 versions") {
		t.Fatalf("browser does not list the imported conflict:\n%s", rendered)
	}

	// The conflict stands, so the passes after it are idle again and the tick
	// keeps counting it without reading.
	session.pass(t)
	if passes, reads := session.pass(t); passes != 1 || reads != 0 {
		t.Fatalf("an idle pass over an open conflict ran %d exchanges and read the sync status %d times", passes, reads)
	}
	model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()})
	if session.reads != 0 || model.feedback.outstandingConflictCount() != 1 {
		t.Fatalf("the periodic tick read the sync status %d times and left the count at %d",
			session.reads, model.feedback.outstandingConflictCount())
	}
}

func TestAutomaticSyncRetriesFailedRefresh(t *testing.T) {
	for _, tc := range []struct {
		name string
		// statusFails names the port the failed refresh stops at: the sync status
		// read, or the conflict view read under a status that was read.
		statusFails bool
	}{
		{name: "status_read", statusFails: true},
		{name: "conflict_read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pair := newEnrolledPair(t)
			session := newCountedSyncSession(t, pair.desk)
			session.pass(t)
			model := session.model
			retitleOnDisk(t, pair.desk, "Desk title")
			retitleOnDisk(t, pair.laptop, "Laptop title")
			exchangeOnce(t, pair.laptop)
			session.pass(t)
			session.pass(t)
			if model.feedback.outstandingConflictCount() != 1 || len(model.feedback.syncConflicts) != 1 {
				t.Fatalf("conflict count = %d with %d sync conflicts before the failure, want the one open conflict",
					model.feedback.outstandingConflictCount(), len(model.feedback.syncConflicts))
			}

			failing, viewReads := true, 0
			readStatus, readViews := model.domain.syncStatus, model.domain.syncConflicts
			model.domain.syncStatus = func() (*engine.SyncStatus, error) {
				if failing && tc.statusFails {
					session.reads++
					return nil, errors.New("sync status unreadable")
				}
				return readStatus()
			}
			model.domain.syncConflicts = func(choices map[string]string) ([]engine.ConflictView, error) {
				viewReads++
				if failing && !tc.statusFails {
					return nil, errors.New("sync conflicts unreadable")
				}
				return readViews(choices)
			}

			// A refresh nothing about a pass asked for fails at the named port.
			session.reads = 0
			model.refreshOutstandingConflicts()
			if failed := map[bool]int{true: session.reads, false: viewReads}[tc.statusFails]; failed != 1 {
				t.Fatalf("the failing refresh reached the %s port %d times, want 1", tc.name, failed)
			}
			if model.feedback.outstandingConflictCount() != 0 || len(model.feedback.syncConflicts) != 0 {
				t.Fatalf("an unreadable refresh still asserts %d conflicts over %d sync conflicts",
					model.feedback.outstandingConflictCount(), len(model.feedback.syncConflicts))
			}

			// The port recovers; the pass that follows changed nothing and owes no
			// reload, and is still what reads again.
			failing, viewReads = false, 0
			if model.externalReload.owed {
				t.Fatal("an external reload is owed before the unchanged pass")
			}
			outcome := model.automaticSync.outcome
			passes, reads := session.pass(t)
			if passes != 1 || model.externalReload.owed || model.automaticSync.outcome != outcome {
				t.Fatalf("the recovering pass ran %d exchanges, owed a reload = %t, and moved the outcome from %q to %q",
					passes, model.externalReload.owed, outcome, model.automaticSync.outcome)
			}
			if reads != 1 || viewReads != 1 {
				t.Fatalf("the recovering pass read the sync status %d times and the conflict views %d times, want 1 and 1", reads, viewReads)
			}
			if model.feedback.outstandingConflictCount() != 1 || len(model.feedback.syncConflicts) != 1 {
				t.Fatalf("conflict count = %d with %d sync conflicts after recovery, want the one open conflict",
					model.feedback.outstandingConflictCount(), len(model.feedback.syncConflicts))
			}
			if strip := model.crossOutliner.StatusStripBody(160); !strings.Contains(strip, "1 unresolved merge conflict — gc to review") {
				t.Fatalf("status strip does not count the recovered conflict:\n%s", strip)
			}
			model.openConflictBrowser()
			if model.conflictBrowser == nil {
				t.Fatal("gc did not open the browser over the recovered conflict")
			}
			if rendered := model.conflictBrowser.RenderWithin(160, 40); !strings.Contains(rendered, "sync conflict · 2 versions") {
				t.Fatalf("browser does not list the recovered conflict:\n%s", rendered)
			}
			model.conflictBrowser = nil

			// Recovered, the obligation is spent: the passes after it are idle.
			viewReads = 0
			if passes, reads := session.pass(t); passes != 1 || reads != 0 || viewReads != 0 {
				t.Fatalf("a healthy pass after recovery ran %d exchanges, read the sync status %d times and the conflict views %d times",
					passes, reads, viewReads)
			}
			session.reads = 0
			model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()})
			if session.reads != 0 || model.feedback.outstandingConflictCount() != 1 {
				t.Fatalf("the periodic tick read the sync status %d times and left the count at %d",
					session.reads, model.feedback.outstandingConflictCount())
			}
		})
	}
}
