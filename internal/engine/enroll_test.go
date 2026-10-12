package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/synclog"
)

// enrollEnvironment is one desktop about to enroll its workspace: the
// original, the paths enrollment will create, a process home for the
// machine-local pointer and locks, and a deterministic clock.
type enrollEnvironment struct {
	root        string
	home        string
	original    string
	workingCopy string
	exchange    string
	snapshots   string
	clock       *headlessClock
	ids         *headlessIDs
}

func newEnrollEnvironment(t *testing.T) enrollEnvironment {
	t.Helper()
	root := t.TempDir()
	env := enrollEnvironment{
		root:        root,
		home:        filepath.Join(root, "home"),
		original:    filepath.Join(root, "original"),
		workingCopy: filepath.Join(root, "local", "tt-synced"),
		exchange:    filepath.Join(root, "cloud", "exchange"),
		snapshots:   filepath.Join(root, "snapshots"),
		clock:       &headlessClock{now: time.Date(2026, time.October, 6, 10, 0, 0, 0, time.UTC)},
		ids:         &headlessIDs{},
	}
	t.Setenv("HOME", env.home)
	if err := os.MkdirAll(env.exchange, 0o700); err != nil {
		t.Fatal(err)
	}
	original := openHeadless(t, Config{
		DataDir:         env.original,
		Machine:         ExplicitMachine(env.home, "desk-a"),
		Clock:           env.clock,
		IDs:             env.ids,
		CreateIfMissing: true,
	})
	unfiled := original.State().SystemUnfiledProject()
	if _, err := original.Service().Capture(app.CaptureRequest{ProjectID: unfiled.ID, Title: "Written before enrollment"}); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	return env
}

// The enrolling desktop and the device that joins it, as the environment
// names them.
const (
	enrollingDevice = "desk-a"
	joiningDevice   = "desk-b"
)

func (env enrollEnvironment) enrollConfig() EnrollConfig {
	return EnrollConfig{
		Original:     env.original,
		WorkingCopy:  env.workingCopy,
		Exchange:     env.exchange,
		SnapshotRoot: env.snapshots,
		Machine:      ExplicitMachine(env.home, enrollingDevice),
		Clock:        env.clock,
		AppVersion:   "test",
	}
}

func (env enrollEnvironment) enroll(t *testing.T) *EnrollResult {
	t.Helper()
	result, err := Enroll(env.enrollConfig())
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	return result
}

func (env enrollEnvironment) open(t *testing.T, dataDir, device string) *Runtime {
	t.Helper()
	return openHeadless(t, Config{DataDir: dataDir, Machine: ExplicitMachine(env.home, device), Clock: env.clock, IDs: env.ids})
}

func (env enrollEnvironment) join(t *testing.T) *Runtime {
	t.Helper()
	device := joiningDevice
	rt, report, err := Join(JoinConfig{
		DataDir:  filepath.Join(env.root, "local", "joined-"+device),
		Exchange: env.exchange,
		Machine:  ExplicitMachine(filepath.Join(env.root, "home-"+device), device),
		Clock:    env.clock,
	})
	if err != nil {
		t.Fatalf("Join(%s): %v (report %+v)", device, err, report)
	}
	return rt
}

// treeDigests maps every regular file under dir to its digest.
func treeDigests(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(dir, path)
		out[filepath.ToSlash(rel)] = synclog.Digest(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return out
}

// syncedDigests is treeDigests restricted to the files sync exchanges.
func syncedDigests(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for rel, digest := range treeDigests(t, dir) {
		if _, _, local, err := synclog.Classify(rel); err == nil && !local {
			out[rel] = digest
		}
	}
	return out
}

func requireSameTree(t *testing.T, label string, want, got map[string]string) {
	t.Helper()
	var diffs []string
	for rel, digest := range want {
		if got[rel] != digest {
			diffs = append(diffs, "changed or missing "+rel)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			diffs = append(diffs, "added "+rel)
		}
	}
	sort.Strings(diffs)
	if len(diffs) > 0 {
		t.Fatalf("%s: %s", label, strings.Join(diffs, "; "))
	}
}

func exchangeManifests(t *testing.T, exchange, device string) []*synclog.Manifest {
	t.Helper()
	dir := filepath.Join(exchange, "tt-sync", "v1", "txns", device)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}
	var out []*synclog.Manifest
	for _, entry := range entries {
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		manifest, decodeErr := synclog.DecodeManifest(data)
		if decodeErr != nil {
			t.Fatalf("decoding %s: %v", entry.Name(), decodeErr)
		}
		out = append(out, manifest)
	}
	return out
}

func captureIn(t *testing.T, rt *Runtime, title string) {
	t.Helper()
	unfiled := rt.State().SystemUnfiledProject()
	if _, err := rt.Service().Capture(app.CaptureRequest{ProjectID: unfiled.ID, Title: title}); err != nil {
		t.Fatalf("Capture(%q): %v", title, err)
	}
}

func exchangeNow(t *testing.T, rt *Runtime) *syncexchange.FolderReport {
	t.Helper()
	report, err := rt.SyncExchange(SyncExchangeOptions{AppVersion: "test"})
	if err != nil {
		t.Fatalf("SyncExchange: %v (report %+v)", err, report)
	}
	return report
}

// TestSyncEnrollment drives desktop enrollment end to end through the shared
// runtime: the snapshot and the local working copy, the initial history and
// its publication, a second device rebuilt from that history alone, restarts,
// interruptions, external edits, and the behavior of workspaces never
// enrolled.
func TestSyncEnrollment(t *testing.T) {
	t.Run("enrolls_from_a_snapshot_into_a_local_working_copy", enrollmentEnrollsFromASnapshotIntoALocalWorkingCopy)

	t.Run("workspaces_never_enrolled_keep_their_behavior", enrollmentWorkspacesNeverEnrolledKeepTheirBehavior)

	t.Run("a_second_device_joins_from_the_published_history", enrollmentASecondDeviceJoinsFromThePublishedHistory)

	t.Run("restart_keeps_history_and_exchanges_new_work", enrollmentRestartKeepsHistoryAndExchangesNewWork)

	t.Run("an_interrupted_enrollment_resumes", enrollmentAnInterruptedEnrollmentResumes)

	t.Run("external_edits_are_published_and_invalid_ones_held", enrollmentExternalEditsArePublishedAndInvalidOnesHeld)

	t.Run("an_incompatible_peer_holds_and_names_the_upgrade", enrollmentAnIncompatiblePeerHoldsAndNamesTheUpgrade)
}

func enrollmentEnrollsFromASnapshotIntoALocalWorkingCopy(t *testing.T) {
	env := newEnrollEnvironment(t)
	before := treeDigests(t, env.original)
	result := env.enroll(t)

	requireSameTree(t, "the original after enrollment", before, treeDigests(t, env.original))

	snapshot := result.Enrollment.Snapshot
	if !strings.HasPrefix(snapshot, env.snapshots) {
		t.Fatalf("snapshot %s is not under the snapshot root %s", snapshot, env.snapshots)
	}
	snapshotTree := treeDigests(t, snapshot)
	delete(snapshotTree, "snapshot.json")
	durable := map[string]string{}
	for rel, digest := range before {
		// The commit lock is the one file a copy never carries.
		if rel != ".tt/transactions/commit.lock" {
			durable[rel] = digest
		}
	}
	requireSameTree(t, "the snapshot", durable, snapshotTree)

	catalog, err := config.NewDomainFileStore(env.workingCopy).LoadCatalogReadOnly()
	if err != nil {
		t.Fatalf("working copy catalog: %v", err)
	}
	if catalog.SchemaVersion != config.SyncEnrolledConfigVersion || !catalog.Enrolled() {
		t.Fatalf("working copy catalog is schema %d enrolled=%v, want the enrolled schema", catalog.SchemaVersion, catalog.Enrolled())
	}
	for _, local := range []string{".tt/undo", ".tt/backups"} {
		if _, statErr := os.Stat(filepath.Join(env.workingCopy, local)); statErr == nil {
			t.Fatalf("working copy carried the original's device-local %s", local)
		}
	}
	if pointer, ok := config.ReadActiveWorkspacePointer(); !ok || pointer != env.workingCopy {
		t.Fatalf("active workspace is %q, want the working copy %s", pointer, env.workingCopy)
	}
	if config.InICloudDrive(env.workingCopy, env.home) {
		t.Fatal("the working copy landed in iCloud Drive")
	}

	published := exchangeManifests(t, env.exchange, "desk-a")
	if len(published) != 1 || published[0].Kind != synclog.KindEnroll || published[0].DeviceSeq != 1 || len(published[0].Parents) != 0 {
		t.Fatalf("exchange holds %+v, want exactly the genesis", published)
	}
	if published[0].TxnID != result.Genesis {
		t.Fatalf("published %s, enrollment reported genesis %s", published[0].TxnID, result.Genesis)
	}
	// The genesis alone rebuilds every synced file of the working copy.
	rebuilt := map[string]string{}
	for _, effect := range published[0].Effects {
		if effect.Op != synclog.OpPut || effect.BaseBlob != nil {
			t.Fatalf("genesis effect %+v is not a creation", effect)
		}
		blob, readErr := os.ReadFile(filepath.Join(env.exchange, "tt-sync", "v1", filepath.FromSlash(syncexchange.BlobPath(*effect.Blob))))
		if readErr != nil || synclog.Digest(blob) != *effect.Blob {
			t.Fatalf("genesis blob for %s is not published intact: %v", effect.Path, readErr)
		}
		rebuilt[effect.Path] = *effect.Blob
	}
	requireSameTree(t, "the genesis", syncedDigests(t, env.workingCopy), rebuilt)
	if published[0].Schemas[synclog.SchemaCatalog] != config.SyncEnrolledConfigVersion {
		t.Fatalf("genesis records catalog schema %d", published[0].Schemas[synclog.SchemaCatalog])
	}

	rt := env.open(t, env.workingCopy, "desk-a")
	status, err := rt.SyncStatus()
	if err != nil {
		t.Fatalf("SyncStatus: %v", err)
	}
	if !status.Enrolled || !status.Genesis || status.OutstandingUp != 0 || status.LastSuccess.IsZero() ||
		status.Enrollment == nil || status.Enrollment.ExchangeFolder != env.exchange {
		t.Fatalf("status after enrollment = %+v", status)
	}
}

func enrollmentWorkspacesNeverEnrolledKeepTheirBehavior(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	original := env.open(t, env.original, "desk-a")
	if schema := original.State().Catalog().SchemaVersion; schema != config.CurrentConfigVersion {
		t.Fatalf("the original moved to schema %d", schema)
	}
	if got := original.repo.svc.SyncHistory().Schemas()[synclog.SchemaCatalog]; got != config.CurrentConfigVersion {
		t.Fatalf("a non-enrolled history records catalog schema %d", got)
	}
	captureIn(t, original, "Still works un-enrolled")
	if _, err := original.SyncExchange(SyncExchangeOptions{Folder: env.exchange}); !errors.Is(err, ErrNotEnrolled) {
		t.Fatalf("exchange on a non-enrolled workspace = %v, want ErrNotEnrolled", err)
	}
	status, err := original.SyncStatus()
	if err != nil || status.Enrolled {
		t.Fatalf("status of a non-enrolled workspace = %+v, %v", status, err)
	}
	if _, err := Enroll(EnrollConfig{
		Original: env.workingCopy, WorkingCopy: filepath.Join(env.root, "again"), Exchange: env.exchange,
		SnapshotRoot: env.snapshots, Machine: ExplicitMachine(env.home, "desk-a"), Clock: env.clock,
	}); !errors.Is(err, config.ErrAlreadyEnrolled) {
		t.Fatalf("enrolling a working copy = %v, want ErrAlreadyEnrolled", err)
	}
	if len(exchangeManifests(t, env.exchange, "desk-a")) != 1 {
		t.Fatal("the un-enrolled original published to the exchange")
	}
}

func enrollmentASecondDeviceJoinsFromThePublishedHistory(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	phone := env.join(t)
	requireSameTree(t, "the joined working copy", syncedDigests(t, env.workingCopy), syncedDigests(t, phone.DataDir()))
	if !phone.State().Catalog().Enrolled() {
		t.Fatal("the joined working copy is not enrolled")
	}
	if len(exchangeManifests(t, env.exchange, "desk-b")) != 0 {
		t.Fatal("joining published a transaction of its own")
	}
}

func enrollmentRestartKeepsHistoryAndExchangesNewWork(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, "desk-a")
	laptop := env.join(t)
	captureIn(t, desk, "Captured on the desk after a restart")
	captureIn(t, laptop, "Captured on the laptop")

	// Restart both: a fresh runtime over each working copy records no
	// second genesis and keeps the outbox it had.
	desk = env.open(t, env.workingCopy, "desk-a")
	laptop = env.open(t, laptop.DataDir(), "desk-b")
	if status, _ := desk.SyncStatus(); status.OutstandingUp != 1 || status.LocalTransactions != 2 {
		t.Fatalf("desk status after restart = %+v, want genesis plus one outstanding capture", status)
	}
	exchangeNow(t, desk)
	exchangeNow(t, laptop)
	if report := exchangeNow(t, desk); report.Imported != 1 {
		t.Fatalf("desk imported %d, want the laptop's capture", report.Imported)
	}
	requireSameTree(t, "after exchanging both ways", syncedDigests(t, env.workingCopy), syncedDigests(t, laptop.DataDir()))
	if genesis := exchangeManifests(t, env.exchange, "desk-a"); len(genesis) != 2 || genesis[0].Kind != synclog.KindEnroll {
		t.Fatalf("desk published %d transactions, want genesis and one capture", len(genesis))
	}
}

func enrollmentAnInterruptedEnrollmentResumes(t *testing.T) {
	for _, stage := range []string{"genesis", "exchange"} {
		t.Run(stage, func(t *testing.T) {
			env := newEnrollEnvironment(t)
			before := treeDigests(t, env.original)
			interrupted := errors.New("interrupted at " + stage)
			enrollHook = func(s string) error {
				if s == stage {
					return interrupted
				}
				return nil
			}
			_, err := Enroll(env.enrollConfig())
			enrollHook = nil
			if !errors.Is(err, interrupted) {
				t.Fatalf("Enroll = %v, want the interruption", err)
			}
			result, err := Enroll(env.enrollConfig())
			if err != nil {
				t.Fatalf("resumed Enroll: %v", err)
			}
			if !result.Enrollment.Resumed {
				t.Fatal("the second run copied the working copy again")
			}
			requireSameTree(t, "the original", before, treeDigests(t, env.original))
			published := exchangeManifests(t, env.exchange, "desk-a")
			if len(published) != 1 || published[0].Kind != synclog.KindEnroll {
				t.Fatalf("exchange after resuming holds %d transactions, want the one genesis", len(published))
			}
			requireSameTree(t, "a device joining the resumed enrollment",
				syncedDigests(t, env.workingCopy), syncedDigests(t, env.join(t).DataDir()))
		})
	}
}

func enrollmentExternalEditsArePublishedAndInvalidOnesHeld(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, "desk-a")
	laptop := env.join(t)
	document := filepath.Join(env.workingCopy, "projects", desk.State().SystemUnfiledProject().Slug, "tasks.md")
	data, err := os.ReadFile(document)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(data, []byte("Written before enrollment"), []byte("Retitled in an editor"), 1)
	if bytes.Equal(edited, data) {
		t.Fatal("the seeded title is not in the document")
	}
	if err := os.WriteFile(document, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	report := exchangeNow(t, desk)
	if report.External == nil || report.External.Imported == nil || len(report.Published) != 1 {
		t.Fatalf("the editor's change was not published: %+v", report)
	}
	exchangeNow(t, laptop)
	requireSameTree(t, "after an external edit", syncedDigests(t, env.workingCopy), syncedDigests(t, laptop.DataDir()))

	planning := filepath.Join(env.workingCopy, ".tt", "planning.json")
	if err := os.WriteFile(planning, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	report = exchangeNow(t, desk)
	if report.External == nil || len(report.External.Held) != 1 || len(report.Published) != 0 {
		t.Fatalf("an invalid edit was not held: %+v", report.External)
	}
	if got, _ := os.ReadFile(planning); string(got) != "{ not json" {
		t.Fatal("holding the invalid edit rewrote it")
	}
}

func enrollmentAnIncompatiblePeerHoldsAndNamesTheUpgrade(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, "desk-a")
	laptop := env.join(t)

	// The desk upgrades to a future catalog schema. The laptop has not.
	future := LiveSchemas()
	future[synclog.SchemaCatalog] = config.SupportedConfigVersion + 1
	desk.repo.svc.WithSyncHistory(syncHistory(desk.DataDir(), "desk-a", desk.clock, future))
	captureIn(t, desk, "Written by the upgraded desk")
	exchangeNow(t, desk)

	historyDir := filepath.Join(laptop.DataDir(), ".tt", "sync")
	recordsBefore := treeDigests(t, historyDir)
	captureIn(t, laptop, "Written by the laptop meanwhile")
	report := exchangeNow(t, laptop)
	if len(report.Held) != 1 || !strings.Contains(report.Held[0].Upgrade, "update tt on this device") ||
		!strings.Contains(report.Held[0].Upgrade, fmt.Sprintf("catalog schema %d", config.SupportedConfigVersion+1)) {
		t.Fatalf("the laptop did not hold the newer transaction with its upgrade: %+v", report.Held)
	}
	if len(report.Published) != 1 {
		t.Fatalf("the laptop's own pending work was not published while incompatible: %+v", report.Published)
	}
	status, err := laptop.SyncStatus()
	if err != nil || len(status.Upgrades) == 0 || status.OutstandingDown != 1 {
		t.Fatalf("laptop status does not report the required upgrade: %+v, %v", status, err)
	}
	// Holding kept the inbox copy and rewrote no historical record.
	after := treeDigests(t, historyDir)
	for rel, digest := range recordsBefore {
		// The tracking state, the transport's files, and the observation stat
		// cache are mutable device-local state, not records.
		if strings.HasPrefix(rel, "devices/desk-b/state.json") || strings.HasPrefix(rel, "transport/") || rel == "stat-cache.json" {
			continue
		}
		if after[rel] != digest {
			t.Fatalf("historical record %s changed while incompatible", rel)
		}
	}

	// The laptop upgrades: the held transaction now applies, its own
	// earlier records are untouched, and the desk applies the work the
	// laptop wrote at the older schema.
	laptop.repo.svc.WithSyncHistory(syncHistory(laptop.DataDir(), "desk-b", laptop.clock, future))
	upgraded := treeDigests(t, historyDir)
	if report := exchangeNow(t, laptop); report.Imported != 1 || len(report.Held) != 0 {
		t.Fatalf("after upgrading the laptop imported %d and holds %+v", report.Imported, report.Held)
	}
	final := treeDigests(t, historyDir)
	for rel, digest := range upgraded {
		if strings.HasPrefix(rel, "transport/") {
			continue
		}
		if strings.Contains(rel, "/txns/") || strings.Contains(rel, "/incoming/") || strings.HasPrefix(rel, "index/") {
			if final[rel] != digest {
				t.Fatalf("upgrading rewrote the immutable record %s", rel)
			}
		}
	}
	if report := exchangeNow(t, desk); report.Imported != 1 || len(report.Held) != 0 {
		t.Fatalf("the upgraded desk imported %d of the laptop's earlier work, holding %+v", report.Imported, report.Held)
	}
	requireSameTree(t, "after both upgraded", syncedDigests(t, env.workingCopy), syncedDigests(t, laptop.DataDir()))
}

// TestSyncEnrollmentRefusesUnsafePlacement holds enrollment to the placement
// rules before anything is written.
func TestSyncEnrollmentRefusesUnsafePlacement(t *testing.T) {
	env := newEnrollEnvironment(t)
	icloud := filepath.Join(env.home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	if err := os.MkdirAll(icloud, 0o700); err != nil {
		t.Fatal(err)
	}
	occupied := filepath.Join(env.root, "occupied")
	if err := os.MkdirAll(occupied, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "keep.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := treeDigests(t, env.original)
	for name, mutate := range map[string]func(*EnrollConfig){
		"working_copy_in_icloud":    func(c *EnrollConfig) { c.WorkingCopy = filepath.Join(icloud, "tt") },
		"working_copy_in_original":  func(c *EnrollConfig) { c.WorkingCopy = filepath.Join(env.original, "copy") },
		"exchange_in_working_copy":  func(c *EnrollConfig) { c.Exchange = filepath.Join(env.workingCopy, "exchange") },
		"exchange_in_original":      func(c *EnrollConfig) { c.Exchange = filepath.Join(env.original, "exchange") },
		"working_copy_not_empty":    func(c *EnrollConfig) { c.WorkingCopy = occupied },
		"original_is_not_workspace": func(c *EnrollConfig) { c.Original = occupied },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := env.enrollConfig()
			mutate(&cfg)
			if _, err := Enroll(cfg); err == nil {
				t.Fatal("enrollment was accepted")
			}
		})
	}
	requireSameTree(t, "the original after refusals", before, treeDigests(t, env.original))
	if _, err := os.Stat(env.workingCopy); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused enrollment created the working copy: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(occupied, "keep.txt")); err != nil || string(data) != "mine" {
		t.Fatal("a refused enrollment touched an occupied directory")
	}
}
