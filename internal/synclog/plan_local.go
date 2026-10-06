package synclog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
)

// pendingEffect is an effect still waiting for its project identity.
type pendingEffect struct {
	effect Effect
	slug   string
}

// PlanLocal plans the outgoing record of a local transaction: it reads ops —
// the journal operations about to commit, in order — against the workspace as
// it stands under the caller's locks, and returns the history operations to
// stage in the same journal manifest. Each changed synced file becomes exactly
// one effect naming the bytes it replaced and the bytes it wrote; device-local
// paths are not effects, and a rewrite of identical bytes changes nothing. A
// transaction with no effect records nothing and plans no operations.
//
// localTxnID is the journal transaction id the record cites. A path the
// history cannot classify, or a project directory no catalog names, refuses the
// whole commit: publishing part of a transaction would be worse than not
// committing it.
func (h *History) PlanLocal(kind Kind, localTxnID string, ops []transaction.Op) (*Plan, error) {
	if !kind.valid() {
		return nil, fmt.Errorf("synclog: unknown transaction kind %q", kind)
	}
	diskCatalog, err := h.readCatalog()
	if err != nil {
		return nil, err
	}
	o := newOverlay(h)
	// A resolution may choose the bytes the workspace already holds. It still
	// has to be an effect: writing the key is what resolves its conflict.
	o.keepIdentical = kind == KindResolution
	var pending []pendingEffect
	for i, op := range ops {
		effects, effectErr := h.effectsFor(o, op)
		if effectErr != nil {
			return nil, fmt.Errorf("synclog: op %d (%s %s): %w", i, op.Kind, op.Target, effectErr)
		}
		pending = append(pending, effects...)
	}
	if len(pending) == 0 {
		return &Plan{}, nil
	}

	var newCatalog *catalogIdentity
	if f, ok := o.files[h.abs(catalogRel)]; ok && f.exists {
		if newCatalog, err = parseCatalog(f.data); err != nil {
			return nil, err
		}
	}
	state, err := h.LoadState()
	if err != nil {
		return nil, err
	}
	workspaceID := ""
	for _, catalog := range []*catalogIdentity{newCatalog, diskCatalog} {
		if catalog != nil && catalog.workspaceID != "" {
			workspaceID = catalog.workspaceID
			break
		}
	}
	if state == nil {
		if workspaceID == "" {
			return nil, fmt.Errorf("synclog: no workspace catalog names this workspace's identity")
		}
		state = h.newState(workspaceID)
	} else if workspaceID != "" && workspaceID != state.WorkspaceID {
		return nil, fmt.Errorf("%w: catalog %s, sync history %s", ErrForeignWorkspace, workspaceID, state.WorkspaceID)
	}

	effects := make([]Effect, 0, len(pending))
	requires := map[string]bool{}
	for _, p := range pending {
		effect := p.effect
		if effect.Kind.projectScoped() {
			id := ""
			for _, catalog := range []*catalogIdentity{newCatalog, diskCatalog} {
				if catalog != nil && catalog.slugs[p.slug] != "" {
					id = catalog.slugs[p.slug]
					break
				}
			}
			if id == "" {
				return nil, fmt.Errorf("%w: project directory %q is in no catalog", ErrUnclassifiedPath, p.slug)
			}
			effect.ProjectID = id
		}
		requires[h.requirement(effect.Kind)] = true
		effects = append(effects, effect)
	}

	manifest := &Manifest{
		Format:        ManifestFormat,
		FormatVersion: ManifestFormatVersion,
		WorkspaceID:   state.WorkspaceID,
		DeviceID:      h.deviceID,
		DeviceSeq:     state.NextSeq,
		Parents:       append([]string{}, state.Heads...),
		Kind:          kind,
		LocalTxnID:    localTxnID,
		CreatedAt:     h.now().UTC().Format(time.RFC3339Nano),
		Schemas:       h.schemas,
		Requires:      sortedKeys(requires),
		Effects:       effects,
	}
	sort.Strings(manifest.Parents)
	if err = manifest.seal(); err != nil {
		return nil, err
	}
	if err = manifest.Validate(); err != nil {
		return nil, fmt.Errorf("synclog: planned an invalid record: %w", err)
	}
	record, err := manifest.Encode()
	if err != nil {
		return nil, err
	}
	recordPath := h.outgoingRecordPath(manifest.DeviceSeq, manifest.TxnID)
	recordOp, err := immutableOp(recordPath, record)
	if err != nil {
		return nil, err
	}

	state.NextSeq++
	state.Heads = []string{manifest.TxnID}
	state.Outbox = append(state.Outbox, OutboxEntry{TxnID: manifest.TxnID, DeviceSeq: manifest.DeviceSeq, Record: h.rel(recordPath)})
	track(state, manifest.Effects)

	planOps := append([]transaction.Op{}, o.blobOps...)
	planOps = append(planOps, recordOp)
	planOps, err = h.appendRecords(planOps, state, &IndexEntry{
		Format:    IndexFormat,
		TxnID:     manifest.TxnID,
		DeviceID:  h.deviceID,
		DeviceSeq: manifest.DeviceSeq,
		Origin:    "local",
		Record:    h.rel(recordPath),
	})
	if err != nil {
		return nil, err
	}
	return &Plan{Ops: planOps}, nil
}

// effectsFor folds one journal operation into the overlay and returns the
// effects it publishes.
func (h *History) effectsFor(o *overlay, op transaction.Op) ([]pendingEffect, error) {
	target := filepath.Clean(op.Target)
	switch op.Kind {
	case transaction.OpWrite:
		before, isDir, err := o.lookup(target)
		if err != nil {
			return nil, err
		}
		if isDir {
			return nil, fmt.Errorf("write replaces a directory")
		}
		after := &vfile{exists: true, digest: Digest(op.Data), data: op.Data}
		o.set(target, after)
		rel, kind, slug, synced, err := h.classifyAbs(target)
		if err != nil || !synced || (before.exists && before.digest == after.digest && !o.keepIdentical) {
			return nil, err
		}
		effect := Effect{Kind: kind, Path: rel, Op: OpPut, Blob: digestPtr(after.digest)}
		if before.exists {
			effect.BaseBlob = digestPtr(before.digest)
			o.retain(before)
		}
		o.retain(after)
		return []pendingEffect{{effect: effect, slug: slug}}, nil
	case transaction.OpDelete:
		before, isDir, err := o.lookup(target)
		if err != nil {
			return nil, err
		}
		if isDir || !before.exists {
			// A directory is removed only once its files were, each its own effect.
			return nil, nil
		}
		o.set(target, &vfile{})
		rel, kind, slug, synced, err := h.classifyAbs(target)
		if err != nil || !synced {
			return nil, err
		}
		o.retain(before)
		return []pendingEffect{{effect: Effect{Kind: kind, Path: rel, Op: OpDelete, BaseBlob: digestPtr(before.digest)}, slug: slug}}, nil
	case transaction.OpMove:
		source := filepath.Clean(op.Source)
		files, err := o.filesUnder(source)
		if err != nil {
			return nil, err
		}
		var out []pendingEffect
		for _, from := range files {
			to := target
			if from != source {
				suffix, relErr := filepath.Rel(source, from)
				if relErr != nil {
					return nil, relErr
				}
				to = filepath.Join(target, suffix)
			}
			moved, err := h.moveEffect(o, from, to)
			if err != nil {
				return nil, err
			}
			out = append(out, moved...)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown operation kind %q", op.Kind)
	}
}

// moveEffect publishes one file's move. A move between a synced and a
// device-local path is the put or delete it amounts to on the synced side.
func (h *History) moveEffect(o *overlay, from, to string) ([]pendingEffect, error) {
	file, _, err := o.lookup(from)
	if err != nil {
		return nil, err
	}
	replaced, isDir, err := o.lookup(to)
	if err != nil {
		return nil, err
	}
	if isDir {
		return nil, fmt.Errorf("move onto directory %s", to)
	}
	o.set(from, &vfile{})
	o.set(to, file)
	fromRel, fromKind, fromSlug, fromSynced, err := h.classifyAbs(from)
	if err != nil {
		return nil, err
	}
	toRel, toKind, toSlug, toSynced, err := h.classifyAbs(to)
	if err != nil {
		return nil, err
	}
	switch {
	case fromSynced && toSynced && fromKind == toKind && !replaced.exists:
		o.retain(file)
		slug := toSlug
		return []pendingEffect{{effect: Effect{
			Kind: toKind, Path: fromRel, Op: OpMove, TargetPath: toRel,
			BaseBlob: digestPtr(file.digest), Blob: digestPtr(file.digest),
		}, slug: slug}}, nil
	default:
		var out []pendingEffect
		if fromSynced {
			o.retain(file)
			out = append(out, pendingEffect{effect: Effect{Kind: fromKind, Path: fromRel, Op: OpDelete, BaseBlob: digestPtr(file.digest)}, slug: fromSlug})
		}
		if toSynced && (!replaced.exists || replaced.digest != file.digest) {
			effect := Effect{Kind: toKind, Path: toRel, Op: OpPut, Blob: digestPtr(file.digest)}
			if replaced.exists {
				effect.BaseBlob = digestPtr(replaced.digest)
				o.retain(replaced)
			}
			o.retain(file)
			out = append(out, pendingEffect{effect: effect, slug: toSlug})
		}
		return out, nil
	}
}

// classifyAbs classifies an absolute path. A path outside the workspace — the
// machine-local runtime root — is never synced.
func (h *History) classifyAbs(abs string) (rel string, kind EffectKind, slug string, synced bool, err error) {
	if !hasPathPrefix(abs, h.dataDir) {
		return "", "", "", false, nil
	}
	r, err := filepath.Rel(h.dataDir, abs)
	if err != nil {
		return "", "", "", false, err
	}
	rel = filepath.ToSlash(r)
	kind, slug, local, err := Classify(rel)
	if err != nil {
		return "", "", "", false, err
	}
	return rel, kind, slug, !local, nil
}

// readCatalog parses the on-disk catalog, or returns nil when there is none.
func (h *History) readCatalog() (*catalogIdentity, error) {
	data, err := os.ReadFile(h.abs(catalogRel))
	if isNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading workspace catalog: %w", err)
	}
	return parseCatalog(data)
}

// track advances the last indexed digest of every path effects changed.
func track(state *State, effects []Effect) {
	for _, effect := range effects {
		switch effect.Op {
		case OpPut:
			state.Tracked[effect.Path] = *effect.Blob
		case OpDelete:
			delete(state.Tracked, effect.Path)
		case OpMove:
			delete(state.Tracked, effect.Path)
			state.Tracked[effect.TargetPath] = *effect.Blob
		}
	}
}

func digestPtr(digest string) *string { return &digest }

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
