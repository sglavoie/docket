package storage_test

// The sync fault harness: a deterministic multi-replica simulation of devices
// that edit offline, publish immutable transactions to a shared exchange, and
// receive them through a faulty transport, with controlled clocks, minted
// identities, crashes and restarts. Every random choice comes from one seeded
// source and every event is appended to a trace, so a failure names its seed,
// the command that replays it, and the events that led to it.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/syncmerge"
)

// faultFixtureDir is the representative compatibility workspace every replica
// starts from.
const faultFixtureDir = "../compat/testdata/workspaces/representative"

const (
	faultAtlasID  = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"
	faultNotePath = "projects/atlas-launch/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md"
	// faultSeedEnv replays one seed of a seeded suite.
	faultSeedEnv = "TT_SYNC_FAULT_SEED"
	// faultTraceEnv names the directory failure traces are written to.
	faultTraceEnv = "TT_SYNC_FAULT_TRACE_DIR"
)

var faultSlugs = []string{"atlas-launch", "home-renovation"}

var faultSchemas = map[string]int{
	synclog.SchemaDomainEnvelope: config.CurrentDomainSchemaVersion,
	synclog.SchemaCatalog:        config.CurrentConfigVersion,
	synclog.SchemaTaskMarkdown:   storage.CurrentTaskSchemaVersion,
}

// faultConfig is how hostile the transport and the devices' processes are.
// Probabilities apply per message, per blob, or per commit.
type faultConfig struct {
	// MaxDelay is the most ticks a delivery or a blob download may lag.
	MaxDelay int
	// Duplicate delivers a manifest a second time.
	Duplicate float64
	// Truncate first delivers an incomplete copy of a manifest file.
	Truncate float64
	// Corrupt first delivers a copy with one byte changed.
	Corrupt float64
	// BlobLag makes a blob arrive after the manifest naming it.
	BlobLag float64
	// CorruptBlob serves one damaged read of a blob before the real bytes.
	CorruptBlob float64
	// Crash stops a commit at a random point of its journal transaction.
	Crash float64
	// Restart restarts a device's process between operations.
	Restart float64
	// Offline takes a device offline, or back online, between operations.
	Offline float64
}

// faultSim is one seeded run.
type faultSim struct {
	t       *testing.T
	name    string
	seed    int64
	rng     *rand.Rand
	cfg     faultConfig
	tick    int
	genesis string
	devices []*simDevice
	ex      *exchange
	minted  int
	queued  int
	writes  []userWrite
	trace   []string
	stats   map[string]int
	// fail reports an invariant violation; it never returns.
	fail func(report string)
}

// simDevice is one replica: a workspace, its process (history and commit
// service, rebuilt on every restart), and its transport inbox.
type simDevice struct {
	name     string
	root     string
	observer bool
	history  *synclog.History
	svc      *storage.CommitService
	offline  bool
	inbox    []*delivery
	// blobDue is the tick a lagging blob download completes.
	blobDue map[string]int
	// corruptBlob marks blobs whose next read is damaged.
	corruptBlob map[string]bool
	// applied is the highest exchange position applied, to measure reordering.
	applied int
}

// delivery is one manifest file arriving at a device, possibly damaged.
type delivery struct {
	txnID string
	data  []byte
	fault string
	due   int
	order int
}

// exchange is the shared folder: immutable manifests by content address, in
// publication order, and the blobs they name.
type exchange struct {
	manifests map[string][]byte
	position  map[string]int
	order     []string
	blobs     map[string][]byte
}

// userWrite is one user-visible value a transaction wrote: a unique marker
// placed in a task title or appended to a note, a task deletion, or a
// resolution of a file's conflicts. The preservation check reads them.
type userWrite struct {
	txn     string
	device  string
	path    string
	subject string
	marker  string
	kind    string
}

func newFaultSim(t *testing.T, name string, seed int64, cfg faultConfig, devices ...string) *faultSim {
	t.Helper()
	s := &faultSim{
		t:     t,
		name:  name,
		seed:  seed,
		rng:   rand.New(rand.NewSource(seed)),
		cfg:   cfg,
		ex:    &exchange{manifests: map[string][]byte{}, position: map[string]int{}, blobs: map[string][]byte{}},
		stats: map[string]int{},
	}
	s.fail = func(report string) {
		t.Helper()
		t.Fatal(report)
	}
	// Journal transaction ids are minted by uuid; a seeded source makes them,
	// and so every sync transaction id, a function of the seed.
	uuid.SetRand(rand.New(rand.NewSource(seed ^ 0x7a11)))
	t.Cleanup(func() { uuid.SetRand(nil) })
	s.genesis = filepath.Join(t.TempDir(), "genesis")
	copyFaultTree(t, faultFixtureDir, s.genesis)
	for _, device := range devices {
		s.addDevice(device, false)
	}
	s.logf("start seed=%d devices=%s config=%+v", seed, strings.Join(devices, ","), cfg)
	return s
}

// now is the controlled clock every history stamps records with.
func (s *faultSim) now() time.Time {
	return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC).Add(time.Duration(s.tick) * time.Minute)
}

func (s *faultSim) logf(format string, args ...any) {
	s.trace = append(s.trace, fmt.Sprintf("t%04d ", s.tick)+fmt.Sprintf(format, args...))
}

func (s *faultSim) count(stat string) { s.stats[stat]++ }

func (s *faultSim) chance(p float64) bool { return p > 0 && s.rng.Float64() < p }

// fatalf fails the run with its seed, the command replaying it, and its trace.
func (s *faultSim) fatalf(format string, args ...any) {
	s.t.Helper()
	s.logf("FAIL "+format, args...)
	s.fail(s.report(fmt.Sprintf(format, args...)))
	panic("faultSim.fail returned")
}

// report renders a failure: the reason, the seed and scenario, a replay
// command, where the full trace was written, and the trace's tail.
func (s *faultSim) report(reason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "sync fault failure: %s\n", reason)
	fmt.Fprintf(&b, "scenario=%s seed=%d tick=%d\n", s.name, s.seed, s.tick)
	fmt.Fprintf(&b, "replay: %s=%d go test -count=1 -run '^%s$' ./internal/storage/\n", faultSeedEnv, s.seed, regexp.QuoteMeta(topLevelTest(s.t.Name())))
	dir := os.Getenv(faultTraceEnv)
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "tt-sync-fault")
	}
	path := filepath.Join(dir, strings.NewReplacer("/", "__", " ", "_").Replace(s.t.Name())+fmt.Sprintf("-seed-%d.trace", s.seed))
	full := strings.Join(s.trace, "\n") + "\n"
	if err := os.MkdirAll(dir, 0o700); err == nil && os.WriteFile(path, []byte(full), 0o600) == nil {
		fmt.Fprintf(&b, "trace: %d events written to %s\n", len(s.trace), path)
	} else {
		fmt.Fprintf(&b, "trace: %d events (could not be written to %s)\n", len(s.trace), path)
	}
	tail := s.trace
	if len(tail) > 60 {
		tail = tail[len(tail)-60:]
		b.WriteString("  ...\n")
	}
	for _, line := range tail {
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func topLevelTest(name string) string {
	top, _, _ := strings.Cut(name, "/")
	return top
}

// summary is the run's fault and outcome counts, in a stable order.
func (s *faultSim) summary() string {
	keys := make([]string, 0, len(s.stats))
	for key := range s.stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, s.stats[key]))
	}
	return strings.Join(parts, " ")
}

// --- devices and processes ---

func (s *faultSim) addDevice(name string, observer bool) *simDevice {
	s.t.Helper()
	d := &simDevice{
		name:        name,
		root:        filepath.Join(s.t.TempDir(), name),
		observer:    observer,
		blobDue:     map[string]int{},
		corruptBlob: map[string]bool{},
		applied:     -1,
	}
	copyFaultTree(s.t, s.genesis, d.root)
	s.open(d)
	s.devices = append(s.devices, d)
	return d
}

func (s *faultSim) device(name string) *simDevice {
	for _, d := range s.devices {
		if d.name == name {
			return d
		}
	}
	s.fatalf("no device %s", name)
	return nil
}

// authors are the devices that edit, as opposed to genesis observers.
func (s *faultSim) authors() []*simDevice {
	var out []*simDevice
	for _, d := range s.devices {
		if !d.observer {
			out = append(out, d)
		}
	}
	return out
}

// open starts a device's process: a fresh history and commit service over the
// workspace on disk, exactly what a relaunch builds.
func (s *faultSim) open(d *simDevice) {
	history, err := synclog.New(synclog.Config{DataDir: d.root, DeviceID: d.name, Schemas: faultSchemas, Now: s.now})
	if err != nil {
		s.fatalf("%s: synclog.New: %v", d.name, err)
	}
	ledger := storage.NewWorkspaceLedger(filepath.Join(d.root, ".tt", "undo"), []string{d.root})
	d.history = history
	d.svc = storage.NewCommitServiceWithLedger(d.txDir(), []string{d.root}, ledger).
		WithSyncHistory(history).
		WithSyncMerger(syncmerge.Merger{})
}

func (d *simDevice) txDir() string { return filepath.Join(d.root, ".tt", "transactions") }

func (d *simDevice) path(rel string) string { return filepath.Join(d.root, filepath.FromSlash(rel)) }

// restart drops the device's process and starts a new one, which recovers
// whatever journal transaction the old one left, then holds the device to
// the atomicity invariants.
func (s *faultSim) restart(d *simDevice, why string) {
	s.open(d)
	report, err := d.svc.RecoverWorkspace()
	if err != nil {
		s.fatalf("%s: recovery after %s: %v", d.name, why, err)
	}
	if report.Blocked() {
		s.fatalf("%s: recovery after %s quarantined %+v", d.name, why, report.Quarantined)
	}
	s.count("restarts")
	s.logf("%s restart (%s)", d.name, why)
	s.requireAtomic(d, why)
}

// requireAtomic holds a device to what an atomic commit boundary leaves: an
// empty journal, a workspace holding exactly the bytes the history last
// recorded (no edit without its record, no record without its edit), and
// receipts, the index and the received sequence numbers in agreement.
func (s *faultSim) requireAtomic(d *simDevice, why string) {
	entries, err := os.ReadDir(d.txDir())
	if err != nil && !os.IsNotExist(err) {
		s.fatalf("%s: read journal: %v", d.name, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			s.fatalf("%s: journal still holds %s after %s", d.name, entry.Name(), why)
		}
	}
	state, err := d.history.LoadState()
	if err != nil {
		s.fatalf("%s: load state after %s: %v", d.name, why, err)
	}
	if state == nil || !state.Indexed {
		return
	}
	disk := s.syncedFiles(d)
	for rel, digest := range disk {
		if state.Tracked[rel] != digest {
			s.fatalf("%s: after %s %s holds %s but the history recorded %q", d.name, why, rel, digest, state.Tracked[rel])
		}
	}
	for rel := range state.Tracked {
		if _, ok := disk[rel]; !ok {
			s.fatalf("%s: after %s the history records %s, which is not on disk", d.name, why, rel)
		}
	}
	for _, txnID := range s.ex.order {
		var m synclog.Manifest
		if err := json.Unmarshal(s.ex.manifests[txnID], &m); err != nil {
			s.fatalf("decode exchange manifest: %v", err)
		}
		if m.DeviceID == d.name {
			continue
		}
		_, known, err := d.history.Lookup(txnID)
		if err != nil {
			s.fatalf("%s: lookup: %v", d.name, err)
		}
		if received := state.Received[m.DeviceID] >= m.DeviceSeq; received != known {
			s.fatalf("%s: after %s %s seq %d is indexed=%v but received=%v", d.name, why, m.DeviceID, m.DeviceSeq, known, received)
		}
	}
}

// fingerprint is everything a refused or repeated delivery must leave alone:
// the device's sync state, its synced files, and its stored conflicts.
func (s *faultSim) fingerprint(d *simDevice) string {
	var b strings.Builder
	state, _ := os.ReadFile(d.history.StatePath())
	b.Write(state)
	files := s.syncedFiles(d)
	for _, rel := range sortedKeys(files) {
		fmt.Fprintf(&b, "%s %s\n", rel, files[rel])
	}
	conflicts, _ := filepath.Glob(filepath.Join(d.history.Dir(), "conflicts", "*", "*.json"))
	sort.Strings(conflicts)
	for _, path := range conflicts {
		b.WriteString(filepath.Base(path) + "\n")
	}
	return b.String()
}

// --- the transport ---

// blobSource is the exchange as one device sees it: blobs still downloading
// are absent and a damaged read is served once.
type blobSource struct {
	s *faultSim
	d *simDevice
}

func (b blobSource) Blob(digest string) ([]byte, error) {
	if due, lagging := b.d.blobDue[digest]; lagging && b.s.tick < due {
		b.s.count("blob_unavailable_reads")
		return nil, fmt.Errorf("blob %s is still downloading", digest)
	}
	data, ok := b.s.ex.blobs[digest]
	if !ok {
		return nil, fmt.Errorf("blob %s is not in the exchange", digest)
	}
	if b.d.corruptBlob[digest] {
		delete(b.d.corruptBlob, digest)
		b.s.count("blob_corrupt_reads")
		damaged := append([]byte{}, data...)
		damaged[len(damaged)/2] ^= 0x20
		return damaged, nil
	}
	return data, nil
}

func manifestDigests(m *synclog.Manifest) []string {
	var out []string
	for _, effect := range m.Effects {
		for _, digest := range []*string{effect.BaseBlob, effect.Blob} {
			if digest != nil {
				out = append(out, *digest)
			}
		}
	}
	return out
}

// publish copies every outgoing transaction of d, and every blob it names,
// into the exchange, queues it for every other device, and then confirms the
// publication, possibly crashing while it does.
func (s *faultSim) publish(d *simDevice) {
	if d.offline || d.observer {
		return
	}
	pending, err := d.history.Pending()
	if err != nil {
		s.fatalf("%s: pending: %v", d.name, err)
	}
	if len(pending) == 0 {
		return
	}
	ids := make([]string, 0, len(pending))
	for _, m := range pending {
		data, encodeErr := m.Encode()
		if encodeErr != nil {
			s.fatalf("%s: encode: %v", d.name, encodeErr)
		}
		for _, digest := range manifestDigests(m) {
			blob, blobErr := d.history.Blob(digest)
			if blobErr != nil {
				s.fatalf("%s: outgoing %s names blob %s it does not retain: %v", d.name, m.TxnID, digest, blobErr)
			}
			s.ex.blobs[digest] = blob
		}
		if previous, seen := s.ex.manifests[m.TxnID]; seen {
			if !bytes.Equal(previous, data) {
				s.fatalf("%s republished %s with different bytes", d.name, m.TxnID)
			}
			s.count("republished")
			s.logf("%s republish %s seq %d", d.name, short(m.TxnID), m.DeviceSeq)
		} else {
			s.ex.manifests[m.TxnID] = data
			s.ex.position[m.TxnID] = len(s.ex.order)
			s.ex.order = append(s.ex.order, m.TxnID)
			s.count("published")
			s.logf("%s publish %s seq %d kind %s parents %d effects %d", d.name, short(m.TxnID), m.DeviceSeq, m.Kind, len(m.Parents), len(m.Effects))
		}
		for _, other := range s.devices {
			if other != d {
				s.enqueue(other, m, data)
			}
		}
		ids = append(ids, m.TxnID)
	}
	before := len(pending)
	crashed := s.withCrash(d, "publication confirmation", func() error { return d.svc.CommitSyncPublished(ids) })
	after, err := d.history.Pending()
	if err != nil {
		s.fatalf("%s: pending: %v", d.name, err)
	}
	switch {
	case !crashed && len(after) != 0:
		s.fatalf("%s: %d transactions still pending after confirming publication", d.name, len(after))
	case crashed && len(after) != 0 && len(after) != before:
		s.fatalf("%s: a crashed confirmation drained %d of %d transactions", d.name, before-len(after), before)
	}
}

// enqueue sends a published manifest toward d through the faulty transport.
func (s *faultSim) enqueue(d *simDevice, m *synclog.Manifest, data []byte) {
	delay := 0
	if s.cfg.MaxDelay > 0 {
		delay = s.rng.Intn(s.cfg.MaxDelay + 1)
	}
	if delay > 0 {
		s.count("delayed")
	}
	if s.chance(s.cfg.Truncate) {
		s.push(d, m.TxnID, data[:1+s.rng.Intn(len(data)-3)], "truncated", s.tick+s.rng.Intn(delay+1))
	}
	if s.chance(s.cfg.Corrupt) {
		damaged := append([]byte{}, data...)
		damaged[s.rng.Intn(len(damaged))] ^= byte(1 + s.rng.Intn(255))
		s.push(d, m.TxnID, damaged, "corrupted", s.tick+s.rng.Intn(delay+1))
	}
	s.push(d, m.TxnID, data, "", s.tick+delay)
	if s.chance(s.cfg.Duplicate) {
		extra := 0
		if s.cfg.MaxDelay > 0 {
			extra = s.rng.Intn(s.cfg.MaxDelay + 1)
		}
		s.push(d, m.TxnID, data, "duplicate", s.tick+extra)
	}
	for _, digest := range manifestDigests(m) {
		if _, lagging := d.blobDue[digest]; lagging {
			continue
		}
		if s.chance(s.cfg.BlobLag) {
			d.blobDue[digest] = s.tick + 1 + s.rng.Intn(s.cfg.MaxDelay+2)
			s.count("blob_lagged")
		}
		if s.chance(s.cfg.CorruptBlob) {
			d.corruptBlob[digest] = true
			s.count("blob_corrupted")
		}
	}
}

func (s *faultSim) push(d *simDevice, txnID string, data []byte, fault string, due int) {
	s.queued++
	if fault != "" {
		s.count("sent_" + fault)
	}
	d.inbox = append(d.inbox, &delivery{txnID: txnID, data: data, fault: fault, due: due, order: s.queued})
}

// drain delivers, in a seeded order, everything due at d.
func (s *faultSim) drain(d *simDevice) {
	if d.offline {
		return
	}
	var ready, later []*delivery
	for _, del := range d.inbox {
		if del.due <= s.tick {
			ready = append(ready, del)
		} else {
			later = append(later, del)
		}
	}
	d.inbox = later
	s.rng.Shuffle(len(ready), func(i, j int) { ready[i], ready[j] = ready[j], ready[i] })
	for _, del := range ready {
		s.deliver(d, del)
	}
}

// requeue retries a delivery the device could not apply yet.
func (s *faultSim) requeue(d *simDevice, del *delivery) {
	s.push(d, del.txnID, s.ex.manifests[del.txnID], "", s.tick+1)
}

// deliver hands one manifest file to d, holding every outcome to its
// invariant: a damaged file is refused and changes nothing, a repeat changes
// nothing, an incomplete one changes nothing and is retried, and an apply is
// atomic even when the process dies in the middle of it.
func (s *faultSim) deliver(d *simDevice, del *delivery) {
	m, err := synclog.DecodeManifest(del.data)
	if err != nil {
		if del.fault == "" {
			s.fatalf("%s: a complete manifest %s does not decode: %v", d.name, short(del.txnID), err)
		}
		if !errors.Is(err, synclog.ErrInvalidManifest) && !errors.Is(err, synclog.ErrHeld) {
			s.fatalf("%s: a %s manifest was refused as %v", d.name, del.fault, err)
		}
		s.count("refused_" + del.fault)
		s.logf("%s refuse %s copy of %s", d.name, del.fault, short(del.txnID))
		return
	}
	if m.TxnID != del.txnID {
		s.fatalf("%s: a %s copy of %s decoded as a different valid transaction %s", d.name, del.fault, short(del.txnID), short(m.TxnID))
	}
	if del.fault == "truncated" || del.fault == "corrupted" {
		// Damage to insignificant whitespace leaves the same transaction.
		s.count("harmless_" + del.fault)
	}
	before := s.fingerprint(d)
	var crashed bool
	err = nil
	crashed = s.withCrash(d, "incoming "+short(m.TxnID), func() error {
		_, err = d.svc.CommitIncoming(m, blobSource{s: s, d: d})
		return err
	})
	if crashed {
		s.requeue(d, del)
		return
	}
	switch {
	case err == nil:
		s.count("applied")
		position := s.ex.position[m.TxnID]
		if position < d.applied {
			s.count("applied_out_of_order")
		}
		d.applied = max(d.applied, position)
		s.logf("%s apply %s from %s seq %d", d.name, short(m.TxnID), m.DeviceID, m.DeviceSeq)
	case errors.Is(err, synclog.ErrAlreadyApplied), errors.Is(err, synclog.ErrOwnTransaction):
		s.requireUnchanged(d, before, "a repeated delivery")
		s.count("repeat_ignored")
		s.logf("%s ignore repeat of %s", d.name, short(m.TxnID))
	case errors.Is(err, synclog.ErrIncomplete):
		s.requireUnchanged(d, before, "an incomplete delivery")
		if strings.Contains(err.Error(), "blob") {
			s.count("waiting_for_blob")
		} else {
			s.count("waiting_for_ancestor")
		}
		s.logf("%s wait %s: %v", d.name, short(m.TxnID), err)
		s.requeue(d, del)
	default:
		s.fatalf("%s: apply %s from %s seq %d: %v", d.name, short(m.TxnID), m.DeviceID, m.DeviceSeq, err)
	}
}

func (s *faultSim) requireUnchanged(d *simDevice, before, what string) {
	if after := s.fingerprint(d); after != before {
		s.fatalf("%s: %s changed the device:\n%s\n---\n%s", d.name, what, before, after)
	}
}

// withCrash runs a commit, and with the configured probability stops its
// journal transaction at a seeded point and restarts the device's process.
// It reports whether the commit was stopped.
func (s *faultSim) withCrash(d *simDevice, what string, fn func() error) bool {
	if !s.chance(s.cfg.Crash) {
		if err := fn(); err != nil && !isRefusal(err) {
			s.fatalf("%s: %s: %v", d.name, what, err)
		}
		return false
	}
	prefix, ops, crashed, err := storage.CrashCommitForTest(s.t, d.txDir(), func(ops int) int { return s.rng.Intn(ops+2) - 1 }, fn)
	if !crashed {
		if err != nil && !isRefusal(err) {
			s.fatalf("%s: %s: %v", d.name, what, err)
		}
		return false
	}
	s.count("crashes")
	s.logf("%s crash during %s after %d of %d operations", d.name, what, prefix, ops)
	s.restart(d, "crash during "+what)
	return true
}

// isRefusal is an incoming manifest the device declines without applying it.
func isRefusal(err error) bool {
	return errors.Is(err, synclog.ErrAlreadyApplied) || errors.Is(err, synclog.ErrOwnTransaction) || errors.Is(err, synclog.ErrIncomplete)
}

func short(txnID string) string {
	hex := strings.TrimPrefix(txnID, "sha256:")
	if len(hex) > 10 {
		return hex[:10]
	}
	return hex
}

// --- local edits ---

// commitLocal commits one user edit on d, possibly crashing, and returns the
// sync transaction it recorded, or "" when the crash lost the edit.
func (s *faultSim) commitLocal(d *simDevice, what string, artifacts ...storage.ArtifactChange) string {
	changed := false
	for _, artifact := range artifacts {
		current, err := os.ReadFile(artifact.Path)
		if err != nil || !bytes.Equal(current, artifact.Data) {
			changed = true
		}
	}
	if !changed {
		s.logf("%s skip %s: it changes nothing", d.name, what)
		return ""
	}
	before, err := d.history.LoadState()
	if err != nil {
		s.fatalf("%s: state: %v", d.name, err)
	}
	var nextSeq uint64
	if before != nil {
		nextSeq = before.NextSeq
	}
	crashed := s.withCrash(d, what, func() error {
		_, commitErr := d.svc.CommitArtifacts(artifacts)
		return commitErr
	})
	state, err := d.history.LoadState()
	if err != nil {
		s.fatalf("%s: state after %s: %v", d.name, what, err)
	}
	if state == nil || state.NextSeq == nextSeq {
		if !crashed {
			s.fatalf("%s: %s recorded no transaction", d.name, what)
		}
		s.logf("%s lost %s to the crash", d.name, what)
		return ""
	}
	if len(state.Heads) != 1 {
		s.fatalf("%s: after %s heads are %v", d.name, what, state.Heads)
	}
	s.count("local_commits")
	s.logf("%s commit %s: %s", d.name, short(state.Heads[0]), what)
	return state.Heads[0]
}

func (s *faultSim) read(d *simDevice, rel string) []byte {
	data, err := os.ReadFile(d.path(rel))
	if err != nil {
		s.fatalf("%s: read %s: %v", d.name, rel, err)
	}
	return data
}

func (s *faultSim) doc(d *simDevice, slug string) *storage.ProjectDocument {
	doc, err := storage.ParseProjectDocument(string(s.read(d, "projects/"+slug+"/tasks.md")))
	if err != nil {
		s.fatalf("%s: parse %s: %v", d.name, slug, err)
	}
	return doc
}

func (s *faultSim) put(d *simDevice, rel string, data []byte) storage.ArtifactChange {
	return storage.ArtifactChange{Path: d.path(rel), Data: data, Mode: storage.FilePermissions}
}

func (s *faultSim) docArtifact(d *simDevice, slug string, doc *storage.ProjectDocument) storage.ArtifactChange {
	content, err := doc.WriteCurrent()
	if err != nil {
		s.fatalf("%s: render %s: %v", d.name, slug, err)
	}
	return s.put(d, "projects/"+slug+"/tasks.md", []byte(content))
}

func (s *faultSim) marker(d *simDevice) string {
	s.minted++
	return fmt.Sprintf("mk%dx%04d%s", s.seed, s.minted, strings.TrimPrefix(d.name, "device-"))
}

func (s *faultSim) record(txn string, d *simDevice, path, subject, marker, kind string) {
	if txn != "" {
		s.writes = append(s.writes, userWrite{txn: txn, device: d.name, path: path, subject: subject, marker: marker, kind: kind})
	}
}

// retitle gives a task a fresh unique title.
func (s *faultSim) retitle(d *simDevice, slug, taskID string) {
	doc := s.doc(d, slug)
	task := findDocTask(doc, taskID)
	if task == nil {
		s.logf("%s cannot retitle missing task %s", d.name, taskID)
		return
	}
	marker := s.marker(d)
	task.Title = "Title " + marker
	txn := s.commitLocal(d, "retitle "+taskID[len(taskID)-3:]+" "+marker, s.docArtifact(d, slug, doc))
	s.record(txn, d, "projects/"+slug+"/tasks.md", "task:"+taskID, marker, "title")
}

// retitleRandom retitles a seeded task of a seeded project.
func (s *faultSim) retitleRandom(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	s.retitle(d, slug, doc.Tasks[s.rng.Intn(len(doc.Tasks))].ID)
}

// capture creates a root task with a minted identity and a unique title.
func (s *faultSim) capture(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	marker := s.marker(d)
	id := fmt.Sprintf("0a7e5d10-2c3b-4f6a-8e9d-%012x", 0xf0000000+s.seed*0x10000+int64(s.minted))
	template := *doc.Tasks[len(doc.Tasks)-1]
	task := &template
	task.ID = id
	task.Title = "Capture " + marker
	task.ParentID = nil
	task.Dependencies = nil
	task.Pinned = false
	task.PinnedAt = nil
	maxShort := 0
	for _, existing := range doc.Tasks {
		maxShort = max(maxShort, existing.ShortID)
	}
	task.ShortID = maxShort + 1
	doc.Tasks = append(doc.Tasks, task)
	txn := s.commitLocal(d, "capture "+marker, s.docArtifact(d, slug, doc))
	s.record(txn, d, "projects/"+slug+"/tasks.md", "task:"+id, marker, "title")
}

// deleteLeaf deletes a seeded task that has no children and no planning slot.
func (s *faultSim) deleteLeaf(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	if len(doc.Tasks) < 3 {
		return
	}
	task := doc.Tasks[s.rng.Intn(len(doc.Tasks))]
	planned := string(s.read(d, ".tt/planning.json"))
	for _, other := range doc.Tasks {
		if other.ParentID != nil && *other.ParentID == task.ID {
			return
		}
		for _, dep := range other.Dependencies {
			if strings.Contains(fmt.Sprint(dep), task.ID) {
				return
			}
		}
	}
	if strings.Contains(planned, task.ID) {
		return
	}
	kept := doc.Tasks[:0]
	for _, other := range doc.Tasks {
		if other.ID != task.ID {
			kept = append(kept, other)
		}
	}
	doc.Tasks = kept
	txn := s.commitLocal(d, "delete "+task.ID[len(task.ID)-3:], s.docArtifact(d, slug, doc))
	s.record(txn, d, "projects/"+slug+"/tasks.md", "task:"+task.ID, "", "delete")
}

// reprioritize sets a seeded task's priority.
func (s *faultSim) reprioritize(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	task := doc.Tasks[s.rng.Intn(len(doc.Tasks))]
	task.Priority = s.rng.Intn(4)
	s.commitLocal(d, fmt.Sprintf("priority %s %d", task.ID[len(task.ID)-3:], task.Priority), s.docArtifact(d, slug, doc))
}

// appendNote appends a unique line to the shared runbook note.
func (s *faultSim) appendNote(d *simDevice) {
	marker := s.marker(d)
	data := append(append([]byte{}, s.read(d, faultNotePath)...), []byte("- "+marker+"\n")...)
	txn := s.commitLocal(d, "note "+marker, s.put(d, faultNotePath, data))
	s.record(txn, d, faultNotePath, "note", marker, "note")
}

// plan moves a seeded open atlas task into Today or Next, or reverses Today.
func (s *faultSim) plan(d *simDevice) {
	raw := s.read(d, ".tt/planning.json")
	var env faultEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		s.fatalf("%s: planning envelope: %v", d.name, err)
	}
	planning := &config.PlanningState{}
	if err := json.Unmarshal(env.Data, planning); err != nil {
		s.fatalf("%s: planning: %v", d.name, err)
	}
	doc := s.doc(d, "atlas-launch")
	task := doc.Tasks[s.rng.Intn(len(doc.Tasks))]
	if task.IsTerminalState() || task.State() == models.StateBacklog {
		today := append([]config.ProjectTaskLocator{}, planning.Today...)
		for i, j := 0, len(today)-1; i < j; i, j = i+1, j-1 {
			today[i], today[j] = today[j], today[i]
		}
		planning.Today = today
	} else {
		item := config.ProjectTaskLocator{ProjectID: faultAtlasID, TaskID: task.ID}
		planning.Today = dropLocator(planning.Today, item)
		planning.Next = dropLocator(planning.Next, item)
		if s.rng.Intn(2) == 0 {
			planning.Today = append(planning.Today, item)
		} else {
			planning.Next = append(planning.Next, item)
		}
	}
	data, err := json.Marshal(planning)
	if err != nil {
		s.fatalf("encode planning: %v", err)
	}
	env.Data = data
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		s.fatalf("encode planning envelope: %v", err)
	}
	s.commitLocal(d, "plan "+task.ID[len(task.ID)-3:], s.put(d, ".tt/planning.json", out))
}

type faultEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
}

func dropLocator(items []config.ProjectTaskLocator, drop config.ProjectTaskLocator) []config.ProjectTaskLocator {
	var out []config.ProjectTaskLocator
	for _, item := range items {
		if item != drop {
			out = append(out, item)
		}
	}
	return out
}

func findDocTask(doc *storage.ProjectDocument, id string) *models.Task {
	for _, task := range doc.Tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// conflicts are the sync conflicts d derives.
func (s *faultSim) conflicts(d *simDevice) []synclog.Conflict {
	conflicts, err := d.svc.SyncConflicts()
	if err != nil {
		s.fatalf("%s: conflicts: %v", d.name, err)
	}
	return conflicts
}

// resolve resolves one of d's open conflicts by choosing one present
// competing version, or the current bytes, as the user would in a picker.
// choice < 0 is seeded.
func (s *faultSim) resolve(d *simDevice, conflict synclog.Conflict, choice int) {
	var options [][]byte
	for _, candidate := range conflict.Candidates {
		if candidate.Blob == nil {
			continue
		}
		data, err := d.history.Blob(*candidate.Blob)
		if err != nil {
			s.fatalf("%s: competing version %s of %s is not recoverable: %v", d.name, *candidate.Blob, conflict.Key, err)
		}
		options = append(options, data)
	}
	if current, err := os.ReadFile(d.path(conflict.Path)); err == nil {
		options = append(options, current)
	}
	if len(options) == 0 {
		return
	}
	if choice < 0 || choice >= len(options) {
		choice = s.rng.Intn(len(options))
	}
	s.commitResolution(d, conflict, options[choice], fmt.Sprintf("option %d of %d", choice, len(options)))
}

// resolveRetitling resolves a conflict on a project document by writing the
// current document with one task given a fresh title: a resolution that is
// none of the competing versions.
func (s *faultSim) resolveRetitling(d *simDevice, conflict synclog.Conflict, slug, taskID string) {
	doc := s.doc(d, slug)
	task := findDocTask(doc, taskID)
	if task == nil {
		s.fatalf("%s: task %s is not in %s", d.name, taskID, slug)
	}
	marker := s.marker(d)
	task.Title = "Resolved " + marker
	content, err := doc.WriteCurrent()
	if err != nil {
		s.fatalf("%s: render %s: %v", d.name, slug, err)
	}
	if txn := s.commitResolution(d, conflict, []byte(content), "retitle "+marker); txn != "" {
		s.record(txn, d, conflict.Path, "task:"+taskID, marker, "title")
	}
}

// commitResolution commits one resolution, possibly crashing, and returns
// its transaction, or "" when the crash lost it.
func (s *faultSim) commitResolution(d *simDevice, conflict synclog.Conflict, data []byte, how string) string {
	var txn string
	crashed := s.withCrash(d, "resolve "+short(conflict.ID), func() error {
		_, err := d.svc.CommitSyncResolution(conflict.ID, data)
		return err
	})
	if state, err := d.history.LoadState(); err == nil && state != nil && len(state.Heads) == 1 {
		if m, txErr := d.history.Transaction(state.Heads[0]); txErr == nil && m.Kind == synclog.KindResolution && m.DeviceID == d.name {
			txn = m.TxnID
		}
	}
	if txn == "" {
		if !crashed {
			s.fatalf("%s: resolving %s recorded no resolution", d.name, short(conflict.ID))
		}
		s.logf("%s lost a resolution of %s to the crash", d.name, short(conflict.ID))
		return ""
	}
	s.count("resolutions")
	s.logf("%s resolve %s at %s with %s as %s", d.name, short(conflict.ID), conflict.Path, how, short(txn))
	s.record(txn, d, conflict.Path, "", "", "resolution")
	return txn
}

// resolveRandom resolves a seeded open conflict on d, if it has one.
func (s *faultSim) resolveRandom(d *simDevice) bool {
	open := s.conflicts(d)
	if len(open) == 0 {
		return false
	}
	conflict := open[s.rng.Intn(len(open))]
	if conflict.Kind == synclog.EffectCatalog {
		return false
	}
	s.resolve(d, conflict, -1)
	return true
}

// --- driving runs ---

// randomStep is one seeded event: a local edit, a publication, a delivery
// round, a restart, or a device going offline or online.
func (s *faultSim) randomStep() {
	s.tick++
	authors := s.authors()
	d := authors[s.rng.Intn(len(authors))]
	if s.chance(s.cfg.Restart) {
		s.restart(d, "relaunch")
	}
	if s.chance(s.cfg.Offline) {
		d.offline = !d.offline
		s.count("offline_toggles")
		s.logf("%s offline=%v", d.name, d.offline)
	}
	switch op := s.rng.Intn(14); op {
	case 0, 1:
		s.retitleRandom(d)
	case 2:
		s.capture(d)
	case 3:
		s.deleteLeaf(d)
	case 4:
		s.reprioritize(d)
	case 5:
		s.plan(d)
	case 6:
		s.appendNote(d)
	case 7:
		if !s.resolveRandom(d) {
			s.retitleRandom(d)
		}
	case 8, 9, 10:
		s.publish(d)
	default:
		s.drain(d)
	}
}

// settle brings every device online and exchanges until every outbox and
// inbox is empty, crashes and transport faults included.
func (s *faultSim) settle() {
	s.logf("settle")
	for _, d := range s.devices {
		d.offline = false
	}
	for round := 0; ; round++ {
		if round > 400 {
			s.fatalf("the exchange did not settle")
		}
		s.tick++
		busy := false
		for _, d := range s.devices {
			s.publish(d)
		}
		for _, d := range s.devices {
			s.drain(d)
		}
		for _, d := range s.devices {
			pending, err := d.history.Pending()
			if err != nil {
				s.fatalf("%s: pending: %v", d.name, err)
			}
			if len(d.inbox) > 0 || len(pending) > 0 {
				busy = true
			}
		}
		if !busy {
			return
		}
	}
}

// addObservers clones devices that start from genesis and receive the whole
// exchange through the same faulty transport, each in its own seeded order.
func (s *faultSim) addObservers(n int) {
	for i := 0; i < n; i++ {
		o := s.addDevice(fmt.Sprintf("observer-%d", i), true)
		order := append([]string{}, s.ex.order...)
		if i%2 == 1 {
			for a, b := 0, len(order)-1; a < b; a, b = a+1, b-1 {
				order[a], order[b] = order[b], order[a]
			}
		} else {
			s.rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
		}
		for _, txnID := range order {
			s.enqueue(o, s.decode(txnID), s.ex.manifests[txnID])
		}
		s.logf("observer-%d joins and receives %d transactions", i, len(order))
	}
}

func (s *faultSim) decode(txnID string) *synclog.Manifest {
	m, err := synclog.DecodeManifest(s.ex.manifests[txnID])
	if err != nil {
		s.fatalf("exchange holds an invalid manifest %s: %v", short(txnID), err)
	}
	return m
}

// finish settles, adds genesis observers, settles again, and holds every
// replica to convergence, preservation, and idempotence. It returns the
// conflicts every replica agrees on.
func (s *faultSim) finish(observers int) []synclog.Conflict {
	s.settle()
	s.addObservers(observers)
	s.settle()
	conflicts := s.requireConverged()
	s.requirePreserved(conflicts)
	s.requireIdempotent()
	for _, d := range s.devices {
		s.restart(d, "final relaunch")
		report, err := d.svc.CommitExternalChanges()
		if err != nil {
			s.fatalf("%s: observe after relaunch: %v", d.name, err)
		}
		if report.Imported != nil || len(report.Held) > 0 {
			s.fatalf("%s: the converged workspace holds changes its history never recorded: %+v", d.name, report)
		}
	}
	s.logf("finished: %d transactions, %d open conflicts; %s", len(s.ex.order), len(conflicts), s.summary())
	return conflicts
}

// --- invariants ---

// syncedPath reports a path synclog.Classify names and does not call
// device-local; an unclassifiable path is not synced.
func syncedPath(rel string) bool {
	_, _, local, err := synclog.Classify(rel)
	return err == nil && !local
}

func (s *faultSim) syncedFiles(d *simDevice) map[string]string {
	out := map[string]string{}
	err := filepath.WalkDir(d.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(d.root, path)
		rel = filepath.ToSlash(rel)
		if !syncedPath(rel) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out[rel] = synclog.Digest(data)
		return nil
	})
	if err != nil {
		s.fatalf("%s: walk: %v", d.name, err)
	}
	return out
}

// requireConverged holds every replica, authors and genesis observers alike,
// to identical synced bytes, identical derived conflicts, a workspace every
// live reader accepts, and competing versions every replica can recover.
func (s *faultSim) requireConverged() []synclog.Conflict {
	first := s.devices[0]
	want := s.syncedFiles(first)
	wantConflicts := s.conflictsJSON(first)
	for _, d := range s.devices[1:] {
		got := s.syncedFiles(d)
		for _, rel := range sortedKeys(want) {
			if got[rel] != want[rel] {
				s.fatalf("%s and %s disagree on %s:\n%s\n---\n%s", first.name, d.name, rel, readOrErr(first, rel), readOrErr(d, rel))
			}
		}
		for _, rel := range sortedKeys(got) {
			if _, ok := want[rel]; !ok {
				s.fatalf("%s holds %s and %s does not", d.name, rel, first.name)
			}
		}
		if gotConflicts := s.conflictsJSON(d); gotConflicts != wantConflicts {
			s.fatalf("%s and %s derive different conflicts:\n%s\n---\n%s", first.name, d.name, wantConflicts, gotConflicts)
		}
	}
	for _, d := range s.devices {
		s.requireValidWorkspace(d)
	}
	conflicts := s.conflicts(first)
	for _, conflict := range conflicts {
		for _, candidate := range conflict.Candidates {
			if candidate.Blob == nil {
				continue
			}
			for _, d := range s.devices {
				if _, err := d.history.Blob(*candidate.Blob); err != nil {
					s.fatalf("%s cannot recover competing version %s of %s: %v", d.name, *candidate.Blob, conflict.Key, err)
				}
			}
		}
	}
	s.logf("converged: %d replicas, %d synced files, %d open conflicts", len(s.devices), len(want), len(conflicts))
	return conflicts
}

func readOrErr(d *simDevice, rel string) string {
	data, err := os.ReadFile(d.path(rel))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (s *faultSim) conflictsJSON(d *simDevice) string {
	data, err := json.MarshalIndent(s.conflicts(d), "", "  ")
	if err != nil {
		s.fatalf("encode conflicts: %v", err)
	}
	return string(data)
}

// requireValidWorkspace reads a replica through the live readers.
func (s *faultSim) requireValidWorkspace(d *simDevice) {
	var env faultEnvelope
	if err := json.Unmarshal(s.read(d, ".tt/workspace.json"), &env); err != nil {
		s.fatalf("%s: catalog envelope: %v", d.name, err)
	}
	catalog := &config.WorkspaceCatalog{}
	if err := json.Unmarshal(env.Data, catalog); err != nil {
		s.fatalf("%s: catalog: %v", d.name, err)
	}
	if err := catalog.Validate(); err != nil {
		s.fatalf("%s: catalog invalid: %v", d.name, err)
	}
	if err := json.Unmarshal(s.read(d, ".tt/planning.json"), &env); err != nil {
		s.fatalf("%s: planning envelope: %v", d.name, err)
	}
	planning := &config.PlanningState{}
	if err := json.Unmarshal(env.Data, planning); err != nil {
		s.fatalf("%s: planning: %v", d.name, err)
	}
	if err := planning.Validate(); err != nil {
		s.fatalf("%s: planning invalid: %v", d.name, err)
	}
	for _, project := range catalog.Projects {
		rel := "projects/" + project.Slug + "/tasks.md"
		if _, err := os.Stat(d.path(rel)); err != nil {
			continue
		}
		doc := s.doc(d, project.Slug)
		if err := models.AuditProject(&models.Project{Name: doc.ProjectName, Tasks: doc.Tasks, Comments: doc.Comments}); err != nil {
			s.fatalf("%s: %s fails the audit: %v", d.name, rel, err)
		}
		shortIDs := map[int]string{}
		for _, task := range doc.Tasks {
			if other, clash := shortIDs[task.ShortID]; clash {
				s.fatalf("%s: %s gives short id %d to %s and %s", d.name, rel, task.ShortID, other, task.ID)
			}
			shortIDs[task.ShortID] = task.ID
		}
	}
}

// ancestry is every authored transaction's strict ancestors, read from the
// exchange.
func (s *faultSim) ancestry() map[string]map[string]bool {
	parents := map[string][]string{}
	for _, txnID := range s.ex.order {
		parents[txnID] = s.decode(txnID).Parents
	}
	memo := map[string]map[string]bool{}
	var visit func(string) map[string]bool
	visit = func(id string) map[string]bool {
		if out, ok := memo[id]; ok {
			return out
		}
		out := map[string]bool{}
		for _, parent := range parents[id] {
			out[parent] = true
			for ancestor := range visit(parent) {
				out[ancestor] = true
			}
		}
		memo[id] = out
		return out
	}
	for _, txnID := range s.ex.order {
		visit(txnID)
	}
	return memo
}

// requirePreserved holds the converged workspace to the user's work: every
// marker a user wrote that no later edit of theirs replaced — a later title
// for the same task, a deletion of it, or a resolution of the file it sits in,
// each made after seeing it — is still in the workspace or in a competing
// version of a conflict that is still open.
func (s *faultSim) requirePreserved(conflicts []synclog.Conflict) {
	ancestors := s.ancestry()
	d := s.devices[0]
	var corpus strings.Builder
	files := s.syncedFiles(d)
	for _, rel := range sortedKeys(files) {
		corpus.Write(s.read(d, rel))
	}
	workspace := corpus.String()
	var competing strings.Builder
	for _, conflict := range conflicts {
		for _, candidate := range conflict.Candidates {
			if candidate.Blob == nil {
				continue
			}
			data, err := d.history.Blob(*candidate.Blob)
			if err != nil {
				s.fatalf("competing version %s is not recoverable: %v", *candidate.Blob, err)
			}
			competing.Write(data)
		}
	}
	live, kept := 0, 0
	for _, w := range s.writes {
		if w.marker == "" {
			continue
		}
		if _, published := s.ex.position[w.txn]; !published {
			s.fatalf("%s's write %s in %s was never published", w.device, w.marker, short(w.txn))
		}
		if s.superseded(w, ancestors) {
			continue
		}
		live++
		switch {
		case strings.Contains(workspace, w.marker):
			kept++
		case strings.Contains(competing.String(), w.marker):
			s.count("preserved_in_conflict")
		default:
			s.fatalf("%s's %s %s (%s in %s) is in neither the workspace nor any open conflict", w.device, w.kind, w.marker, w.subject, short(w.txn))
		}
	}
	s.stats["live_user_writes"] += live
	s.logf("preserved: %d live user writes, %d in the workspace", live, kept)
}

func (s *faultSim) superseded(w userWrite, ancestors map[string]map[string]bool) bool {
	for _, later := range s.writes {
		if !ancestors[later.txn][w.txn] {
			continue
		}
		switch {
		case later.kind == "resolution" && later.path == w.path:
			return true
		case w.kind == "title" && later.subject == w.subject && (later.kind == "title" || later.kind == "delete"):
			return true
		}
	}
	return false
}

// requireIdempotent redelivers the whole exchange to every replica, damaged,
// duplicated and reordered, and requires that nothing changes anywhere.
func (s *faultSim) requireIdempotent() {
	before := map[string]string{}
	for _, d := range s.devices {
		before[d.name] = s.fingerprint(d)
		for _, txnID := range s.ex.order {
			s.enqueue(d, s.decode(txnID), s.ex.manifests[txnID])
		}
	}
	s.logf("redeliver everything to every replica")
	s.settle()
	for _, d := range s.devices {
		if after := s.fingerprint(d); after != before[d.name] {
			s.fatalf("%s: redelivering the exchange changed the replica:\n%s\n---\n%s", d.name, before[d.name], after)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func copyFaultTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", from, err)
	}
}
