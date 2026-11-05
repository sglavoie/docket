//go:build darwin

package handoff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A subscribed session receives the nudge instead of dying of it.
func TestListenForNudgesReceivesTheNudge(t *testing.T) {
	nudges, stop := ListenForNudges()
	defer stop()
	if err := System().Nudge(os.Getpid()); err != nil {
		t.Fatalf("Nudge: %v", err)
	}
	select {
	case <-nudges:
	case <-time.After(5 * time.Second):
		t.Fatal("the nudge never arrived")
	}
}

// The real parent read is read-only, so it runs against this test process:
// ps must name the parent the kernel reports and an absolute executable.
func TestSystemParentReadsThisProcess(t *testing.T) {
	ppid, exe, err := System().Parent(os.Getpid())
	if err != nil {
		t.Fatalf("Parent: %v", err)
	}
	if ppid != os.Getppid() {
		t.Fatalf("Parent ppid = %d, want %d", ppid, os.Getppid())
	}
	if !filepath.IsAbs(exe) {
		t.Fatalf("Parent executable = %q, want an absolute path", exe)
	}
}
