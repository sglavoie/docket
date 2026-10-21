package syncexchange

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/synclog"
)

// Outcome is what importing one staged manifest came to.
type Outcome string

const (
	// Imported: applied, with its receipt, in one local transaction.
	Imported Outcome = "imported"
	// Known: already applied, or authored here; the staged copy is spent.
	Known Outcome = "known"
	// Incomplete: a parent, an earlier sequence number, or required content is
	// not here yet. The staged copy waits; nothing was applied.
	Incomplete Outcome = "incomplete"
	// Held: this device cannot read the manifest's format, schemas, or
	// capabilities yet. It waits for an upgrade and is never dropped.
	Held Outcome = "held"
	// Invalid: the staged bytes are not the transaction their name promises.
	// The transport discards this copy and fetches the manifest again.
	Invalid Outcome = "invalid"
	// Foreign: the manifest belongs to another workspace.
	Foreign Outcome = "foreign"
	// Failed: applying it failed locally (for example an unrecorded edit to a
	// file it writes). It is retried on the next exchange.
	Failed Outcome = "failed"
)

// Committer is the commit boundary an import applies through.
type Committer interface {
	SyncHistory() *synclog.History
	CommitIncoming(manifest *synclog.Manifest, blobs synclog.BlobSource) (string, error)
}

// bootstrapCommitter can install a complete history into an empty working
// copy in one checked transaction. Existing copies and incomplete batches
// decline this optimization and retain ordinary per-item import semantics.
type bootstrapCommitter interface {
	BootstrapIncoming([]*synclog.Manifest, synclog.BlobSource) (string, bool, error)
}

// Result reports one staged manifest.
type Result struct {
	Path      string  `json:"path"`
	TxnID     string  `json:"txn_id"`
	DeviceID  string  `json:"device_id"`
	DeviceSeq uint64  `json:"device_seq"`
	Outcome   Outcome `json:"outcome"`
	Reason    string  `json:"reason,omitempty"`
	// Upgrade names the update a held manifest waits for: which device has to
	// update tt, and why.
	Upgrade string `json:"upgrade,omitempty"`
	// LocalTxnID is the journal transaction that applied an imported manifest.
	LocalTxnID string `json:"local_txn_id,omitempty"`
	// Missing lists the required blobs not yet staged or retained.
	Missing []string `json:"missing,omitempty"`
	// Unavailable lists the workspace paths whose new content is among the
	// missing blobs: what a status surface shows as not yet downloaded. The
	// local files stay as they are; absence is never a deletion.
	Unavailable []string `json:"unavailable,omitempty"`
}

// Report is one import pass over an inbox.
type Report struct {
	Results []Result `json:"results"`
}

type staged struct {
	result   Result
	manifest *synclog.Manifest
}

// ImportInbox applies every manifest a transport staged under inbox, laid out
// as the exchange is (txns/ and blobs/ beneath it). A manifest is applied only
// once every blob it names is present and matches its digest, and only through
// the checked commit boundary, which applies it and records its receipt in one
// local transaction. Manifests are tried in sequence order, repeatedly, until
// a pass applies nothing, so one staged batch may arrive in any order. It
// reads the inbox and never writes it: the transport removes what the report
// says is spent.
func ImportInbox(committer Committer, inbox string) (*Report, error) {
	history := committer.SyncHistory()
	if history == nil {
		return nil, fmt.Errorf("syncexchange: import requires a sync history")
	}
	blobs := inboxBlobs{dir: inbox, history: history}
	var queue []*staged
	report := &Report{Results: []Result{}}
	root := filepath.Join(inbox, "txns")
	err := filepath.WalkDir(root, func(abs string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrNotExist) && abs == root {
				return filepath.SkipDir
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(inbox, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		deviceID, seq, txnID, ok := ParseManifestPath(rel)
		if !ok {
			return nil
		}
		item := &staged{result: Result{Path: rel, TxnID: txnID, DeviceID: deviceID, DeviceSeq: seq}}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		manifest, err := synclog.DecodeManifest(data)
		switch {
		case errors.Is(err, synclog.ErrHeld):
			item.result.Outcome, item.result.Reason = Held, err.Error()
			item.result.Upgrade = synclog.RequiredUpgrade(err, deviceID)
		case err != nil:
			item.result.Outcome, item.result.Reason = Invalid, err.Error()
		case manifest.TxnID != txnID || manifest.DeviceID != deviceID || manifest.DeviceSeq != seq:
			item.result.Outcome = Invalid
			item.result.Reason = fmt.Sprintf("staged as %s but carries %s from %s seq %d", rel, manifest.TxnID, manifest.DeviceID, manifest.DeviceSeq)
		default:
			item.manifest = manifest
			queue = append(queue, item)
			return nil
		}
		report.Results = append(report.Results, item.result)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading sync inbox: %w", err)
	}
	sort.Slice(queue, func(i, j int) bool {
		if queue[i].manifest.DeviceSeq != queue[j].manifest.DeviceSeq {
			return queue[i].manifest.DeviceSeq < queue[j].manifest.DeviceSeq
		}
		return queue[i].manifest.DeviceID < queue[j].manifest.DeviceID
	})
	bootstrapped := false
	if bootstrap, ok := committer.(bootstrapCommitter); ok && len(report.Results) == 0 && len(queue) > 1 {
		manifests := make([]*synclog.Manifest, 0, len(queue))
		for _, item := range queue {
			manifests = append(manifests, item.manifest)
		}
		id, accepted, err := bootstrap.BootstrapIncoming(manifests, blobs)
		if err != nil {
			return nil, fmt.Errorf("bootstrapping sync history: %w", err)
		}
		bootstrapped = accepted
		if accepted {
			for _, item := range queue {
				item.result.Outcome, item.result.LocalTxnID = Imported, id
			}
		}
	}
	for progress := !bootstrapped; progress; {
		progress = false
		for _, item := range queue {
			if item.result.Outcome != "" && item.result.Outcome != Incomplete {
				continue
			}
			if applyStaged(committer, blobs, item) {
				progress = true
			}
		}
	}
	for _, item := range queue {
		report.Results = append(report.Results, item.result)
	}
	sort.SliceStable(report.Results, func(i, j int) bool { return report.Results[i].Path < report.Results[j].Path })
	return report, nil
}

// applyStaged tries one manifest and reports whether it was applied.
func applyStaged(committer Committer, blobs inboxBlobs, item *staged) bool {
	item.result.Missing, item.result.Unavailable, item.result.Upgrade = nil, nil, ""
	for _, digest := range RequiredBlobs(item.manifest) {
		if !blobs.has(digest) {
			item.result.Missing = append(item.result.Missing, digest)
		}
	}
	if len(item.result.Missing) > 0 {
		missing := make(map[string]bool, len(item.result.Missing))
		for _, digest := range item.result.Missing {
			missing[digest] = true
		}
		for _, effect := range item.manifest.Effects {
			if effect.Blob != nil && missing[*effect.Blob] {
				target := effect.Path
				if effect.TargetPath != "" {
					target = effect.TargetPath
				}
				item.result.Unavailable = append(item.result.Unavailable, target)
			}
		}
		item.result.Outcome = Incomplete
		item.result.Reason = fmt.Sprintf("%d required blob(s) not yet delivered", len(item.result.Missing))
		return false
	}
	localID, err := committer.CommitIncoming(item.manifest, blobs)
	switch {
	case err == nil:
		item.result.Outcome, item.result.Reason, item.result.LocalTxnID = Imported, "", localID
		return true
	case errors.Is(err, synclog.ErrAlreadyApplied), errors.Is(err, synclog.ErrOwnTransaction):
		item.result.Outcome, item.result.Reason = Known, err.Error()
	case errors.Is(err, synclog.ErrIncomplete):
		item.result.Outcome, item.result.Reason = Incomplete, err.Error()
	case errors.Is(err, synclog.ErrHeld):
		item.result.Outcome, item.result.Reason = Held, err.Error()
		item.result.Upgrade = synclog.RequiredUpgrade(err, item.manifest.DeviceID)
	case errors.Is(err, synclog.ErrForeignWorkspace):
		item.result.Outcome, item.result.Reason = Foreign, err.Error()
	case errors.Is(err, synclog.ErrInvalidManifest):
		item.result.Outcome, item.result.Reason = Invalid, err.Error()
	default:
		item.result.Outcome, item.result.Reason = Failed, err.Error()
	}
	return false
}
