package transaction

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// ErrRecoveryQuarantine is wrapped by RecoveryReport.Err when recovery moved one
// or more irrecoverable manifests to quarantine. It is the sentinel the workspace
// layer matches to block further mutation until a human resolves the quarantine.
var ErrRecoveryQuarantine = errors.New("transaction: irrecoverable transactions quarantined; workspace mutations are blocked until resolved")

// quarantineDirName is the subdirectory of the transactions directory into which
// irrecoverable manifests are moved. It is a directory so List and the top-level
// manifest scan skip it.
const quarantineDirName = "quarantine"

// QuarantineEntry describes a manifest recovery could neither roll forward nor
// safely discard, along with why. It is surfaced in a RecoveryReport so a doctor
// command can present an actionable message.
type QuarantineEntry struct {
	ID     string
	Path   string
	Reason string
}

// RecoveryReport records the outcome of a Recover pass: which transactions were
// rolled forward to completion, which committed transactions were reclaimed,
// which orphaned staging files were swept, and which manifests were quarantined.
type RecoveryReport struct {
	// RolledForward holds the ids of prepared or committing transactions whose
	// file operations were applied and marked committed during this pass.
	RolledForward []string
	// CleanedUp holds the ids of already-committed (or discardable) transactions
	// whose staged artifacts and manifest were reclaimed during this pass.
	CleanedUp []string
	// SweptOrphans holds the absolute paths of orphan staging files removed —
	// staged bytes left by a crash before any manifest referenced them.
	SweptOrphans []string
	// ForeignLive holds live transactions another machine is still driving, which
	// this pass deliberately left alone. They are reported rather than recovered
	// and they do not block: the workspace this machine opens is complete, and the
	// owning machine will finish or a later pass will adopt the transaction once
	// its heartbeat goes stale.
	ForeignLive []ForeignTransaction
	// Quarantined holds every manifest recovery refused to act on, including any
	// quarantined by an earlier pass and still awaiting resolution.
	Quarantined []QuarantineEntry
}

// ForeignTransaction describes an in-flight transaction owned by a different
// machine whose heartbeat is still fresh. Recovery neither applies nor discards
// it, because both would race the machine that is mid-commit: applying it can
// interleave renames with the owner's own, and discarding it destroys staged
// bytes the owner is about to consume.
type ForeignTransaction struct {
	ID        string
	Path      string
	State     State
	Machine   string
	Heartbeat string
}

// Blocked reports whether recovery quarantined any manifest. While true, the
// workspace must refuse further mutation until the quarantine is resolved.
func (r *RecoveryReport) Blocked() bool { return len(r.Quarantined) > 0 }

// Err returns an actionable, quarantine-wrapped error when recovery is blocked,
// or nil otherwise. The message names each quarantined transaction so a human can
// inspect the moved manifests and staged bytes under the quarantine directory.
func (r *RecoveryReport) Err() error {
	if !r.Blocked() {
		return nil
	}
	var b strings.Builder
	if len(r.Quarantined) == 1 {
		b.WriteString("1 transaction requires attention")
	} else {
		fmt.Fprintf(&b, "%d transactions require attention", len(r.Quarantined))
	}
	for _, q := range r.Quarantined {
		fmt.Fprintf(&b, "\n  - %s: %s (manifest at %s)", q.ID, q.Reason, q.Path)
	}
	return fmt.Errorf("%s: %w", b.String(), ErrRecoveryQuarantine)
}

// manifestEntry pairs a top-level manifest file with its load result so a corrupt
// manifest can be quarantined individually instead of aborting the whole pass.
type manifestEntry struct {
	id       string
	path     string
	manifest *Manifest
	loadErr  error
}

// Recover finishes or cleans up any transaction interrupted by a crash before
// normal workspace loading. It rolls prepared and committing transactions forward
// deterministically (verifying staged bytes against their recorded hashes rather
// than guessing from timestamps), reclaims already-committed transactions, sweeps
// orphaned staging files, and quarantines any manifest it cannot safely act on.
//
// Recovery is idempotent: once a transaction is rolled forward and reclaimed a
// later pass finds nothing to do for it, and re-applying a partially applied
// transaction converges on the same committed result.
func (j *Journal) Recover() (*RecoveryReport, error) {
	return j.recover(true)
}

// RecoverPending completes every journaled transaction and checks quarantine,
// without walking the workspace for unreferenced staging files. Mutation paths
// use this while holding the commit lock: orphan stages cannot affect live data
// or a later transaction, whose staging names carry a fresh ID. Full startup
// recovery still sweeps them, including stages left by older processes.
func (j *Journal) RecoverPending() (*RecoveryReport, error) {
	return j.recover(false)
}

func (j *Journal) recover(sweep bool) (*RecoveryReport, error) {
	report := &RecoveryReport{}

	// protected collects every transaction id known to have a manifest (loadable,
	// corrupt, or already quarantined). A staging file is an orphan only when its
	// id is absent from this set, so recovery never sweeps bytes a manifest still
	// references.
	protected := map[string]bool{}

	// Prior quarantine persists across passes: surface it and protect its staged
	// bytes so the blocked state stays visible until a human clears it.
	if err := j.loadExistingQuarantine(report, protected); err != nil {
		return report, err
	}

	manifests, err := j.enumerateManifests(protected)
	if err != nil {
		return report, err
	}

	for _, me := range manifests {
		if me.loadErr != nil {
			j.quarantine(report, me.id, me.path, fmt.Sprintf("manifest could not be loaded: %v", me.loadErr))
			continue
		}
		if foreign, ok := j.foreignLive(me.manifest, me.path); ok {
			report.ForeignLive = append(report.ForeignLive, foreign)
			continue
		}
		if err := j.recoverManifest(report, me.manifest, me.path); err != nil {
			j.quarantine(report, me.id, me.path, err.Error())
		}
	}

	if sweep {
		if err := j.sweepOrphans(report, protected); err != nil {
			return report, err
		}
	}
	return report, nil
}

// enumerateManifests reads the top-level transactions directory, recording every
// transaction id in protected and returning each manifest paired with its load
// result. A missing directory yields no manifests.
func (j *Journal) enumerateManifests(protected map[string]bool) ([]manifestEntry, error) {
	entries, err := os.ReadDir(j.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading transactions directory: %w", err)
	}
	var out []manifestEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), manifestSuffix) {
			continue
		}
		id := strings.TrimSuffix(e.Name(), manifestSuffix)
		protected[id] = true
		path := filepath.Join(j.dir, e.Name())
		m, loadErr := LoadManifest(path)
		out = append(out, manifestEntry{id: id, path: path, manifest: m, loadErr: loadErr})
	}
	return out, nil
}

// loadExistingQuarantine records manifests quarantined by an earlier pass, adding
// their ids to protected and re-reporting them so the workspace stays blocked
// until they are resolved by hand.
func (j *Journal) loadExistingQuarantine(report *RecoveryReport, protected map[string]bool) error {
	qdir := filepath.Join(j.dir, quarantineDirName)
	entries, err := os.ReadDir(qdir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading quarantine directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), manifestSuffix) {
			continue
		}
		id := strings.TrimSuffix(e.Name(), manifestSuffix)
		protected[id] = true
		report.Quarantined = append(report.Quarantined, QuarantineEntry{
			ID:     id,
			Path:   filepath.Join(qdir, e.Name()),
			Reason: "previously quarantined; awaiting manual resolution",
		})
	}
	return nil
}

// ForeignTransactionGrace is how long a live transaction's heartbeat vouches for
// its owning machine. Within the grace period the owner is presumed to be mid
// commit and the transaction is left alone; past it the transaction is treated as
// abandoned and adopted by whichever machine recovers next, which is what keeps a
// machine that dies mid-commit from wedging the workspace for the other one.
//
// The window is generous on purpose. A heartbeat is refreshed only when the
// manifest changes state, so a large commit legitimately goes quiet for as long
// as its file operations take, and a synced tree adds the sync service's own
// propagation delay on top. Waiting too long only postpones an interrupted
// transaction's completion; adopting too early corrupts a live one.
const ForeignTransactionGrace = 10 * time.Minute

// foreignLive reports whether a manifest belongs to another machine that is
// still driving it, and must therefore be reported instead of recovered.
//
// A manifest with no recorded owner is not foreign: that is what a build
// predating transaction ownership wrote, and refusing to recover it would strand
// exactly the transactions this machine is responsible for. A committed manifest
// is never foreign either — reclaiming its staged bytes is idempotent and safe
// from any machine, and leaving it would strand those bytes on whichever machine
// happens to open the workspace next.
func (j *Journal) foreignLive(m *Manifest, path string) (ForeignTransaction, bool) {
	if m.Machine == "" || m.Machine == j.machineID {
		return ForeignTransaction{}, false
	}
	switch m.State {
	case StatePreparing, StatePrepared, StateCommitting:
	default:
		return ForeignTransaction{}, false
	}
	if !j.heartbeatFresh(m.Heartbeat) {
		return ForeignTransaction{}, false
	}
	return ForeignTransaction{
		ID:        m.ID,
		Path:      path,
		State:     m.State,
		Machine:   m.Machine,
		Heartbeat: m.Heartbeat,
	}, true
}

// heartbeatFresh reports whether a heartbeat still vouches for a live owner. A
// missing or unparseable heartbeat cannot, so it reads as stale and the
// transaction becomes adoptable — the same answer an owner-less manifest gets.
//
// A heartbeat stamped in the future, which two machines with skewed clocks
// inside one synced workspace produce routinely, yields a negative age and so
// reads as fresh. That is the safe direction: it defers to the other machine
// rather than adopting a transaction it may still be applying.
func (j *Journal) heartbeatFresh(heartbeat string) bool {
	if heartbeat == "" {
		return false
	}
	beat, err := time.Parse(time.RFC3339Nano, heartbeat)
	if err != nil {
		return false
	}
	return j.now().UTC().Sub(beat) < ForeignTransactionGrace
}

// recoverManifest acts on a single loadable manifest according to its state. It
// returns an error (never partially mutating the report) when the transaction is
// irrecoverable, so the caller quarantines it.
func (j *Journal) recoverManifest(report *RecoveryReport, m *Manifest, path string) error {
	// Defense in depth: a manifest that somehow names a target outside the
	// approved roots is refused rather than applied.
	for i, op := range m.Ops {
		if !j.withinRoots(op.Target) {
			return fmt.Errorf("op %d target %q escapes approved workspace roots", i, op.Target)
		}
		if op.Kind == OpMove && !j.withinRoots(op.Source) {
			return fmt.Errorf("op %d move source %q escapes approved workspace roots", i, op.Source)
		}
	}

	switch m.State {
	case StatePrepared, StateCommitting:
		return j.rollForward(report, m, path)
	case StateCommitted:
		if err := j.reclaim(m, path); err != nil {
			return err
		}
		report.CleanedUp = append(report.CleanedUp, m.ID)
		return nil
	case StatePreparing:
		// Staging is not guaranteed durable, so the transaction never reached its
		// commit point: discard it rather than applying half-staged bytes.
		if err := j.reclaim(m, path); err != nil {
			return err
		}
		report.CleanedUp = append(report.CleanedUp, m.ID)
		return nil
	default:
		return fmt.Errorf("unknown manifest state %q", m.State)
	}
}

// rollForward applies every file operation of a prepared or committing manifest,
// advancing it through committing to committed, then reclaims its artifacts. The
// committing manifest write is a durable intent marker so a crash mid-apply is
// re-driven to completion on the next pass. Apply is idempotent, so re-running
// after such a crash converges on the same result.
func (j *Journal) rollForward(report *RecoveryReport, m *Manifest, path string) error {
	if err := j.driveCommit(m, path); err != nil {
		return err
	}
	report.RolledForward = append(report.RolledForward, m.ID)
	return nil
}

// quarantine moves an irrecoverable manifest into the quarantine directory and
// records it in the report. Its staged bytes are deliberately left in place for
// inspection; the manifest id stays protected so they are not swept as orphans.
func (j *Journal) quarantine(report *RecoveryReport, id, path, reason string) {
	entry := QuarantineEntry{ID: id, Path: path, Reason: reason}
	qdir := filepath.Join(j.dir, quarantineDirName)
	if err := j.mkdirAll(qdir, util.DirPermissions); err != nil {
		entry.Reason = fmt.Sprintf("%s; additionally failed to create quarantine directory: %v", reason, err)
		report.Quarantined = append(report.Quarantined, entry)
		return
	}
	dest := filepath.Join(qdir, id+manifestSuffix)
	if err := j.rename(path, dest); err != nil {
		entry.Reason = fmt.Sprintf("%s; additionally failed to move manifest into quarantine: %v", reason, err)
		report.Quarantined = append(report.Quarantined, entry)
		return
	}
	entry.Path = dest
	report.Quarantined = append(report.Quarantined, entry)
}

// sweepOrphans removes staging files left beside their targets by a crash before
// any manifest referenced them. A staging file is an orphan only when the
// transaction id encoded in its name has no manifest in protected.
func (j *Journal) sweepOrphans(report *RecoveryReport, protected map[string]bool) error {
	seen := map[string]bool{}
	for _, root := range j.roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			id, ok := parseStageTxnID(d.Name())
			if !ok || protected[id] || seen[path] {
				return nil
			}
			seen[path] = true
			if err := j.remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("sweeping orphan staging file %q: %w", path, err)
			}
			report.SweptOrphans = append(report.SweptOrphans, path)
			return nil
		})
		if err != nil {
			return fmt.Errorf("sweeping staging files under %q: %w", root, err)
		}
	}
	return nil
}

// parseStageTxnID extracts the transaction id from a staging file name of the
// form ".tt-stage-<id>-<index>.stage". The id may itself contain hyphens (it is a
// UUID), so the trailing "-<index>" is split off from the right. It returns false
// for any name that is not a well-formed staging file.
func parseStageTxnID(name string) (string, bool) {
	if !strings.HasPrefix(name, stagePrefix) || !strings.HasSuffix(name, stageSuffix) {
		return "", false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(name, stagePrefix), stageSuffix)
	dash := strings.LastIndex(body, "-")
	if dash <= 0 || dash == len(body)-1 {
		return "", false
	}
	id, index := body[:dash], body[dash+1:]
	for _, r := range index {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return id, true
}
