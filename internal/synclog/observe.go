package synclog

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/transaction"
)

// External changes are the workspace edits no commit recorded: an editor
// rewriting tasks.md, a script dropping a note, or a checked domain save that
// writes the catalog, planning, or lenses outside the journal. Observation
// compares every synced file with the digest the history last indexed for it
// (State.Tracked) and imports each valid difference as a change of its own, an
// outgoing transaction of kind external, so it reaches every other device the
// way a commit would. A difference that is not a valid artifact is held: it
// stays on disk exactly as written, it is not published, and because it is not
// indexed either, reconciliation refuses to write over it
// (ErrUnrecordedChange) until it is repaired and observed again.

// Validator judges bytes changed outside a commit. It must accept exactly what
// the live readers accept, so a held change is one a device could not load.
type Validator interface {
	// ValidateArtifact returns nil when data is a valid artifact of kind.
	ValidateArtifact(kind EffectKind, data []byte) error
}

// structured reports whether an artifact of this kind has a format a reader
// can refuse. Notes, attachments, and other project files are opaque bytes.
func (k EffectKind) structured() bool {
	switch k {
	case EffectProjectDocument, EffectCatalog, EffectPlanning, EffectLenses:
		return true
	case EffectNote, EffectGlobalNote, EffectAttachment, EffectProjectTree:
		return false
	}
	return false
}

// HeldChange is an external change observation refused to publish, and why.
// Its bytes are left untouched on disk for repair.
type HeldChange struct {
	Path   string     `json:"path"`
	Kind   EffectKind `json:"kind"`
	Reason string     `json:"reason"`
}

// ObservePlan is an observation ready to stage: the outgoing record of every
// valid external change, or the device's first index of its workspace.
type ObservePlan struct {
	Plan
	// Baselined reports that this device had not indexed the workspace yet:
	// the plan records the files as they stand and imports nothing.
	Baselined bool
	// Manifest is the external transaction the plan records, or nil.
	Manifest *Manifest
	// Held lists the changes left on disk unpublished, by path.
	Held []HeldChange
}

// baseline indexes every synced file not yet tracked, as it stands on disk,
// and retains its bytes so a later change to it still names a merge base this
// device can supply. It runs once per device, the first time the history
// records anything: devices start from one shared copy of the workspace, and
// from then on every difference from the index is a change.
func (h *History) baseline(state *State, o *overlay) error {
	if state.Indexed {
		return nil
	}
	err := h.walkSynced(func(rel string, data []byte) error {
		if _, tracked := state.Tracked[rel]; tracked {
			return nil
		}
		digest := Digest(data)
		state.Tracked[rel] = digest
		if !o.planned[digest] {
			o.retainBytes(digest, data)
		}
		return nil
	})
	if err != nil {
		return err
	}
	state.Indexed = true
	return nil
}

// walkSynced calls fn with every regular synced file in the workspace and its
// bytes.
func (h *History) walkSynced(fn func(rel string, data []byte) error) error {
	return h.walkSyncedFiles(func(rel, abs string, _ fs.FileInfo) error {
		data, err := h.readSynced(rel, abs)
		if err != nil {
			return err
		}
		return fn(rel, data)
	})
}

func (h *History) readSynced(rel, abs string) ([]byte, error) {
	data, err := readSyncedFile(abs)
	if err != nil {
		return nil, fmt.Errorf("reading %s for sync observation: %w", rel, err)
	}
	return data, nil
}

// walkSyncedFiles calls fn with every regular synced file in the workspace
// and its stat identity, reading none. It does not descend into device-local
// directories, and it passes over files the history cannot classify, which
// are not part of the synced workspace.
func (h *History) walkSyncedFiles(fn func(rel, abs string, info fs.FileInfo) error) error {
	err := filepath.WalkDir(h.dataDir, func(abs string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if abs == h.dataDir {
			return nil
		}
		rel := h.rel(abs)
		if d.IsDir() {
			parts := strings.Split(rel, "/")
			if len(parts) == 2 && parts[0] == ".tt" && localInternalDirs[parts[1]] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if !synced(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("reading %s for sync observation: %w", rel, err)
		}
		return fn(rel, abs, info)
	})
	if err != nil && !isNotExist(err) {
		return err
	}
	return nil
}

// synced reports whether rel is part of the synced workspace: classified, and
// not device-local.
func synced(rel string) bool {
	_, _, local, err := Classify(rel)
	return err == nil && !local
}

// PlanObserve plans importing every external change: each synced file whose
// bytes differ from the last indexed revision becomes one effect of a single
// outgoing transaction of kind external, named by localTxnID, unless it is
// invalid, which holds it. A device that has never indexed its workspace
// indexes it instead when index is set, and imports nothing either way: with
// no index there is nothing to compare against. A workspace with no catalog,
// or with nothing changed, plans no operations.
func (h *History) PlanObserve(v Validator, localTxnID string, index bool) (*ObservePlan, error) {
	state, err := h.LoadState()
	if err != nil {
		return nil, err
	}
	if state == nil {
		catalog, catalogErr := h.readCatalog()
		if catalogErr != nil {
			return nil, catalogErr
		}
		if catalog == nil || catalog.workspaceID == "" {
			return &ObservePlan{}, nil
		}
		state = h.newState(catalog.workspaceID)
	}
	o := newOverlay(h)
	if !state.Indexed {
		if !index {
			return &ObservePlan{}, nil
		}
		if err = h.baseline(state, o); err != nil {
			return nil, err
		}
		ops, appendErr := h.appendRecords(append([]transaction.Op{}, o.blobOps...), state, nil)
		if appendErr != nil {
			return nil, appendErr
		}
		return &ObservePlan{Plan: Plan{Ops: ops}, Baselined: true}, nil
	}

	return h.observe(state, o, v, KindExternal, localTxnID)
}

// ErrGenesisRefused is a genesis that cannot be recorded: the device already
// has a history, or a file of the workspace would be held rather than
// published.
var ErrGenesisRefused = errors.New("synclog: the enrollment genesis cannot be recorded")

// PlanGenesis plans the first transaction of a workspace enrolled in sync, on
// the device that enrolled it: one outgoing transaction of kind enroll, at
// sequence 1 and with no parents, that writes every synced file as it stands.
// It is observation against an empty index, so the history records the whole
// workspace as a change and a device that joins with nothing can rebuild it
// from the history alone. It refuses (ErrGenesisRefused) a device that has
// recorded anything already, and a workspace any of whose files the validator
// would hold: a genesis never publishes part of a workspace.
func (h *History) PlanGenesis(v Validator, localTxnID string) (*ObservePlan, error) {
	state, err := h.LoadState()
	if err != nil {
		return nil, err
	}
	if state != nil {
		return nil, fmt.Errorf("%w: device %s has already recorded history", ErrGenesisRefused, h.deviceID)
	}
	catalog, err := h.readCatalog()
	if err != nil {
		return nil, err
	}
	if catalog == nil || catalog.workspaceID == "" {
		return nil, fmt.Errorf("%w: no workspace catalog names this workspace's identity", ErrGenesisRefused)
	}
	state = h.newState(catalog.workspaceID)
	state.Indexed = true
	plan, err := h.observe(state, newOverlay(h), v, KindEnroll, localTxnID)
	if err != nil {
		return nil, err
	}
	if len(plan.Held) > 0 {
		return nil, fmt.Errorf("%w: %s: %s", ErrGenesisRefused, plan.Held[0].Path, plan.Held[0].Reason)
	}
	if plan.Manifest == nil {
		return nil, fmt.Errorf("%w: the workspace holds no synced file", ErrGenesisRefused)
	}
	return plan, nil
}

// observe records every synced file whose bytes differ from state's index as
// one outgoing transaction of txnKind, holding the ones the validator refuses.
// Only files the stat cache cannot vouch for are read.
func (h *History) observe(state *State, o *overlay, v Validator, txnKind Kind, localTxnID string) (*ObservePlan, error) {
	current, err := h.digestSynced()
	if err != nil {
		return nil, err
	}
	var paths []string
	for rel, digest := range current.digests {
		if state.Tracked[rel] == digest {
			continue
		}
		// A reused digest only says where to look: the change is judged by the
		// bytes, so read them and compare again.
		_, exists, readErr := current.bytes(h, rel)
		if readErr != nil {
			return nil, readErr
		}
		if exists && state.Tracked[rel] != current.digests[rel] {
			paths = append(paths, rel)
		}
	}
	for rel := range state.Tracked {
		if _, exists := current.digests[rel]; !exists {
			paths = append(paths, rel)
		}
	}
	if len(paths) == 0 {
		return &ObservePlan{}, nil
	}
	sort.Strings(paths)

	diskCatalog, err := h.readCatalog()
	if err != nil {
		return nil, err
	}
	indexedCatalog := h.indexedCatalog(state)
	var effects []Effect
	var held []HeldChange
	for _, rel := range paths {
		kind, slug, local, classifyErr := Classify(rel)
		if classifyErr != nil || local {
			continue
		}
		data, exists := current.contents[rel]
		effect, reason := h.judge(state, v, rel, kind, slug, data, exists, diskCatalog, indexedCatalog)
		if reason != "" {
			held = append(held, HeldChange{Path: rel, Kind: kind, Reason: reason})
			continue
		}
		if effect.Blob != nil {
			o.retainBytes(*effect.Blob, data)
		}
		effects = append(effects, effect)
	}
	if len(effects) == 0 {
		return &ObservePlan{Held: held}, nil
	}
	plan, manifest, err := h.recordLocal(state, txnKind, localTxnID, effects, o)
	if err != nil {
		return nil, err
	}
	return &ObservePlan{Plan: *plan, Manifest: manifest, Held: held}, nil
}

// judge turns one observed difference into its effect, or into the reason it
// is held. A removed workspace domain, a removed project document its project
// still names, a file in a project directory no catalog names, a catalog
// claiming another workspace, and a structured artifact the validator refuses
// are all held.
func (h *History) judge(
	state *State,
	v Validator,
	rel string,
	kind EffectKind,
	slug string,
	data []byte,
	exists bool,
	diskCatalog, indexedCatalog *catalogIdentity,
) (Effect, string) {
	effect := Effect{Kind: kind, Path: rel}
	if base, tracked := state.Tracked[rel]; tracked {
		effect.BaseBlob = digestPtr(base)
	}
	if !exists {
		effect.Op = OpDelete
		if !kind.projectScoped() {
			if kind == EffectGlobalNote {
				return effect, ""
			}
			return effect, "a workspace domain file cannot be removed"
		}
		if diskCatalog != nil && diskCatalog.slugs[slug] != "" && kind == EffectProjectDocument {
			return effect, fmt.Sprintf("the catalog still names project directory %q", slug)
		}
		for _, catalog := range []*catalogIdentity{diskCatalog, indexedCatalog} {
			if catalog != nil && catalog.slugs[slug] != "" {
				effect.ProjectID = catalog.slugs[slug]
				return effect, ""
			}
		}
		return effect, fmt.Sprintf("project directory %q is in no catalog", slug)
	}

	effect.Op = OpPut
	effect.Blob = digestPtr(Digest(data))
	if kind.projectScoped() {
		if diskCatalog == nil || diskCatalog.slugs[slug] == "" {
			return effect, fmt.Sprintf("project directory %q is in no catalog", slug)
		}
		effect.ProjectID = diskCatalog.slugs[slug]
	}
	if kind == EffectCatalog {
		catalog, err := parseCatalog(data)
		if err != nil {
			return effect, err.Error()
		}
		if catalog.workspaceID != state.WorkspaceID {
			return effect, fmt.Sprintf("the catalog names workspace %q, not %q", catalog.workspaceID, state.WorkspaceID)
		}
	}
	if kind.structured() {
		if v == nil {
			return effect, "no validator is attached to judge this artifact"
		}
		if err := v.ValidateArtifact(kind, data); err != nil {
			return effect, err.Error()
		}
	}
	return effect, ""
}

// indexedCatalog is the catalog as last indexed, which still names the
// projects an external change removed. It is nil when its bytes are not
// retained.
func (h *History) indexedCatalog(state *State) *catalogIdentity {
	digest, ok := state.Tracked[catalogRel]
	if !ok {
		return nil
	}
	data, err := h.Blob(digest)
	if err != nil {
		return nil
	}
	catalog, err := parseCatalog(data)
	if err != nil {
		return nil
	}
	return catalog
}
