package transaction

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverPendingFinishesJournalAndDefersOnlyOrphans(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	target := filepath.Join(root, "project", "tasks.md")
	m := prepareWrite(t, j, target, "committed work")
	orphan := filepath.Join(root, ".tt-stage-abandoned-0.stage")
	if err := os.WriteFile(orphan, []byte("never prepared"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := j.RecoverPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.RolledForward) != 1 || report.RolledForward[0] != m.ID || report.Blocked() {
		t.Fatalf("pending recovery: %+v", report)
	}
	if got := mustReadFile(t, target); got != "committed work" {
		t.Fatal(got)
	}
	if got := mustReadFile(t, orphan); got != "never prepared" {
		t.Fatal(got)
	}
	assertMissing(t, j.ManifestPath(m.ID))
	report, err = j.Recover()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.SweptOrphans) != 1 || report.SweptOrphans[0] != orphan {
		t.Fatalf("full recovery: %+v", report)
	}
	assertMissing(t, orphan)
}

func TestRecoverPendingStillBlocksCorruptJournal(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)
	if err := os.MkdirAll(j.dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(j.ManifestPath("broken"), []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		report, err := j.RecoverPending()
		if err != nil {
			t.Fatal(err)
		}
		if !report.Blocked() {
			t.Fatalf("pass %d lost quarantine: %+v", n, report)
		}
	}
}

// Keep the cost of a foreground recovery independent of retained sync history.
func BenchmarkRecoveryWithHistory(b *testing.B) {
	root := b.TempDir()
	dir := filepath.Join(root, ".tt", "sync", "index")
	if err := os.MkdirAll(dir, 0700); err != nil {
		b.Fatal(err)
	}
	for n := 0; n < 10000; n++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%05d.json", n)), []byte("{}"), 0600); err != nil {
			b.Fatal(err)
		}
	}
	j := NewJournal(filepath.Join(root, ".tt", "transactions"), []string{root}, "benchmark")
	for _, pending := range []bool{false, true} {
		name := "full"
		if pending {
			name = "foreground"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if pending {
					_, err = j.RecoverPending()
				} else {
					_, err = j.Recover()
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
