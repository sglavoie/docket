package synclog

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
)

// PlanIncoming plans applying another device's transaction to this workspace
// together with its receipt. The returned operations write, move, and delete
// the workspace files the manifest names, retain its blobs, record the receipt
// and index entry, and advance the tracking state last, so the journal makes
// the applied bytes and the acknowledgement durable together.
//
// Only a fast-forward is planned: every effect's base must be this device's
// current bytes. A manifest is refused, with nothing planned and nothing
// acknowledged, when it is malformed (ErrInvalidManifest), addressed to another
// workspace (ErrForeignWorkspace), authored here (ErrOwnTransaction), already
// received (ErrAlreadyApplied), unreadable by this device (ErrHeld), missing an
// earlier sequence number, a parent, or a blob (ErrIncomplete), or concurrent
// with local changes to the same paths (ErrNeedsMerge).
func (h *History) PlanIncoming(m *Manifest, blobs BlobSource, localTxnID string) (*Plan, error) {
	state, err := h.admit(m)
	if err != nil {
		return nil, err
	}
	o := newOverlay(h)
	if err = h.baseline(state, o); err != nil {
		return nil, err
	}
	effectOps, moveTargets, err := h.applyEffects(o, m, blobs)
	if err != nil {
		return nil, err
	}

	receiptPath := h.receiptPath(m.DeviceID, m.DeviceSeq, m.TxnID)
	receipt, err := encodeJSON(Receipt{
		Format:     ReceiptFormat,
		TxnID:      m.TxnID,
		LocalTxnID: localTxnID,
		ReceivedAt: h.now().UTC().Format(time.RFC3339Nano),
		Manifest:   *m,
	})
	if err != nil {
		return nil, err
	}
	receiptOp, err := immutableOp(receiptPath, receipt)
	if err != nil {
		return nil, err
	}

	state.Received[m.DeviceID] = m.DeviceSeq
	state.Heads = mergeHeads(state.Heads, m.Parents, m.TxnID)
	track(state, m.Effects)

	ops := append([]transaction.Op{}, effectOps...)
	ops = append(ops, o.blobOps...)
	ops = append(ops, receiptOp)
	ops, err = h.appendRecords(ops, state, &IndexEntry{
		Format:    IndexFormat,
		TxnID:     m.TxnID,
		DeviceID:  m.DeviceID,
		DeviceSeq: m.DeviceSeq,
		Origin:    "incoming",
		Record:    h.rel(receiptPath),
	})
	if err != nil {
		return nil, err
	}
	return &Plan{Ops: ops, MoveTargets: moveTargets}, nil
}

// admit decides whether m may be applied now and returns the tracking state it
// would advance. It reads and writes nothing else.
func (h *History) admit(m *Manifest) (*State, error) {
	if m == nil {
		return nil, fmt.Errorf("%w: no manifest", ErrInvalidManifest)
	}
	if m.Format == ManifestFormat && m.FormatVersion > ManifestFormatVersion {
		return nil, &HeldError{FormatVersion: m.FormatVersion}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if m.DeviceID == h.deviceID {
		return nil, fmt.Errorf("%w: %s", ErrOwnTransaction, m.TxnID)
	}
	state, err := h.loadOrStartState(m)
	if err != nil {
		return nil, err
	}
	if m.WorkspaceID != state.WorkspaceID {
		return nil, fmt.Errorf("%w: manifest %s, local %s", ErrForeignWorkspace, m.WorkspaceID, state.WorkspaceID)
	}
	if _, known, lookupErr := h.Lookup(m.TxnID); lookupErr != nil {
		return nil, lookupErr
	} else if known {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyApplied, m.TxnID)
	}
	received := state.Received[m.DeviceID]
	if m.DeviceSeq <= received {
		return nil, fmt.Errorf("%w: device %s already delivered sequence %d as another transaction", ErrInvalidManifest, m.DeviceID, m.DeviceSeq)
	}
	if m.DeviceSeq != received+1 {
		return nil, fmt.Errorf("%w: device %s sequence %d arrived before %d", ErrIncomplete, m.DeviceID, m.DeviceSeq, received+1)
	}
	if err := h.readable(m); err != nil {
		return nil, err
	}
	var missingParents []string
	for _, parent := range m.Parents {
		if _, known, lookupErr := h.Lookup(parent); lookupErr != nil {
			return nil, lookupErr
		} else if !known {
			missingParents = append(missingParents, parent)
		}
	}
	if len(missingParents) > 0 {
		return nil, fmt.Errorf("%w: parents %s have not arrived", ErrIncomplete, strings.Join(missingParents, ", "))
	}
	return state, nil
}

// loadOrStartState reads the tracking state, or starts one for the workspace
// the catalog names when this device has recorded nothing yet. A device
// joining an enrolled workspace holds no catalog at all: it starts from the
// workspace's genesis, which alone may begin a history in an empty directory,
// and with an index that is complete because there is nothing to index.
func (h *History) loadOrStartState(m *Manifest) (*State, error) {
	state, err := h.LoadState()
	if err != nil || state != nil {
		return state, err
	}
	catalog, err := h.readCatalog()
	if err != nil {
		return nil, err
	}
	if catalog == nil && m.Kind == KindEnroll && m.DeviceSeq == 1 && len(m.Parents) == 0 {
		joining := h.newState(m.WorkspaceID)
		joining.Indexed = true
		return joining, nil
	}
	if catalog == nil || catalog.workspaceID == "" {
		return nil, fmt.Errorf("synclog: no workspace catalog names this workspace's identity")
	}
	return h.newState(catalog.workspaceID), nil
}

// applyEffects plans m's effects as journal operations over the overlay,
// refusing any whose base is not this device's current bytes.
func (h *History) applyEffects(o *overlay, m *Manifest, blobs BlobSource) ([]transaction.Op, []string, error) {
	var ops []transaction.Op
	var moveTargets []string
	for i, effect := range m.Effects {
		path := h.abs(effect.Path)
		current, isDir, err := o.lookup(path)
		if err != nil {
			return nil, nil, err
		}
		if isDir || !baseMatches(current, effect.BaseBlob) {
			return nil, nil, fmt.Errorf("%w: effect %d (%s %s)", ErrNeedsMerge, i, effect.Op, effect.Path)
		}
		if current.exists {
			o.retain(current)
		}
		switch effect.Op {
		case OpPut:
			data, fetchErr := fetchBlob(blobs, *effect.Blob)
			if fetchErr != nil {
				return nil, nil, fetchErr
			}
			o.set(path, &vfile{exists: true, digest: *effect.Blob, data: data})
			o.retainBytes(*effect.Blob, data)
			ops = append(ops, writeOp(path, data))
		case OpDelete:
			o.set(path, &vfile{})
			ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
		case OpMove:
			target := h.abs(effect.TargetPath)
			occupant, targetDir, targetErr := o.lookup(target)
			if targetErr != nil {
				return nil, nil, targetErr
			}
			if targetDir || occupant.exists {
				return nil, nil, fmt.Errorf("%w: effect %d moves onto occupied %s", ErrNeedsMerge, i, effect.TargetPath)
			}
			o.set(path, &vfile{})
			o.set(target, current)
			ops = append(ops, transaction.Op{Kind: transaction.OpMove, Source: path, Target: target})
			moveTargets = append(moveTargets, filepath.Dir(target))
		}
	}
	return ops, moveTargets, nil
}

// fetchBlob reads a blob an incoming manifest names, verified by its digest.
func fetchBlob(blobs BlobSource, digest string) ([]byte, error) {
	if blobs == nil {
		return nil, fmt.Errorf("%w: blob %s is not available", ErrIncomplete, digest)
	}
	data, err := blobs.Blob(digest)
	if err != nil {
		return nil, fmt.Errorf("%w: blob %s: %w", ErrIncomplete, digest, err)
	}
	if Digest(data) != digest {
		return nil, fmt.Errorf("%w: blob %s does not match its digest", ErrIncomplete, digest)
	}
	return data, nil
}

// readable holds a manifest this device cannot read: an unsupported format,
// any schema newer than this device's, an older schema that governs one of
// its effects, or a capability this device lacks. An older schema that governs
// none of its effects does not hold it: a device that upgraded still applies
// what a peer wrote before upgrading wherever the bytes are the same format,
// so an upgrade on one device does not strand the other's earlier work.
func (h *History) readable(m *Manifest) error {
	if m.FormatVersion != ManifestFormatVersion {
		return &HeldError{FormatVersion: m.FormatVersion}
	}
	keys := map[string]bool{}
	for key := range m.Schemas {
		keys[key] = true
	}
	for key := range h.schemas {
		keys[key] = true
	}
	held := &HeldError{}
	newer := false
	for key := range keys {
		if m.Schemas[key] != h.schemas[key] {
			held.Schemas = append(held.Schemas, SchemaDifference{Key: key, Manifest: m.Schemas[key], Device: h.schemas[key]})
			newer = newer || m.Schemas[key] > h.schemas[key]
		}
	}
	sort.Slice(held.Schemas, func(i, j int) bool { return held.Schemas[i].Key < held.Schemas[j].Key })
	if newer {
		return held
	}
	if len(held.Schemas) > 0 {
		governed := map[string]bool{}
		for _, effect := range m.Effects {
			for _, key := range governingSchemas(effect.Kind) {
				governed[key] = true
			}
		}
		older := held.Schemas
		held.Schemas = nil
		for _, diff := range older {
			if governed[diff.Key] {
				held.Schemas = append(held.Schemas, diff)
			}
		}
		if len(held.Schemas) > 0 {
			return held
		}
	}
	for _, requirement := range m.Requires {
		if !h.capabilities[requirement] {
			held.Missing = append(held.Missing, requirement)
		}
	}
	for _, effect := range m.Effects {
		if requirement := h.requirement(effect.Kind); !h.capabilities[requirement] {
			held.Missing = append(held.Missing, requirement)
		}
	}
	if len(held.Missing) > 0 {
		sort.Strings(held.Missing)
		return held
	}
	return nil
}

// governingSchemas are the schema keys an effect's bytes are written at.
// Notes, attachments, and other project files are opaque and governed by
// none.
func governingSchemas(kind EffectKind) []string {
	switch kind {
	case EffectProjectDocument:
		return []string{SchemaTaskMarkdown}
	case EffectCatalog:
		return []string{SchemaCatalog, SchemaDomainEnvelope}
	case EffectPlanning, EffectLenses:
		return []string{SchemaDomainEnvelope}
	}
	return nil
}

func baseMatches(current *vfile, base *string) bool {
	if base == nil {
		return !current.exists
	}
	return current.exists && current.digest == *base
}

// PlanPublished plans confirming that the outgoing transactions txnIDs reached
// the exchange: it drops them from the outbox. Confirming an id no longer
// pending is a no-op, so a repeated confirmation plans nothing.
func (h *History) PlanPublished(txnIDs []string) (*Plan, error) {
	state, err := h.LoadState()
	if err != nil || state == nil {
		return &Plan{}, err
	}
	confirmed := make(map[string]bool, len(txnIDs))
	for _, id := range txnIDs {
		confirmed[id] = true
	}
	kept := make([]OutboxEntry, 0, len(state.Outbox))
	for _, entry := range state.Outbox {
		if !confirmed[entry.TxnID] {
			kept = append(kept, entry)
		}
	}
	if len(kept) == len(state.Outbox) {
		return &Plan{}, nil
	}
	state.Outbox = kept
	ops, err := h.appendRecords(nil, state, nil)
	if err != nil {
		return nil, err
	}
	return &Plan{Ops: ops}, nil
}
