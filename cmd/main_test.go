package cmd

import (
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/testutil/nofsync"

	"github.com/sglavoie/docket/internal/testutil/residue"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/testutil"
)

type testDeps = testutil.TestDeps[*config.RuntimeState]

func setCommandDataDir(t *testing.T, dir string) {
	t.Helper()
	previous := dataDir
	dataDir = dir
	t.Cleanup(func() {
		dataDir = previous
	})
}

func setupTestDeps(t *testing.T) *testDeps {
	t.Helper()
	return testutil.SetupTestDeps(t, func(dataDir string) *config.RuntimeState {
		return config.NewRuntimeState(dataDir, config.NewDomains())
	})
}

func saveTestRuntime(manager *config.Manager, runtime *config.RuntimeState) error {
	domains, err := runtime.CloneDomains()
	if err != nil {
		return err
	}
	if err := manager.CommitCatalog(domains.Catalog); err != nil {
		return err
	}
	if err := manager.CommitPlanning(domains.Planning); err != nil {
		return err
	}
	if err := manager.CommitMachine(domains.Machine); err != nil {
		return err
	}
	return nil
}

// TestMain isolates $HOME for every test in the cmd package. Command tests that
// build a config.Manager directly (rather than through setupTestDeps)
// still exercise Save/LoadOrCreate, which resolves machine-local state under
// ~/.docket/runtime/<workspace-id>/machine.json relative to $HOME. Isolating it
// here keeps those paths from writing into the developer's real home directory
// once that tier becomes part of Manager persistence. Tests that override $HOME
// via t.Setenv restore to this throwaway directory rather than the real home.
//
// residue.Run also fails the run when it leaves .tt/, projects/, or a lock file
// in the package directory, which is what an unrooted runtime or project writes.
func TestMain(m *testing.M) {
	nofsync.Enable()
	tmp, err := os.MkdirTemp("", "tt-cmd-home-*")
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
