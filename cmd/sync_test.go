package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/syncexchange"
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
	if err := os.MkdirAll(filepath.Join(fixture.original, ".docket"), 0o700); err != nil {
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

// TestSyncCommands enrolls through `docket sync enroll`, then reads the result back
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
		if decodeErr := json.Unmarshal([]byte(stdout), &status); decodeErr != nil {
			t.Fatalf("status JSON: %v\n%s", decodeErr, stdout)
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
		if renameErr := os.Rename(fixture.exchange, fixture.exchange+"-moved"); renameErr != nil {
			t.Fatal(renameErr)
		}
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
		if err == nil || !strings.Contains(stdout, "!! exchange") {
			t.Fatalf("doctor did not report the missing exchange folder: %v\n%s", err, stdout)
		}
		requireSyncDoctorIssuesExit(t, err)
		requireFailedExchangeKeepsFolder(t, fixture)
		syncExchangeFolder = fixture.exchange + "-moved"
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncExchange(nil, nil) })
		if err != nil {
			t.Fatalf("exchange --exchange: %v\n%s", err, stdout)
		}
		record, err := config.LoadEnrollmentRecord(fixture.workingCopy)
		if err != nil || record.ExchangeFolder != fixture.exchange+"-moved" {
			t.Fatalf("exchange --exchange did not repoint the device: %+v, %v", record, err)
		}
	})

	t.Run("a_workspace_never_enrolled_is_told_how", func(t *testing.T) {
		newSyncCommandFixture(t)
		stdout, _, err := captureDoctorOutput(t, func() error { return runSyncStatus(nil, nil) })
		if err != nil || !strings.Contains(stdout, "not enrolled in sync") {
			t.Fatalf("status: %v\n%s", err, stdout)
		}
		if _, _, exchangeErr := captureDoctorOutput(t, func() error { return runSyncExchange(nil, nil) }); exchangeErr == nil ||
			!strings.Contains(exchangeErr.Error(), "not enrolled") {
			t.Fatalf("exchange on a workspace never enrolled = %v", exchangeErr)
		}
		stdout, _, err = captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
		if err != nil || !strings.Contains(stdout, "not enrolled") {
			t.Fatalf("doctor: %v\n%s", err, stdout)
		}
		requireNotEnrolledReportsSucceed(t)
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

func TestSyncCommandsReportDownloadRequestFailure(t *testing.T) {
	fixture := newSyncCommandFixture(t)
	fixture.enroll(t)
	setCommandDataDir(t, fixture.workingCopy)
	rt, err := openSyncRuntime()
	if err != nil {
		t.Fatal(err)
	}
	state := struct {
		Last        syncexchange.FolderReport `json:"last"`
		LastSuccess time.Time                 `json:"last_success"`
	}{Last: syncexchange.FolderReport{Succeeded: true, Errors: []string{"iCloud download request refused"}}, LastSuccess: time.Now()}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(syncexchange.StatusPath(rt.SyncCommits().SyncHistory()), data, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	out, _, err := captureDoctorOutput(t, func() error { return runSyncStatus(nil, nil) })
	if err != nil || !strings.Contains(out, "iCloud download request refused") {
		t.Fatalf("status hid download error: %s (%v)", out, err)
	}
	out, _, err = captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
	if err == nil || !strings.Contains(out, "iCloud download request refused") {
		t.Fatalf("doctor hid download error: %s (%v)", out, err)
	}
}

// requireSyncDoctorIssuesExit holds sync doctor's issues verdict to doctor's:
// exit 2, apart from "could not run" (1), and quiet under --json.
func requireSyncDoctorIssuesExit(t *testing.T, err error) {
	t.Helper()
	var coded interface{ ExitCode() int }
	if !errors.As(err, &coded) || coded.ExitCode() != 2 || err.Error() != "sync doctor found 1 issue" {
		t.Fatalf("sync doctor issues error = %v, want exit 2 naming one issue", err)
	}
	syncJSON = true
	stdout, _, err := captureDoctorOutput(t, func() error { return runSyncDoctor(nil, nil) })
	syncJSON = false
	if !errors.As(err, &coded) || coded.ExitCode() != 2 || err.Error() != "" || !json.Valid([]byte(stdout)) {
		t.Fatalf("sync doctor --json = %v (%q), want a quiet exit 2 over valid JSON:\n%s", err, err, stdout)
	}
}

// requireFailedExchangeKeepsFolder holds that a folder the pass cannot use is
// never remembered: the record keeps naming the folder configured before.
func requireFailedExchangeKeepsFolder(t *testing.T, fixture syncCommandFixture) {
	t.Helper()
	notAFolder := filepath.Join(t.TempDir(), "not-a-folder")
	if writeErr := os.WriteFile(notAFolder, []byte("x"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	syncExchangeFolder = notAFolder
	if _, _, exchangeErr := captureDoctorOutput(t, func() error { return runSyncExchange(nil, nil) }); exchangeErr == nil ||
		!strings.Contains(exchangeErr.Error(), "the exchange folder was not changed") {
		t.Fatalf("exchange --exchange over a file = %v, want a refusal that keeps the folder", exchangeErr)
	}
	if kept, loadErr := config.LoadEnrollmentRecord(fixture.workingCopy); loadErr != nil || kept.ExchangeFolder != fixture.exchange {
		t.Fatalf("a failed exchange --exchange repointed the device: %+v, %v", kept, loadErr)
	}
}

// requireNotEnrolledReportsSucceed holds the read-only reports doctor sits
// beside to the same answer for a workspace that does not sync: the hint, exit 0.
// Diagnostics is the exception: its only output is a JSON document, so it
// refuses with the hint as its error and leaves stdout and --file empty.
func requireNotEnrolledReportsSucceed(t *testing.T) {
	t.Helper()
	stdout, _, err := captureDoctorOutput(t, func() error { return runSyncConflicts(nil, nil) })
	if err != nil || !strings.Contains(stdout, notEnrolledHint) {
		t.Fatalf("conflicts: %v\n%s", err, stdout)
	}
	syncDiagnosticsOut = ""
	stdout, _, err = captureDoctorOutput(t, func() error { return runSyncDiagnostics(nil, nil) })
	if err == nil || err.Error() != notEnrolledHint || stdout != "" {
		t.Fatalf("diagnostics: err = %v, stdout = %q; want the hint as the error and no output", err, stdout)
	}
	diagnosticsFile := filepath.Join(t.TempDir(), "diagnostics.json")
	syncDiagnosticsOut = diagnosticsFile
	t.Cleanup(func() { syncDiagnosticsOut = "" })
	if _, _, err = captureDoctorOutput(t, func() error { return runSyncDiagnostics(nil, nil) }); err == nil {
		t.Fatal("diagnostics --file accepted a workspace that is not enrolled")
	}
	if _, statErr := os.Stat(diagnosticsFile); !os.IsNotExist(statErr) {
		t.Fatalf("diagnostics wrote %s for a workspace that is not enrolled: %v", diagnosticsFile, statErr)
	}
	syncDiagnosticsOut = ""
	syncJSON = true
	t.Cleanup(func() { syncJSON = false })
	stdout, _, err = captureDoctorOutput(t, func() error { return runSyncConflicts(nil, nil) })
	if err != nil || !strings.Contains(stdout, `"conflicts": []`) {
		t.Fatalf("conflicts --json: %v\n%s", err, stdout)
	}
}
