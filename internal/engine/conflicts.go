package engine

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/syncmerge"
)

// Sync conflict refusals. Callers classify with errors.Is.
var (
	// ErrConflictStale marks a resolution of a conflict that changed since it
	// was read: another device's resolution closed it, or an edit gave it new
	// bytes and a new id. Nothing was written; the racing edit and every
	// competing version stay as they are, so re-read and choose again.
	ErrConflictStale = errors.New("engine: the sync conflict changed since it was read")
	// ErrInvalidResolution marks a resolution the engine cannot write: an
	// unknown choice or version, replacement text for a kind that takes none,
	// or text the workspace's readers would refuse.
	ErrInvalidResolution = errors.New("engine: invalid sync conflict resolution")
)

// Resolution choices.
const (
	// ResolveVersion keeps one competing version, and with it the same
	// transactions' versions of every other conflict they left, in one
	// resolution transaction.
	ResolveVersion = "version"
	// ResolveCurrent keeps what the workspace holds now.
	ResolveCurrent = "current"
	// ResolveReplace writes replacement text a person composed.
	ResolveReplace = "replace"
)

// ConflictView is one open sync conflict as a person reads it: what the file
// is, the version both sides started from, each competing version with the
// devices and changes that wrote it, what the workspace holds meanwhile, and
// what else a choice would settle.
type ConflictView struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	ProjectID string `json:"project_id,omitempty"`
	// Label names the file the way the app does: the project's tasks, a
	// note's name, the planning lists.
	Label string `json:"label"`
	// Base is the version every competing change started from, or nil when
	// the file did not exist then.
	Base *ConflictContent `json:"base"`
	// Current is what the workspace holds now.
	Current  *ConflictContent  `json:"current"`
	Versions []ConflictVersion `json:"versions"`
	// Details are the parts the merge could not reconcile on its own.
	Details []synclog.ConflictDetail `json:"details"`
	// Replaceable reports that replacement text may be written for this kind.
	Replaceable bool `json:"replaceable"`
}

// ConflictContent is one version's bytes as far as a reader can show them.
type ConflictContent struct {
	Digest string `json:"digest,omitempty"`
	// Deleted marks a version that removed the file.
	Deleted bool `json:"deleted"`
	Size    int  `json:"size"`
	// Text is the content when it is text of a readable size.
	Text *string `json:"text,omitempty"`
	// Available is false when the bytes are not retained on this device.
	Available bool `json:"available"`
}

// ConflictVersion is one competing version and who left it.
type ConflictVersion struct {
	Index int `json:"index"`
	ConflictContent
	// Current marks the version the workspace holds meanwhile.
	Current bool             `json:"current"`
	Writers []ConflictWriter `json:"writers"`
	// AlsoResolves are the other open conflicts the same transactions left;
	// choosing this version keeps their versions too, in one transaction.
	AlsoResolves []ConflictRef `json:"also_resolves"`
}

// ConflictWriter is one transaction that wrote a competing version.
type ConflictWriter struct {
	TxnID    string `json:"txn_id"`
	DeviceID string `json:"device_id"`
	// DeviceName is the name the device advertises, when this device knows it.
	DeviceName string `json:"device_name,omitempty"`
	// ThisDevice marks a change made here.
	ThisDevice bool   `json:"this_device"`
	Change     string `json:"change"`
	CreatedAt  string `json:"created_at"`
	// Files counts every file the transaction changed.
	Files int `json:"files"`
}

// ConflictRef names an open conflict.
type ConflictRef struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Label string `json:"label"`
}

// ResolveRequest chooses how one open conflict, read with ConflictID, ends.
type ResolveRequest struct {
	ConflictID string `json:"conflict_id"`
	Choice     string `json:"choice"`
	// Version is the index of the competing version ResolveVersion keeps.
	Version int `json:"version"`
	// Text is the replacement ResolveReplace writes.
	Text string `json:"text"`
}

// ResolveResult is a committed resolution.
type ResolveResult struct {
	TxnID string `json:"txn_id"`
	// Resolved are the conflicts the resolution closed: the one chosen and
	// every one coupled to it.
	Resolved []ConflictRef `json:"resolved"`
	// Remaining counts the conflicts still open.
	Remaining int `json:"remaining"`
}

// SyncConflicts reads every open sync conflict with its versions and their
// context. names maps device ids to the names their advertisements give;
// on a desktop the last exchange pass supplies them when names is nil.
func (r *Runtime) SyncConflicts(names map[string]string) ([]ConflictView, error) {
	commits := r.repo.svc
	history := commits.SyncHistory()
	if history == nil {
		return []ConflictView{}, nil
	}
	conflicts, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = r.peerNames()
	}
	reader := conflictReader{runtime: r, history: history, names: names, writers: map[string]ConflictWriter{}, all: conflicts}
	views := make([]ConflictView, 0, len(conflicts))
	for _, conflict := range conflicts {
		view, err := reader.view(conflict)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].Path != views[j].Path {
			return views[i].Path < views[j].Path
		}
		return views[i].ID < views[j].ID
	})
	return views, nil
}

// ResolveSyncConflict commits a resolution. Keeping a competing version also
// keeps that version's transactions' side of every coupled conflict, so a
// change that spanned files is resolved whole. The conflict must still be
// open with the bytes it was read with; otherwise nothing is written and the
// refusal wraps ErrConflictStale.
func (r *Runtime) ResolveSyncConflict(req ResolveRequest) (*ResolveResult, error) {
	commits := r.repo.svc
	history := commits.SyncHistory()
	if history == nil {
		return nil, fmt.Errorf("%w: this workspace records no sync history", ErrInvalidResolution)
	}
	conflicts, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(conflicts, func(c synclog.Conflict) bool { return c.ID == req.ConflictID })
	if index < 0 {
		return nil, fmt.Errorf("%w: conflict %s is no longer open", ErrConflictStale, shortDigest(req.ConflictID))
	}
	chosen := conflicts[index]
	reader := conflictReader{runtime: r, history: history, all: conflicts}
	var resolutions []storage.SyncResolution
	resolved := []ConflictRef{reader.ref(chosen)}
	switch req.Choice {
	case ResolveCurrent:
		data, readErr := reader.bytes(chosen.ResultBlob, chosen.Path)
		if readErr != nil {
			return nil, readErr
		}
		resolutions = append(resolutions, storage.SyncResolution{ConflictID: chosen.ID, Data: data})
	case ResolveReplace:
		if !replaceable(chosen.Kind) {
			return nil, fmt.Errorf("%w: a %s takes a version, not replacement text", ErrInvalidResolution, chosen.Kind)
		}
		data := []byte(req.Text)
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%w: replacement text is not UTF-8", ErrInvalidResolution)
		}
		if invalid := (syncmerge.Merger{}).ValidateArtifact(chosen.Kind, data); invalid != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidResolution, invalid)
		}
		resolutions = append(resolutions, storage.SyncResolution{ConflictID: chosen.ID, Data: data})
	case ResolveVersion:
		if req.Version < 0 || req.Version >= len(chosen.Candidates) {
			return nil, fmt.Errorf("%w: conflict %s has no version %d", ErrInvalidResolution, shortDigest(chosen.ID), req.Version)
		}
		candidate := chosen.Candidates[req.Version]
		data, keepErr := reader.kept(chosen, candidate)
		if keepErr != nil {
			return nil, keepErr
		}
		resolutions = append(resolutions, storage.SyncResolution{ConflictID: chosen.ID, Data: data})
		for _, coupled := range reader.coupled(chosen, candidate) {
			coupledData, coupledErr := reader.kept(coupled.conflict, coupled.candidate)
			if coupledErr != nil {
				return nil, coupledErr
			}
			resolutions = append(resolutions, storage.SyncResolution{ConflictID: coupled.conflict.ID, Data: coupledData})
			resolved = append(resolved, reader.ref(coupled.conflict))
		}
	default:
		return nil, fmt.Errorf("%w: choice %q is not version, current, or replace", ErrInvalidResolution, req.Choice)
	}
	txnID, err := commits.CommitSyncResolutions(resolutions)
	switch {
	case errors.Is(err, storage.ErrSyncConflictClosed), errors.Is(err, storage.ErrStaleArtifact):
		return nil, fmt.Errorf("%w: %w", ErrConflictStale, err)
	case err != nil:
		return nil, err
	}
	if err = r.Refresh(); err != nil {
		return nil, err
	}
	remaining, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	return &ResolveResult{TxnID: txnID, Resolved: resolved, Remaining: len(remaining)}, nil
}

// replaceable reports whether a person may write replacement text for a
// kind: prose they can read and edit. Structured domain files and binary
// content take a version instead.
func replaceable(kind synclog.EffectKind) bool {
	switch kind {
	case synclog.EffectProjectDocument, synclog.EffectNote, synclog.EffectGlobalNote:
		return true
	default:
		return false
	}
}
