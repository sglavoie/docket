package engine

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
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
