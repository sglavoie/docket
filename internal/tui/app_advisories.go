package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"

	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/tui/conflict"
)

// The shell half of the advisory opt-out. advisories.go owns what an advisory
// is and the two helpers every ambient line routes through; this file owns the
// session value those helpers read, which is where the machine-local setting and
// the process-level `--no-advisories` flag meet.
//
// The suppression is session state on the shell rather than a read of the
// machine file, because two inputs decide it: the machine-local flag the menu
// toggle writes, and the process-level `--no-advisories` flag, which forces
// advisories hidden for one run and writes nothing. The toggle commits, so
// toggling back after a flagged launch persists "shown" — the state a user can
// see is the state that was saved.

// The session value itself lives on the feedback cluster beside the conflict
// count, because both are ambient status-strip inputs and both travel into an
// Outliner as the same kind of injected accessor; see feedback_controller.go.

// WithHiddenAdvisories starts the session with advisories suppressed. RunApp
// translates the `--no-advisories` flag into this option; the machine-local
// setting is read in NewAppModel, and either one hiding is enough.
func WithHiddenAdvisories() AppOption {
	return func(m *AppModel) { m.feedback.hideAdvisories = true }
}

// coreToggleAdvisories flips the machine-local setting and the session value
// together. It mirrors coreToggleMenuBar: chrome follows the terminal you are
// sitting at, so the durable home is machine.json rather than the workspace.
func (m *AppModel) coreToggleAdvisories() error {
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	machine.AdvisoriesHidden = !m.feedback.hideAdvisories
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save advisory visibility: %w", err)
	}
	m.feedback.hideAdvisories = machine.AdvisoriesHidden
	if m.feedback.hideAdvisories {
		m.setInfoStatus("Advisories hidden")
	} else {
		m.setInfoStatus("Advisories shown")
	}
	return nil
}

// The shell half of the outstanding-conflict indicator. conflict_indicator.go
// owns the line the Outliner's status strip draws; this file owns the count it
// draws, which is cached here so no render reads the filesystem.
//
// The store is read straight from the storage layer rather than through an
// internal/app port, following the direct storage.LoadProject reads the TUI
// already makes: nothing here validates, mutates, or commits, so there is no
// application-layer policy for a port to hold.

// countOutstandingConflicts reports how many conflict bundles the workspace has
// recorded. An absent directory is zero, not an error, and a read failure is
// also zero: an indicator that cannot be counted must not be asserted.
func countOutstandingConflicts(dataDir string) int {
	if dataDir == "" {
		return 0
	}
	records, err := storage.NewConflictStore(config.ConflictsDir(dataDir)).List()
	if err != nil {
		logging.Warn("counting outstanding conflicts: %v", err)
		return 0
	}
	return len(records)
}

// refreshOutstandingConflicts recounts the workspace's conflict bundles and
// re-reads the sync status beside them. It runs at launch, when a reload adopts
// a change, when `gc` opens or resolves, and after an exchange pass that could
// have moved either; it arms no timer of its own.
func (m *AppModel) refreshOutstandingConflicts() {
	if m == nil || m.domain.runtime == nil {
		return
	}
	// What another device delivered or left in conflict is as invisible to a
	// local mutation as a bundle another process wrote.
	m.refreshSyncStatus()
	m.recountOutstandingConflicts()
}

// recountOutstandingConflicts recounts the conflict bundles against the sync
// conflicts last read and redraws the sync line from the status last read. It
// is the 60s tick's whole refresh in a session exchanging on its own timer:
// deriving sync conflicts costs a walk of the history, and there every pass
// that could have changed them has already read them again.
func (m *AppModel) recountOutstandingConflicts() {
	if m == nil || m.domain.runtime == nil {
		return
	}
	m.renderSyncAdvisory()
	m.feedback.outstandingConflicts = countOutstandingConflicts(m.domain.runtime.DataDir()) + len(m.feedback.syncConflicts)
}

// refreshSyncStatus re-reads an enrolled workspace's sync status into the
// feedback cluster: the strip's sync line and the open sync conflicts the
// conflict count and `gc` include. A workspace that is not enrolled, a shell
// with no runtime, and a status that cannot be read all report nothing: an
// indicator that cannot be read must not be asserted. A read that failed is
// owed again, at either port, so the next exchange pass repeats it however
// little that pass changed.
func (m *AppModel) refreshSyncStatus() {
	if m == nil {
		return
	}
	m.applySyncStatus(readSyncStatus(m.domain.syncStatus, m.domain.syncConflicts))
}

// syncStatusRead is one read of the sync status and, when that status names
// open conflicts, of those conflicts. It holds only what was read, so it can be
// taken off the UI goroutine and applied on it.
type syncStatusRead struct {
	status       *engine.SyncStatus
	statusErr    error
	conflicts    []engine.ConflictView
	conflictsErr error
}

// readSyncStatus reads through the given ports. A missing status port reads
// nothing; a missing conflicts port reads no conflicts.
func readSyncStatus(
	status func() (*engine.SyncStatus, error),
	conflicts func(map[string]string) ([]engine.ConflictView, error),
) syncStatusRead {
	var read syncStatusRead
	if status == nil {
		return read
	}
	read.status, read.statusErr = status()
	if read.statusErr != nil || read.status == nil || !read.status.Enrolled ||
		len(read.status.Conflicts) == 0 || conflicts == nil {
		return read
	}
	read.conflicts, read.conflictsErr = conflicts(nil)
	return read
}

// applySyncStatus replaces the feedback cluster's sync status with a read.
func (m *AppModel) applySyncStatus(read syncStatusRead) {
	m.feedback.syncStatus, m.feedback.syncAdvisory, m.feedback.syncConflicts = nil, "", nil
	m.feedback.syncRefreshOwed = false
	if read.statusErr != nil {
		logging.Warn("reading sync status: %v", read.statusErr)
		m.feedback.syncRefreshOwed = true
		if m.automaticSync.err != nil {
			m.feedback.syncAdvisory = "sync: automatic exchange failed (tt sync doctor)"
		}
		return
	}
	status := read.status
	if status == nil || !status.Enrolled {
		return
	}
	m.feedback.syncStatus = status
	m.renderSyncAdvisory()
	if read.conflictsErr != nil {
		logging.Warn("reading sync conflicts: %v", read.conflictsErr)
		m.feedback.syncRefreshOwed = true
		return
	}
	if read.conflicts != nil {
		m.feedback.syncConflicts = syncConflictEntries(read.conflicts)
	}
}

// renderSyncAdvisory redraws the strip's sync line from the status last read,
// which is what keeps its age and the failure of a pass current between reads.
// With no status read it leaves the line as the last read left it.
func (m *AppModel) renderSyncAdvisory() {
	status := m.feedback.syncStatus
	if status == nil {
		return
	}
	m.feedback.syncAdvisory = syncAdvisoryText(status, time.Now())
	// A failure before reaching the transport (for example, a removed folder
	// configuration) has no persisted report to show in the ordinary status.
	if m.automaticSync.err != nil && (status.LastExchange == nil || status.LastExchange.Succeeded) {
		pending := strings.TrimPrefix(m.feedback.syncAdvisory, "sync: ")
		m.feedback.syncAdvisory = "sync: automatic exchange failed (tt sync doctor)"
		if pending != "" {
			m.feedback.syncAdvisory += " · " + pending
		}
	}
}

// syncAdvisoryText is the strip's sync line. Local saves, outgoing and
// incoming work, holds, and a failed exchange are named separately, because
// a commit written here is not a delivery; an enrolled workspace with nothing
// waiting draws no line at all.
func syncAdvisoryText(status *engine.SyncStatus, now time.Time) string {
	var parts []string
	if len(status.Upgrades) > 0 || len(status.Held) > 0 {
		parts = append(parts, "update tt to apply held changes (tt sync status)")
	}
	if last := status.LastExchange; last != nil && !last.Succeeded {
		parts = append(parts, "last exchange failed (tt sync doctor)")
	}
	if last := status.LastExchange; last != nil {
		if len(last.Unavailable) > 0 {
			parts = append(parts, util.Count(len(last.Unavailable), "file")+" waiting to download")
		}
		if last.Succeeded && len(last.Errors) > 0 {
			parts = append(parts, "exchange errors (tt sync doctor)")
		}
	}
	if status.OutstandingUp > 0 {
		parts = append(parts, fmt.Sprintf("%d saved here, not yet published", status.OutstandingUp))
	}
	if status.OutstandingDown > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting to apply", status.OutstandingDown))
	}
	if len(parts) == 0 {
		return ""
	}
	switch {
	case status.LastSuccess.IsZero():
		parts = append(parts, "never exchanged")
	default:
		parts = append(parts, "exchanged "+syncAge(now.Sub(status.LastSuccess))+" ago")
	}
	return "sync: " + strings.Join(parts, " · ")
}

// syncAge is a coarse duration: the line says how stale, not when.
func syncAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "<1m"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// syncConflictEntries converts the engine's conflict views into the rows the
// conflict browser lists.
func syncConflictEntries(views []engine.ConflictView) []conflict.SyncConflictEntry {
	if len(views) == 0 {
		return nil
	}
	entries := make([]conflict.SyncConflictEntry, 0, len(views))
	for _, view := range views {
		entry := conflict.SyncConflictEntry{ID: view.ID, Path: view.Path, Kind: view.Kind, Label: view.Label, Base: syncConflictContent(view.Base), BaseText: conflictEditableText(view.Base), CurrentText: conflictEditableText(view.Current), Replaceable: view.Replaceable}
		if view.Current != nil {
			entry.Result = view.Current.Digest
		}
		for _, candidate := range view.Versions {
			version := conflict.SyncConflictVersion{Index: candidate.Index, Text: conflictEditableText(&candidate.ConflictContent), Blob: candidate.Digest, Content: syncConflictContent(&candidate.ConflictContent), Current: candidate.Current}
			for _, writer := range candidate.Writers {
				version.Writers = append(version.Writers, conflict.SyncConflictWriter{
					TxnID: writer.TxnID, Device: writer.DeviceID, DeviceName: writer.DeviceName,
					ThisDevice: writer.ThisDevice, Change: writer.Change, At: writer.CreatedAt,
				})
			}
			for _, also := range candidate.AlsoResolves {
				version.AlsoResolves = append(version.AlsoResolves, also.Label)
			}
			entry.Versions = append(entry.Versions, version)
		}
		for _, detail := range view.Details {
			line := detail.Detail
			if detail.Subject != "" || detail.Field != "" {
				line = strings.TrimSpace(detail.Subject+" "+detail.Field) + ": " + detail.Detail
			}
			entry.Details = append(entry.Details, line)
		}
		entries = append(entries, entry)
	}
	return entries
}

// syncConflictContent is a version's content as the browser prints it: its
// text, or a sentence saying why there is none to print.
func syncConflictContent(content *engine.ConflictContent) []string {
	switch {
	case content == nil:
		return []string{"(the file did not exist)"}
	case content.Deleted:
		return []string{"(deleted)"}
	case !content.Available:
		return []string{"(not on this device)"}
	case content.Text == nil:
		return []string{fmt.Sprintf("(%d bytes, not text)", content.Size)}
	}
	return strings.Split(strings.TrimRight(*content.Text, "\n"), "\n")
}

// A deletion is an empty diff input; unavailable/binary content has no text.
func conflictEditableText(content *engine.ConflictContent) *string {
	if content == nil || content.Deleted {
		empty := ""
		return &empty
	}
	if !content.Available {
		return nil
	}
	return content.Text
}
