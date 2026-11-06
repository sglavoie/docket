// Package handoff is what a second `docket` launch does when the workspace's
// TUI process lock is already held: instead of only refusing, it brings the
// running session to the front.
//
// The second process cannot reach into the first one's terminal, so the hand-off
// has two halves, both best effort. The second process walks the holder's
// parent chain to the application bundle that owns it — the terminal emulator
// for an ordinary session, Docket.app for one it hosts — and activates that
// application. For a terminal session it also sends the holder SIGUSR1, which
// the holder answers by writing RaiseSequence to its own terminal. Activation
// raises the application and, for most terminals, its frontmost window; it does
// not select the tab or split the session lives in. Only activation decides
// success: the nudge reports nothing back.
//
// Every operating-system call goes through Platform, so tests drive the whole
// decision with a fake and never spawn ps, open, or a signal.
package handoff

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/util"
)

// Platform is the operating-system surface a hand-off touches.
type Platform interface {
	// Parent reports pid's parent process and the absolute path of pid's
	// executable.
	Parent(pid int) (ppid int, executable string, err error)
	// Activate brings the application bundle at app to the front.
	Activate(app string) error
	// Nudge asks the running terminal session pid to raise its window.
	Nudge(pid int) error
}

// maxAncestors bounds the parent walk, so a cycle or a pathological tree cannot
// hold a launch.
const maxAncestors = 64

// ErrNoOwningApp is Switch's refusal when no ancestor of the holder lives in an
// application bundle: a session inside tmux or screen, reached over SSH, or
// started by launchd.
var ErrNoOwningApp = errors.New("no application owns the running session")

// ErrUnsupported is Switch's refusal where no Platform exists, as on every
// operating system but macOS.
var ErrUnsupported = errors.New("hand-off is not supported on this platform")

// ErrNotASession is Switch's refusal for a holder that recorded no session
// kind: a workspace move holding the lock, or a Docket built before the field.
var ErrNotASession = errors.New("the lock holder is not a session Docket can switch to")

// Outcome is what a successful Switch did.
type Outcome struct {
	// App is the application bundle Switch activated.
	App string
}

// AppName is the application's display name: the bundle's base name without
// its .app suffix.
func (o Outcome) AppName() string {
	return strings.TrimSuffix(filepath.Base(o.App), ".app")
}

// Switch hands a refused launch off to the session holding the lock. It
// activates the application that owns the holder and, for a terminal session,
// nudges the holder to raise its window. It fails, changing nothing, when there
// is no Platform, the holder is not a session, or no application owns it.
func Switch(p Platform, held *util.LockHeldError) (Outcome, error) {
	if p == nil {
		return Outcome{}, ErrUnsupported
	}
	if held == nil || held.PID <= 1 {
		return Outcome{}, ErrNotASession
	}
	switch held.Session {
	case util.SessionTerminal, util.SessionHeadless:
	default:
		return Outcome{}, ErrNotASession
	}
	app, err := OwningApp(p, held.PID)
	if err != nil {
		return Outcome{}, err
	}
	if err := p.Activate(app); err != nil {
		return Outcome{}, fmt.Errorf("activate %s: %w", app, err)
	}
	outcome := Outcome{App: app}
	if held.Session == util.SessionTerminal {
		// Best effort: an older terminal ignores the escape, and a failed
		// signal leaves the activation that already happened standing.
		_ = p.Nudge(held.PID)
	}
	return outcome, nil
}

// OwningApp walks pid's ancestors, starting at its parent, to the first one
// whose executable lives in an application bundle, and returns that bundle's
// path. The holder's own executable is skipped, since a docket binary inside
// Docket.app run from a terminal is owned by the terminal, not by Docket.app.
func OwningApp(p Platform, pid int) (string, error) {
	current, _, err := p.Parent(pid)
	if err != nil {
		return "", fmt.Errorf("read process %d: %w", pid, err)
	}
	seen := map[int]bool{pid: true}
	for hops := 0; current > 1 && hops < maxAncestors; hops++ {
		if seen[current] {
			break
		}
		seen[current] = true
		parent, executable, err := p.Parent(current)
		if err != nil {
			return "", fmt.Errorf("read process %d: %w", current, err)
		}
		if app, ok := AppBundle(executable); ok {
			return app, nil
		}
		current = parent
	}
	return "", ErrNoOwningApp
}

// AppBundle returns the outermost application bundle an executable path lies
// in, so a helper nested inside an app (iTerm2's server, VS Code's helpers)
// resolves to the app a person switches to.
func AppBundle(executable string) (string, bool) {
	if !filepath.IsAbs(executable) {
		return "", false
	}
	idx := strings.Index(executable, ".app/")
	if idx < 0 {
		return "", false
	}
	return executable[:idx+len(".app")], true
}

// ParsePS reads one line of `ps -o ppid=,comm=`: the parent PID, then the
// executable path, which may contain spaces.
func ParsePS(out string) (int, string, error) {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return 0, "", fmt.Errorf("unexpected ps output %q", strings.TrimSpace(out))
	}
	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", fmt.Errorf("unexpected ps parent %q", fields[0])
	}
	rest := strings.TrimSpace(strings.TrimSpace(out)[len(fields[0]):])
	return ppid, rest, nil
}

// Message is the line a successful hand-off prints.
func Message(held *util.LockHeldError, outcome Outcome) string {
	if held != nil && held.PID > 0 {
		return fmt.Sprintf("Docket is already open (PID %d) — switched to %s.", held.PID, outcome.AppName())
	}
	return fmt.Sprintf("Docket is already open — switched to %s.", outcome.AppName())
}

// RaiseSequence is what a nudged terminal session writes to its own terminal:
// the xterm window operations de-iconify (CSI 1 t) and raise (CSI 5 t). Neither
// produces a reply, and a terminal that does not implement them ignores them.
const RaiseSequence = "\x1b[1t\x1b[5t"
