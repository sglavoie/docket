package storage_test

// The sync fault harness: a deterministic multi-replica simulation of devices
// that edit offline, publish immutable transactions to a shared exchange, and
// receive them through a faulty transport, with controlled clocks, minted
// identities, crashes and restarts. Every random choice comes from one seeded
// source and every event is appended to a trace, so a failure names its seed,
// the command that replays it, and the events that led to it.

import (
	"encoding/json"
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

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/syncmerge"
)

// faultFixtureDir is the representative compatibility workspace every replica
// starts from.
const faultFixtureDir = "../compat/testdata/workspaces/representative"

const (
	faultAtlasID  = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"
	faultNotePath = "projects/atlas-launch/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md"
	// faultSeedEnv replays one seed of a seeded suite.
	faultSeedEnv = "DOCKET_SYNC_FAULT_SEED"
	// faultTraceEnv names the directory failure traces are written to.
	faultTraceEnv = "DOCKET_SYNC_FAULT_TRACE_DIR"
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
	ledger := storage.NewWorkspaceLedger(filepath.Join(d.root, ".docket", "undo"), []string{d.root})
	d.history = history
	d.svc = storage.NewCommitServiceWithLedger(d.txDir(), []string{d.root}, ledger).
		WithSyncHistory(history).
		WithSyncMerger(syncmerge.Merger{})
}

func (d *simDevice) txDir() string { return filepath.Join(d.root, ".docket", "transactions") }

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
