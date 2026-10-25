package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncDiagnosticsWorkspaceDigest holds the diagnostics digest to what a
// device qualification compares: two devices that exchanged both ways report
// the same digest, work not yet exchanged makes them differ, and device-local
// state such as the journal and undo ledger never does.
func TestSyncDiagnosticsWorkspaceDigest(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, "desk-a")
	phone := env.join(t)
	digest := func(rt *Runtime) string {
		t.Helper()
		report, err := rt.SyncDiagnostics(nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(report.WorkspaceDigest, "sha256:") {
			t.Fatalf("diagnostics digest %q", report.WorkspaceDigest)
		}
		return report.WorkspaceDigest
	}

	if digest(desk) != digest(phone) {
		t.Fatal("a freshly joined device reports a different digest")
	}

	captureIn(t, phone, "Captured on the phone")
	if digest(desk) == digest(phone) {
		t.Fatal("an unexchanged capture left the digests equal")
	}
	exchangeNow(t, phone)
	exchangeNow(t, desk)
	if digest(desk) != digest(phone) {
		t.Fatal("the devices exchanged both ways and still report different digests")
	}

	before := digest(desk)
	local := filepath.Join(env.workingCopy, ".docket", "undo", "qualification-probe")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("device-local"), 0o644); err != nil {
		t.Fatal(err)
	}
	if digest(desk) != before {
		t.Fatal("device-local state changed the digest")
	}
}
