package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
)

// syncCommandFixture is one desktop with a workspace to enroll, driven through
// the maintenance commands exactly as a person would.
type syncCommandFixture struct {
	original    string
	workingCopy string
	exchange    string
}

func newSyncCommandFixture(t *testing.T) syncCommandFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	fixture := syncCommandFixture{
		original:    filepath.Join(root, "original"),
		workingCopy: filepath.Join(root, "local", "synced"),
		exchange:    filepath.Join(root, "exchange"),
	}
	if err := os.MkdirAll(filepath.Join(fixture.original, ".tt"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixture.exchange, 0o700); err != nil {
		t.Fatal(err)
	}
	setCommandDataDir(t, fixture.original)
	for _, reset := range []*string{&syncExchangeFolder, &syncWorkingCopy, &syncSnapshotDir} {
		previous := *reset
		t.Cleanup(func() { *reset = previous })
	}
	previousForce, previousJSON := syncForce, syncJSON
	t.Cleanup(func() { syncForce, syncJSON = previousForce, previousJSON })
	return fixture
}

func (f syncCommandFixture) enroll(t *testing.T) string {
	t.Helper()
	syncExchangeFolder, syncWorkingCopy, syncForce = f.exchange, f.workingCopy, true
	stdout, _, err := captureDoctorOutput(t, func() error { return runSyncEnroll(nil, nil) })
	if err != nil {
		t.Fatalf("sync enroll: %v\n%s", err, stdout)
	}
	return stdout
}

// TestSyncCommands enrolls through `tt sync enroll`, then reads the result back
// through `status`, `exchange`, and `doctor`, and holds a workspace never
// enrolled to its old behavior.
func TestSyncCommands(t *testing.T) {
	t.Run("enroll_status_exchange_doctor", func(t *testing.T) {
		fixture := newSyncCommandFixture(t)
		out := fixture.enroll(t)
		for _, want := range []string{"Workspace enrolled; published 1 transaction", "now this machine's workspace", "genesis"} {
			if !strings.Contains(out, want) {
				t.Fatalf("enroll output lacks %q:\n%s", want, out)
			}
		}
		// The working copy is now the active workspace: no --data-dir needed.
		setCommandDataDir(t, "")

		syncJSON = true
		stdout, _, err := captureDoctorOutput(t, func() error { return runSyncStatus(nil, nil) })
		if err != nil {
			t.Fatalf("sync status: %v", err)
		}
		var status engine.SyncStatus
		if err := json.Unmarshal([]byte(stdout), &status); err != nil {
			t.Fatalf("status JSON: %v\n%s", err, stdout)
		}
		if !status.Enrolled || !status.Genesis || status.OutstandingUp != 0 || status.Schemas["catalog"] != config.SyncEnrolledConfigVersion {
			t.Fatalf("status = %+v", status)
		}

		syncJSON = false
		syncExchangeFolder = ""
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncExchange(nil, nil) })
		if err != nil || !strings.Contains(stdout, "published:   0") {
			t.Fatalf("sync exchange: %v\n%s", err, stdout)
		}
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
		if err != nil {
			t.Fatalf("sync doctor on a fresh enrollment: %v\n%s", err, stdout)
		}
		for _, want := range []string{"enrolled", "the working copy is local", "reachable and this workspace's", "the original is preserved"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("doctor output lacks %q:\n%s", want, stdout)
			}
		}

		// Diagnostics: an exchange folder that went away is an issue.
		if err := os.Rename(fixture.exchange, fixture.exchange+"-moved"); err != nil {
			t.Fatal(err)
		}
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
		if err == nil || !strings.Contains(stdout, "!! exchange") {
			t.Fatalf("doctor did not report the missing exchange folder: %v\n%s", err, stdout)
		}
		syncExchangeFolder = fixture.exchange + "-moved"
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncExchange(nil, nil) })
		if err != nil {
			t.Fatalf("exchange --folder: %v\n%s", err, stdout)
		}
		record, err := config.LoadEnrollmentRecord(fixture.workingCopy)
		if err != nil || record.ExchangeFolder != fixture.exchange+"-moved" {
			t.Fatalf("exchange --folder did not repoint the device: %+v, %v", record, err)
		}
	})

	t.Run("a_workspace_never_enrolled_is_told_how", func(t *testing.T) {
		newSyncCommandFixture(t)
		stdout, _, err := captureDoctorOutput(t, func() error { return runSyncStatus(nil, nil) })
		if err != nil || !strings.Contains(stdout, "not enrolled in sync") {
			t.Fatalf("status: %v\n%s", err, stdout)
		}
		if _, _, err := captureDoctorOutput(t, func() error { return runSyncExchange(nil, nil) }); err == nil ||
			!strings.Contains(err.Error(), "not enrolled") {
			t.Fatalf("exchange on a workspace never enrolled = %v", err)
		}
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
		if err != nil || !strings.Contains(stdout, "not enrolled") {
			t.Fatalf("doctor: %v\n%s", err, stdout)
		}
	})

	t.Run("enroll_refuses_unconfirmed_and_an_icloud_working_copy", func(t *testing.T) {
		fixture := newSyncCommandFixture(t)
		syncExchangeFolder, syncWorkingCopy, syncForce = fixture.exchange, fixture.workingCopy, false
		previous := confirmInput
		confirmInput = strings.NewReader("")
		t.Cleanup(func() { confirmInput = previous })
		if _, _, err := captureDoctorOutput(t, func() error { return runSyncEnroll(nil, nil) }); err == nil {
			t.Fatal("an unanswered prompt enrolled")
		}
		home, _ := os.UserHomeDir()
		syncForce = true
		syncWorkingCopy = filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "tt")
		if _, _, err := captureDoctorOutput(t, func() error { return runSyncEnroll(nil, nil) }); err == nil ||
			!strings.Contains(err.Error(), "iCloud Drive") {
			t.Fatalf("an iCloud working copy = %v", err)
		}
		if _, err := os.Stat(fixture.workingCopy); !os.IsNotExist(err) {
			t.Fatal("a refused enrollment created the working copy")
		}
	})
}
