package engine

import (
	"time"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

// SyncDiagnostics is what a person sends when sync misbehaves: the device's
// sync status, the conflicts' structure, and what the transport reported. It
// names files, devices, and transactions, never file content.
type SyncDiagnostics struct {
	Format      string    `json:"format"`
	GeneratedAt time.Time `json:"generated_at"`
	DeviceID    string    `json:"device_id"`
	// WorkspaceDigest digests the synced files (synclog.WorkspaceDigest):
	// two devices that converged report the same value.
	WorkspaceDigest string                   `json:"workspace_digest,omitempty"`
	Status          *SyncStatus              `json:"status"`
	Outbox          []string                 `json:"outbox"`
	Conflicts       []DiagnosticConflict     `json:"conflicts"`
	External        *storage.ExternalChanges `json:"external,omitempty"`
}

// DiagnosticConflict is a conflict without content.
type DiagnosticConflict struct {
	ID       string                   `json:"id"`
	Path     string                   `json:"path"`
	Kind     string                   `json:"kind"`
	Base     string                   `json:"base,omitempty"`
	Current  string                   `json:"current,omitempty"`
	Versions []DiagnosticVersion      `json:"versions"`
	Details  []synclog.ConflictDetail `json:"details"`
}

// DiagnosticVersion is one competing version's digest and writers.
type DiagnosticVersion struct {
	Digest    string           `json:"digest,omitempty"`
	Available bool             `json:"available"`
	Writers   []ConflictWriter `json:"writers"`
}

// SyncDiagnosticsFormat names the diagnostics document.
const SyncDiagnosticsFormat = "tt-sync-diagnostics/v1"

// SyncDiagnostics gathers diagnostics without writing anything; the status
// read never observes, publishes, or imports.
func (r *Runtime) SyncDiagnostics(names map[string]string) (*SyncDiagnostics, error) {
	status, err := r.SyncStatus()
	if err != nil {
		return nil, err
	}
	status.Conflicts = nil
	report := &SyncDiagnostics{Format: SyncDiagnosticsFormat, GeneratedAt: r.Now().UTC(), Status: status, Outbox: []string{}, Conflicts: []DiagnosticConflict{}}
	history := r.repo.svc.SyncHistory()
	if history == nil {
		return report, nil
	}
	pending, err := history.Pending()
	if err != nil {
		return nil, err
	}
	report.DeviceID = history.DeviceID()
	if report.WorkspaceDigest, err = synclog.WorkspaceDigest(r.DataDir()); err != nil {
		return nil, err
	}
	for _, manifest := range pending {
		report.Outbox = append(report.Outbox, manifest.TxnID)
	}
	views, err := r.SyncConflicts(names)
	if err != nil {
		return nil, err
	}
	for _, view := range views {
		entry := DiagnosticConflict{ID: view.ID, Path: view.Path, Kind: view.Kind, Versions: []DiagnosticVersion{}, Details: view.Details}
		if view.Base != nil {
			entry.Base = view.Base.Digest
		}
		if view.Current != nil {
			entry.Current = view.Current.Digest
		}
		for _, version := range view.Versions {
			entry.Versions = append(entry.Versions, DiagnosticVersion{Digest: version.Digest, Available: version.Available, Writers: version.Writers})
		}
		report.Conflicts = append(report.Conflicts, entry)
	}
	return report, nil
}
