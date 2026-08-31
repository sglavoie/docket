package transaction

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Inspect and its result types exist so a diagnostic can describe a workspace's
// transactions directory without touching it. Recover is the only other reader,
// and it is a mutation: it rolls transactions forward, sweeps staged bytes, and
// moves manifests into quarantine. A read-only integrity check cannot use it.

// PendingTransaction is one manifest sitting in the top level of the
// transactions directory, described rather than acted on.
type PendingTransaction struct {
	ID   string
	Path string
	// LoadError is non-empty when the manifest could not be read or validated. The
	// remaining fields are then zero: nothing about the transaction is knowable.
	LoadError string
	State     State
	CreatedAt string
	Machine   string
	Heartbeat string
	// OwnedByThisMachine reports whether the manifest names the inspecting machine.
	// An unowned manifest — one written before transactions recorded ownership —
	// reports false with an empty Machine.
	OwnedByThisMachine bool
	// HeartbeatFresh reports whether the recorded heartbeat still vouches for a
	// live owner, under the same ForeignTransactionGrace recovery applies.
	HeartbeatFresh bool
	// EscapesRoots reports whether any operation names a path outside the approved
	// workspace roots. This is what a manifest prepared under a different absolute
	// path looks like from here — the same workspace synced to a different location
	// on this machine — and it is the condition that sends a transaction to
	// quarantine once its owner's heartbeat goes stale.
	EscapesRoots bool
}

// Live reports whether a pending transaction is one recovery would try to
// finish: staged or mid-apply rather than already committed.
func (p PendingTransaction) Live() bool {
	switch p.State {
	case StatePreparing, StatePrepared, StateCommitting:
		return true
	default:
		return false
	}
}

// ForeignLive reports whether this transaction belongs to another machine that
// still appears to be driving it, matching Journal.foreignLive.
func (p PendingTransaction) ForeignLive() bool {
	return p.LoadError == "" && p.Machine != "" && !p.OwnedByThisMachine && p.Live() && p.HeartbeatFresh
}

// QuarantinedTransaction is one manifest recovery moved into the quarantine
// directory. Quarantine blocks every workspace mutation until it is resolved by
// hand, so a diagnostic must be able to see it without clearing it.
type QuarantinedTransaction struct {
	ID   string
	Path string
	// LoadError is non-empty when the quarantined manifest is itself unreadable,
	// which is one of the reasons it was quarantined in the first place.
	LoadError string
	State     State
	Machine   string
}

// Inventory is a read-only census of a transactions directory.
type Inventory struct {
	// Dir is the transactions directory inspected, present or not.
	Dir         string
	Pending     []PendingTransaction
	Quarantined []QuarantinedTransaction
}

// Blocked reports whether the inventory shows a quarantine, which is the state
// that refuses every workspace mutation.
func (inv *Inventory) Blocked() bool { return inv != nil && len(inv.Quarantined) > 0 }

// QuarantineDir returns the quarantine directory's path, which is what a
// diagnostic names when it tells a human where to look and what to move aside.
func (inv *Inventory) QuarantineDir() string {
	return QuarantineDirFor(inv.Dir)
}

// QuarantineDirFor names the quarantine directory beneath a transactions
// directory. A quarantine blocks every workspace mutation, and the workspace
// cannot be opened while it does, so the path has to be nameable without first
// inspecting the workspace.
func QuarantineDirFor(transactionsDir string) string {
	return filepath.Join(transactionsDir, quarantineDirName)
}

// Inspect reads the transactions directory under dir and reports what is there
// without modifying anything. roots are the approved workspace roots, used only
// to judge whether a manifest's paths still resolve inside this machine's copy of
// the workspace; machineID is the inspecting machine, with "" meaning the running
// process. A missing directory is an empty inventory rather than an error, since a
// workspace that has never committed has no transactions directory.
func Inspect(dir string, roots []string, machineID string) (*Inventory, error) {
	journal := NewJournal(dir, roots, machineID)
	inv := &Inventory{Dir: journal.dir}

	entries, err := os.ReadDir(journal.dir)
	if os.IsNotExist(err) {
		return inv, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading transactions directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), manifestSuffix) {
			continue
		}
		path := filepath.Join(journal.dir, entry.Name())
		pending := PendingTransaction{
			ID:   strings.TrimSuffix(entry.Name(), manifestSuffix),
			Path: path,
		}
		manifest, loadErr := LoadManifest(path)
		if loadErr != nil {
			pending.LoadError = loadErr.Error()
		} else {
			pending.State = manifest.State
			pending.CreatedAt = manifest.CreatedAt
			pending.Machine = manifest.Machine
			pending.Heartbeat = manifest.Heartbeat
			pending.OwnedByThisMachine = manifest.Machine == journal.machineID
			pending.HeartbeatFresh = journal.heartbeatFresh(manifest.Heartbeat)
			pending.EscapesRoots = manifestEscapesRoots(journal, manifest)
		}
		inv.Pending = append(inv.Pending, pending)
	}
	sort.Slice(inv.Pending, func(i, j int) bool { return inv.Pending[i].ID < inv.Pending[j].ID })

	quarantined, err := inspectQuarantine(inv.QuarantineDir())
	if err != nil {
		return nil, err
	}
	inv.Quarantined = quarantined
	return inv, nil
}

// inspectQuarantine lists the quarantine directory. A missing directory is the
// ordinary case: nothing has ever been quarantined.
func inspectQuarantine(qdir string) ([]QuarantinedTransaction, error) {
	entries, err := os.ReadDir(qdir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading quarantine directory: %w", err)
	}
	var out []QuarantinedTransaction
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), manifestSuffix) {
			continue
		}
		path := filepath.Join(qdir, entry.Name())
		record := QuarantinedTransaction{
			ID:   strings.TrimSuffix(entry.Name(), manifestSuffix),
			Path: path,
		}
		if manifest, loadErr := LoadManifest(path); loadErr != nil {
			record.LoadError = loadErr.Error()
		} else {
			record.State = manifest.State
			record.Machine = manifest.Machine
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// manifestEscapesRoots reports whether any operation names a path outside the
// approved roots, the same check recoverManifest applies before acting.
func manifestEscapesRoots(journal *Journal, m *Manifest) bool {
	for _, op := range m.Ops {
		if !journal.withinRoots(op.Target) {
			return true
		}
		if op.Kind == OpMove && !journal.withinRoots(op.Source) {
			return true
		}
	}
	return false
}
