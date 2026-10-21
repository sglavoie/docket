package bridge

import (
	"path/filepath"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/synclog"
)

// syncStatusResult is the engine's side of sync status: what is committed
// locally and not yet published, what each peer has delivered, and the open
// conflicts. Whether the exchange is reachable and when it last succeeded is
// the transport's to report, so it is not here.
type syncStatusResult struct {
	DeviceID  string            `json:"device_id"`
	Indexed   bool              `json:"indexed"`
	Heads     []string          `json:"heads"`
	Outbox    []outboxView      `json:"outbox"`
	Received  map[string]uint64 `json:"received"`
	Conflicts []string          `json:"conflicts"`
}

type outboxView struct {
	TxnID     string `json:"txn_id"`
	DeviceSeq uint64 `json:"device_seq"`
}

type exportResult struct {
	DeviceID string                  `json:"device_id"`
	Pending  []syncexchange.Outgoing `json:"pending"`
	// External is the observation export ran first, so edits made outside a
	// commit since the last one are part of what it lists.
	External *storage.ExternalChanges `json:"external,omitempty"`
}

type importPayload struct {
	InboxDir string `json:"inbox_dir"`
}

type acknowledgePayload struct {
	TxnIDs []string `json:"txn_ids"`
}

// syncCommits is the open workspace's commit boundary, refusing a workspace
// that records no sync history.
func (b *Bridge) syncCommits() (*storage.CommitService, error) {
	commits := b.workspace.runtime.SyncCommits()
	if commits.SyncHistory() == nil {
		return nil, newError(CodeRefused, "this workspace records no sync history")
	}
	return commits, nil
}

func (b *Bridge) syncStatus(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	commits, err := b.syncCommits()
	if err != nil {
		return nil, err
	}
	history := commits.SyncHistory()
	result := syncStatusResult{
		DeviceID:  history.DeviceID(),
		Heads:     []string{},
		Outbox:    []outboxView{},
		Received:  map[string]uint64{},
		Conflicts: []string{},
	}
	state, err := history.LoadState()
	if err != nil {
		return nil, err
	}
	if state != nil {
		result.Indexed = state.Indexed
		result.Heads = append(result.Heads, state.Heads...)
		for _, entry := range state.Outbox {
			result.Outbox = append(result.Outbox, outboxView{TxnID: entry.TxnID, DeviceSeq: entry.DeviceSeq})
		}
		for device, seq := range state.Received {
			result.Received[device] = seq
		}
	}
	conflicts, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	for _, conflict := range conflicts {
		result.Conflicts = append(result.Conflicts, conflict.ID)
	}
	return result, nil
}

// syncExportPending observes external changes first, so an edit made outside
// a commit is published like one, then lists the outbox with every file a
// transport has to copy.
func (b *Bridge) syncExportPending(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	commits, err := b.syncCommits()
	if err != nil {
		return nil, err
	}
	external, err := commits.CommitExternalChanges()
	if err != nil {
		return nil, err
	}
	pending, err := syncexchange.Pending(commits.SyncHistory())
	if err != nil {
		return nil, err
	}
	if external.TransactionID == "" && len(external.Held) == 0 {
		external = nil
	}
	return exportResult{DeviceID: commits.SyncHistory().DeviceID(), Pending: pending, External: external}, nil
}

// syncImport applies what a transport staged into inbox_dir, then refreshes
// the snapshot so queries read the imported work.
func (b *Bridge) syncImport(request Request) (any, error) {
	var payload importPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.InboxDir == "" || !filepath.IsAbs(payload.InboxDir) {
		return nil, newError(CodeInvalidRequest, "inbox_dir must be an absolute path")
	}
	commits, err := b.syncCommits()
	if err != nil {
		return nil, err
	}
	workspace := b.workspace
	report, err := syncexchange.ImportInbox(yieldingImport{b, workspace, commits}, filepath.Clean(payload.InboxDir))
	if err != nil {
		return nil, err
	}
	if b.workspace != workspace {
		return nil, newError(CodeWorkspaceMismatch, "the workspace changed during sync")
	}
	for _, result := range report.Results {
		if result.Outcome == syncexchange.Imported {
			if err := workspace.runtime.Refresh(); err != nil {
				return nil, err
			}
			break
		}
	}
	return report, nil
}

// An open workspace imports one transaction at a time, letting queued local
// requests run between them. Initial joins still use the batched bootstrap in
// ImportInbox directly. A close/reopen invalidates the captured workspace even
// when it has the same ID; an old pass cannot write into its replacement.
type yieldingImport struct {
	bridge    *Bridge
	workspace *openWorkspace
	commits   *storage.CommitService
}

func (i yieldingImport) SyncHistory() *synclog.History { return i.commits.SyncHistory() }

func (i yieldingImport) CommitIncoming(manifest *synclog.Manifest, blobs synclog.BlobSource) (string, error) {
	i.bridge.requests.yieldBackground()
	if i.bridge.workspace != i.workspace {
		return "", newError(CodeWorkspaceMismatch, "the workspace changed during sync")
	}
	return i.commits.CommitIncoming(manifest, blobs)
}

// syncAcknowledge records that the transport durably published the named
// outgoing transactions.
func (b *Bridge) syncAcknowledge(request Request) (any, error) {
	var payload acknowledgePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	for _, id := range payload.TxnIDs {
		if !synclog.ValidDigest(id) {
			return nil, newError(CodeInvalidRequest, "txn_id %q is not a content address", id)
		}
	}
	commits, err := b.syncCommits()
	if err != nil {
		return nil, err
	}
	if err := commits.CommitSyncPublished(payload.TxnIDs); err != nil {
		return nil, err
	}
	return struct {
		Acknowledged []string `json:"acknowledged"`
	}{append([]string{}, payload.TxnIDs...)}, nil
}
