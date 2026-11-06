package handoff

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/util"
)

// proc is one process in a fakePlatform's tree.
type proc struct {
	ppid int
	exe  string
}

// fakePlatform is a process tree and a record of what Switch asked of it, so
// no test spawns ps, open, or a signal.
type fakePlatform struct {
	tree        map[int]proc
	activateErr error
	nudgeErr    error
	calls       []string
}

func (f *fakePlatform) Parent(pid int) (int, string, error) {
	p, ok := f.tree[pid]
	if !ok {
		return 0, "", errors.New("no such process")
	}
	return p.ppid, p.exe, nil
}

func (f *fakePlatform) Activate(app string) error {
	f.calls = append(f.calls, "activate "+app)
	return f.activateErr
}

func (f *fakePlatform) Nudge(pid int) error {
	f.calls = append(f.calls, "nudge")
	return f.nudgeErr
}

// ghosttyTree is docket in a zsh under login under Ghostty.
func ghosttyTree() map[int]proc {
	return map[int]proc{
		500: {ppid: 400, exe: "/usr/local/bin/docket"},
		400: {ppid: 300, exe: "-zsh"},
		300: {ppid: 200, exe: "/usr/bin/login"},
		200: {ppid: 1, exe: "/Applications/Ghostty.app/Contents/MacOS/ghostty"},
		1:   {ppid: 0, exe: "/sbin/launchd"},
	}
}

func TestSwitchActivatesTheTerminalAndNudgesTheSession(t *testing.T) {
	fake := &fakePlatform{tree: ghosttyTree()}
	held := &util.LockHeldError{PID: 500, StartedAt: "2026-11-05T18:49:30Z", Session: util.SessionTerminal}

	outcome, err := Switch(fake, held)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	want := []string{"activate /Applications/Ghostty.app", "nudge"}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
	if outcome.AppName() != "Ghostty" {
		t.Fatalf("outcome = %+v, want Ghostty", outcome)
	}
	if got := Message(held, outcome); got != "Docket is already open (PID 500) — switched to Ghostty." {
		t.Fatalf("Message = %q", got)
	}
}

// A headless session has no terminal, so Docket.app, its parent, is what comes
// to the front, and nothing is signaled.
func TestSwitchActivatesDocketAppForAHeadlessSession(t *testing.T) {
	fake := &fakePlatform{tree: map[int]proc{
		700: {ppid: 600, exe: "/Applications/Docket.app/Contents/Resources/docket"},
		600: {ppid: 1, exe: "/Applications/Docket.app/Contents/MacOS/Docket"},
	}}
	_, err := Switch(fake, &util.LockHeldError{PID: 700, Session: util.SessionHeadless})
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if want := []string{"activate /Applications/Docket.app"}; !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

// A docket binary that lives inside Docket.app but runs in a terminal belongs to
// the terminal: the holder's own executable never names the owner.
func TestOwningAppSkipsTheHoldersOwnExecutable(t *testing.T) {
	fake := &fakePlatform{tree: map[int]proc{
		500: {ppid: 400, exe: "/Applications/Docket.app/Contents/Resources/docket"},
		400: {ppid: 200, exe: "/bin/zsh"},
		200: {ppid: 1, exe: "/Applications/iTerm.app/Contents/MacOS/iTermServer-3.5"},
	}}
	app, err := OwningApp(fake, 500)
	if err != nil || app != "/Applications/iTerm.app" {
		t.Fatalf("OwningApp = %q, %v; want iTerm.app", app, err)
	}
}

// Inside tmux the chain ends at launchd with no application, so the launch
// keeps its refusal and nothing is activated or signaled.
func TestSwitchRefusesWhenNoApplicationOwnsTheSession(t *testing.T) {
	fake := &fakePlatform{tree: map[int]proc{
		500: {ppid: 400, exe: "/usr/local/bin/docket"},
		400: {ppid: 300, exe: "-zsh"},
		300: {ppid: 1, exe: "tmux"},
	}}
	_, err := Switch(fake, &util.LockHeldError{PID: 500, Session: util.SessionTerminal})
	if !errors.Is(err, ErrNoOwningApp) {
		t.Fatalf("Switch = %v, want ErrNoOwningApp", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("calls = %q, want none", fake.calls)
	}
}

// A holder that recorded no session is a workspace move or an older Docket,
// which SIGUSR1 would end; it is never touched.
func TestSwitchRefusesAHolderThatIsNotASession(t *testing.T) {
	for _, held := range []*util.LockHeldError{
		nil,
		{PID: 500},
		{PID: 500, Session: "move"},
		{PID: -1, Session: util.SessionTerminal},
	} {
		fake := &fakePlatform{tree: ghosttyTree()}
		if _, err := Switch(fake, held); !errors.Is(err, ErrNotASession) {
			t.Fatalf("Switch(%+v) = %v, want ErrNotASession", held, err)
		}
		if len(fake.calls) != 0 {
			t.Fatalf("Switch(%+v) calls = %q, want none", held, fake.calls)
		}
	}
}

func TestSwitchWithoutAPlatformIsUnsupported(t *testing.T) {
	if _, err := Switch(nil, &util.LockHeldError{PID: 500, Session: util.SessionTerminal}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Switch = %v, want ErrUnsupported", err)
	}
}

// Activation alone decides success: a failed activation nudges nothing, and a
// failed nudge leaves a successful activation standing.
func TestActivationDecidesTheOutcome(t *testing.T) {
	held := &util.LockHeldError{PID: 500, Session: util.SessionTerminal}

	failing := &fakePlatform{tree: ghosttyTree(), activateErr: errors.New("LaunchServices said no")}
	if _, err := Switch(failing, held); err == nil || !strings.Contains(err.Error(), "LaunchServices said no") {
		t.Fatalf("Switch = %v, want the activation failure", err)
	}
	if want := []string{"activate /Applications/Ghostty.app"}; !reflect.DeepEqual(failing.calls, want) {
		t.Fatalf("calls = %q, want %q", failing.calls, want)
	}

	quiet := &fakePlatform{tree: ghosttyTree(), nudgeErr: errors.New("no such process")}
	outcome, err := Switch(quiet, held)
	if err != nil || outcome.AppName() != "Ghostty" {
		t.Fatalf("Switch = %+v, %v; want the activation to stand despite the failed nudge", outcome, err)
	}
}

func TestOwningAppSurvivesACycleAndAMissingProcess(t *testing.T) {
	cycle := &fakePlatform{tree: map[int]proc{
		500: {ppid: 400, exe: "/usr/local/bin/docket"},
		400: {ppid: 300, exe: "a"},
		300: {ppid: 400, exe: "b"},
	}}
	if _, err := OwningApp(cycle, 500); !errors.Is(err, ErrNoOwningApp) {
		t.Fatalf("cycle: OwningApp = %v, want ErrNoOwningApp", err)
	}
	missing := &fakePlatform{tree: map[int]proc{500: {ppid: 400, exe: "/usr/local/bin/docket"}}}
	if _, err := OwningApp(missing, 500); err == nil || errors.Is(err, ErrNoOwningApp) {
		t.Fatalf("missing: OwningApp = %v, want a read failure", err)
	}
}

func TestAppBundleIsTheOutermostBundle(t *testing.T) {
	cases := map[string]string{
		"/Applications/Ghostty.app/Contents/MacOS/ghostty":                                             "/Applications/Ghostty.app",
		"/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper.app/Contents/MacOS/Code": "/Applications/Visual Studio Code.app",
		"/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal":                          "/System/Applications/Utilities/Terminal.app",
	}
	for exe, want := range cases {
		if got, ok := AppBundle(exe); !ok || got != want {
			t.Errorf("AppBundle(%q) = %q, %v; want %q", exe, got, ok, want)
		}
	}
	for _, exe := range []string{"/usr/bin/login", "-zsh", "tmux", "Ghostty.app/Contents/MacOS/ghostty", ""} {
		if got, ok := AppBundle(exe); ok {
			t.Errorf("AppBundle(%q) = %q, want no bundle", exe, got)
		}
	}
}

func TestParsePSKeepsSpacesInThePath(t *testing.T) {
	ppid, exe, err := ParsePS("  1234 /Applications/Visual Studio Code.app/Contents/MacOS/Electron\n")
	if err != nil || ppid != 1234 || exe != "/Applications/Visual Studio Code.app/Contents/MacOS/Electron" {
		t.Fatalf("ParsePS = %d, %q, %v", ppid, exe, err)
	}
	for _, bad := range []string{"", "1234", "x /bin/zsh"} {
		if _, _, err := ParsePS(bad); err == nil {
			t.Errorf("ParsePS(%q) succeeded", bad)
		}
	}
}

// The raise is two xterm window operations that ask for no reply, so nothing
// lands in the session's input.
func TestRaiseSequenceAsksForNoReply(t *testing.T) {
	if RaiseSequence != "\x1b[1t\x1b[5t" {
		t.Fatalf("RaiseSequence = %q", RaiseSequence)
	}
}
