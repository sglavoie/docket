package cmd

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/engine"
)

const atlasTasksPath = "projects/atlas-launch/tasks.md"

// copyFixtureTree copies the representative workspace into dir.
func copyFixtureTree(t *testing.T, dir string) {
	t.Helper()
	source := filepath.Join("..", "internal", "compat", "testdata", "workspaces", "representative")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(source, path)
		target := filepath.Join(dir, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// retitleOnDisk edits a task heading the way an external editor would; the
// next observation publishes it.
func retitleOnDisk(t *testing.T, dataDir, from, to string) {
	t.Helper()
	path := filepath.Join(dataDir, filepath.FromSlash(atlasTasksPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "## Task: "+from+"\n", "## Task: "+to+"\n", 1)
	if edited == string(data) {
		t.Fatalf("%s holds no task %q", path, from)
	}
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestSyncConflictCommands resolves a sync conflict from the desktop through
// `tt sync conflicts` and `tt sync resolve`: the conflict between this
// desktop and a second device is shown with its base, both versions, and who
// wrote each; diagnostics export its structure without content; the
// resolution travels and both replicas converge; and a resolution of a
// conflict that is no longer open is refused.
func TestSyncConflictCommands(t *testing.T) {
	fixture := newSyncCommandFixture(t)
	copyFixtureTree(t, fixture.original)
	fixture.enroll(t)
	setCommandDataDir(t, fixture.workingCopy)
	previousVersion, previousCurrent, previousFile, previousOut := syncResolveVersion, syncResolveCurrent, syncResolveFile, syncDiagnosticsOut
	t.Cleanup(func() {
		syncResolveVersion, syncResolveCurrent, syncResolveFile, syncDiagnosticsOut = previousVersion, previousCurrent, previousFile, previousOut
	})

	laptopDir := filepath.Join(t.TempDir(), "laptop")
	laptop, _, err := engine.Join(engine.JoinConfig{
		DataDir:  laptopDir,
		Exchange: fixture.exchange,
		Machine:  engine.ExplicitMachine(t.TempDir(), "laptop-b"),
	})
	if err != nil {
		t.Fatalf("joining a second device: %v", err)
	}
	setCommandDataDir(t, fixture.workingCopy)

	// Both devices retitle the same task while apart.
	retitleOnDisk(t, fixture.workingCopy, "Ship beta build", "Ship beta build (studio)")
	syncJSON = false
	runSyncCommand(t, "exchange", func() error { return runSyncExchange(nil, nil) })
	retitleOnDisk(t, laptopDir, "Ship beta build", "Ship beta build (laptop)")
	if _, err := laptop.SyncCommits().CommitExternalChanges(); err != nil {
		t.Fatal(err)
	}
	exchangeDevice(t, laptop)
	runSyncCommand(t, "exchange", func() error { return runSyncExchange(nil, nil) })

	listed := listSyncConflicts(t)
	if len(listed) != 1 || listed[0].Path != atlasTasksPath {
		t.Fatalf("conflicts --json lists %+v", listed)
	}
	conflict := listed[0]
	prefix := strings.TrimPrefix(conflict.ID, "sha256:")[:10]

	syncJSON = false
	shown := runSyncCommand(t, "conflicts ID", func() error { return runSyncConflicts(nil, []string{prefix}) })
	for _, want := range []string{
		"Tasks in Atlas Launch", "Started from:", "│ ## Task: Ship beta build\n",
		"## Task: Ship beta build (studio)", "## Task: Ship beta build (laptop)",
		"edit outside tt on", "laptop-b", ", this device", "What differs:", "docket sync resolve " + prefix,
	} {
		if !strings.Contains(shown, want) {
			t.Fatalf("conflicts %s lacks %q:\n%s", prefix, want, shown)
		}
	}
	requireContentFreeDiagnostics(t, conflict.ID)

	for _, version := range conflict.Versions {
		if version.Text != nil && strings.Contains(*version.Text, "(laptop)") {
			syncResolveVersion = version.Index + 1
		}
	}
	resolved := runSyncCommand(t, "resolve", func() error { return runSyncResolve(nil, []string{prefix}) })
	if !strings.Contains(resolved, "Resolved "+prefix) || !strings.Contains(resolved, "0 conflicts still open") {
		t.Fatalf("resolve says:\n%s", resolved)
	}
	if _, _, again := captureDoctorOutput(t, func() error { return runSyncResolve(nil, []string{prefix}) }); again == nil || !strings.Contains(again.Error(), "no open sync conflict") {
		t.Fatalf("resolving a closed conflict again: %v", again)
	}

	// The resolution travels; both replicas converge with nothing open.
	runSyncCommand(t, "exchange", func() error { return runSyncExchange(nil, nil) })
	exchangeDevice(t, laptop)
	if open, openErr := laptop.SyncConflicts(nil); openErr != nil || len(open) != 0 {
		t.Fatalf("the laptop still has %d conflicts (%v)", len(open), openErr)
	}
	if open := listSyncConflicts(t); len(open) != 0 {
		t.Fatalf("the studio still lists %+v", open)
	}
	studio, _ := os.ReadFile(filepath.Join(fixture.workingCopy, filepath.FromSlash(atlasTasksPath)))
	other, _ := os.ReadFile(filepath.Join(laptopDir, filepath.FromSlash(atlasTasksPath)))
	if !bytes.Equal(studio, other) || !bytes.Contains(studio, []byte("## Task: Ship beta build (laptop)\n")) {
		t.Fatalf("the replicas differ or lost the kept version:\nstudio:\n%s\nlaptop:\n%s", studio, other)
	}
}

func runSyncCommand(t *testing.T, name string, fn func() error) string {
	t.Helper()
	stdout, stderr, err := captureDoctorOutput(t, fn)
	if err != nil {
		t.Fatalf("%s: %v\n%s%s", name, err, stdout, stderr)
	}
	return stdout
}

func exchangeDevice(t *testing.T, rt *engine.Runtime) {
	t.Helper()
	if _, err := rt.SyncExchange(engine.SyncExchangeOptions{}); err != nil {
		t.Fatal(err)
	}
}

func listSyncConflicts(t *testing.T) []engine.ConflictView {
	t.Helper()
	syncJSON = true
	defer func() { syncJSON = false }()
	var listed struct {
		Conflicts []engine.ConflictView `json:"conflicts"`
	}
	out := runSyncCommand(t, "conflicts --json", func() error { return runSyncConflicts(nil, nil) })
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("conflicts --json: %v\n%s", err, out)
	}
	return listed.Conflicts
}

// requireContentFreeDiagnostics exports diagnostics to a file and holds them
// to naming the conflict without carrying task Markdown.
func requireContentFreeDiagnostics(t *testing.T, conflictID string) {
	t.Helper()
	syncDiagnosticsOut = filepath.Join(t.TempDir(), "diagnostics.json")
	runSyncCommand(t, "diagnostics", func() error { return runSyncDiagnostics(nil, nil) })
	data, err := os.ReadFile(syncDiagnosticsOut)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics engine.SyncDiagnostics
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Format != engine.SyncDiagnosticsFormat || len(diagnostics.Conflicts) != 1 || diagnostics.Conflicts[0].ID != conflictID ||
		diagnostics.Status == nil || diagnostics.Status.LastExchange == nil {
		t.Fatalf("diagnostics report %s", data)
	}
	if bytes.Contains(data, []byte("## Task:")) {
		t.Fatal("diagnostics carry task Markdown")
	}
}
