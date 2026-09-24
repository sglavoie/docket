package tui

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/testutil/residue"
)

// TestMain isolates $HOME for every test in the tui package. TUI tests that
// build a config.Manager and persist through it resolve machine-local state
// under ~/.tt/runtime/<workspace-id>/machine.json relative to $HOME.
// Isolating it here keeps those paths from writing into the developer's real
// home directory once that tier becomes part of Manager persistence. Tests that
// override $HOME via t.Setenv restore to this throwaway directory rather than
// the real home.
//
// residue.Run also fails the run when it leaves .tt/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
//
// It also raises the test timeout to 30m when the caller left Go's 10m
// default: this package alone measures ~757s under -race on a lightly loaded
// host, so the default fails a passing run. An explicit -timeout still wins.
func TestMain(m *testing.M) {
	raiseDefaultTestTimeout()
	tmp, err := os.MkdirTemp("", "tt-tui-home-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", tmp); err != nil {
		panic(err)
	}
	code := residue.Run(m.Run)
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

const (
	goDefaultTestTimeout = 10 * time.Minute
	packageTestTimeout   = 30 * time.Minute
)

func raiseDefaultTestTimeout() {
	if !flag.Parsed() {
		flag.Parse()
	}
	timeout := flag.Lookup("test.timeout")
	if timeout == nil || timeout.Value.String() != goDefaultTestTimeout.String() {
		return
	}
	if err := timeout.Value.Set(packageTestTimeout.String()); err != nil {
		panic(err)
	}
}
