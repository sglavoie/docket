package residue

import (
	"os"
	"strings"
	"testing"
)

func TestSnapshotReportsOnlyNewResidue(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("projects", 0o700); err != nil {
		t.Fatal(err)
	}
	check := Snapshot()
	if err := check(); err != nil {
		t.Fatalf("pre-existing residue reported: %v", err)
	}
	if err := os.Mkdir(".tt", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("tasks.md.lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := check()
	if err == nil {
		t.Fatal("new residue went unreported")
	}
	for _, want := range []string{".tt", "tasks.md.lock"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("report %q does not name %s", err, want)
		}
	}
	if strings.Contains(err.Error(), "projects") {
		t.Errorf("report %q names pre-existing projects/", err)
	}
}

func TestRunFailsAPassingBinaryThatLeftResidue(t *testing.T) {
	t.Chdir(t.TempDir())
	code := Run(func() int {
		if err := os.WriteFile(".lock", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		return 0
	})
	if code == 0 {
		t.Fatal("Run returned 0 after residue appeared")
	}
}
