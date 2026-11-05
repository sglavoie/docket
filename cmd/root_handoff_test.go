package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/util"
)

// handoffPlatform is a two-process tree, a terminal session under Ghostty, and
// a record of what the hand-off asked of it.
type handoffPlatform struct {
	activateErr error
	calls       []string
}

func (p *handoffPlatform) Parent(pid int) (int, string, error) {
	switch pid {
	case 500:
		return 200, "/usr/local/bin/docket", nil
	case 200:
		return 1, "/Applications/Ghostty.app/Contents/MacOS/ghostty", nil
	}
	return 0, "", fmt.Errorf("no process %d", pid)
}

func (p *handoffPlatform) Activate(app string) error {
	p.calls = append(p.calls, "activate "+app)
	return p.activateErr
}

func (p *handoffPlatform) Nudge(int) error {
	p.calls = append(p.calls, "nudge")
	return nil
}

func TestARefusedLaunchSwitchesToTheRunningSession(t *testing.T) {
	platform := &handoffPlatform{}
	refusal := fmt.Errorf("open: %w", &util.LockHeldError{PID: 500, StartedAt: "2026-11-05T18:49:30Z", Session: util.SessionTerminal})

	line, ok := handOffToRunningSession(platform, refusal)
	if !ok || line != "Docket is already open (PID 500) — switched to Ghostty." {
		t.Fatalf("handOff = %q, %v; want the switched line", line, ok)
	}
	if len(platform.calls) != 2 {
		t.Fatalf("calls = %q, want an activation and a nudge", platform.calls)
	}
}

// Whenever the hand-off cannot happen the caller keeps the refusal it had, so
// the "already running" message and exit status 1 stand.
func TestALaunchThatCannotSwitchKeepsItsRefusal(t *testing.T) {
	held := &util.LockHeldError{PID: 500, Session: util.SessionTerminal}
	cases := map[string]struct {
		platform *handoffPlatform
		err      error
	}{
		"not a lock refusal":   {&handoffPlatform{}, errors.New("open workspace: boom")},
		"no error":             {&handoffPlatform{}, nil},
		"activation fails":     {&handoffPlatform{activateErr: errors.New("no")}, held},
		"holder is no session": {&handoffPlatform{}, &util.LockHeldError{PID: 500}},
	}
	for name, tc := range cases {
		if line, ok := handOffToRunningSession(tc.platform, tc.err); ok {
			t.Errorf("%s: handOff = %q, want the refusal kept", name, line)
		}
	}
	if _, ok := handOffToRunningSession(nil, held); ok {
		t.Error("no platform: handOff switched")
	}
}

// The hand-off reads the refusal RunApp really returns: the lock's own error.
func TestTheLockRefusalIsWhatTheHandOffReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	lock, err := util.AcquireSessionLock(path, util.SessionTerminal)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	_, refusal := util.AcquireSessionLock(path, util.SessionTerminal)

	var held *util.LockHeldError
	if !errors.As(refusal, &held) || held.Session != util.SessionTerminal {
		t.Fatalf("refusal = %v, want a terminal *LockHeldError", refusal)
	}
}
