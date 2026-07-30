package cmd

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
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

func newTestConfig() *config.RuntimeState {
	return config.NewRuntimeState("", config.NewDomains())
}

func setupTestDeps(t *testing.T) *testDeps {
	t.Helper()
	return testutil.SetupTestDeps(t, func(dataDir string) *config.RuntimeState {
		return config.NewRuntimeState(dataDir, config.NewDomains())
	})
}

func setupTestDepsWithProject(t *testing.T, projectName string) (*testDeps, *models.Project) {
	t.Helper()
	return testutil.SetupTestDepsWithProject(t, projectName, func(dataDir string, project *models.Project) *config.RuntimeState {
		domains := config.NewDomains()
		if err := domains.Catalog.UpsertProject(project); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
		return config.NewRuntimeState(dataDir, domains)
	})
}

func setTestProjects(runtime *config.RuntimeState, projects []*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		if err := catalog.UpsertProject(project); err != nil {
			panic(err)
		}
	}
	runtime.SetCatalogInMemory(catalog)
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
// ~/.tt/runtime/<workspace-id>/machine.json relative to $HOME. Isolating it
// here keeps those paths from writing into the developer's real home directory
// once that tier becomes part of Manager persistence. Tests that override $HOME
// via t.Setenv restore to this throwaway directory rather than the real home.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tt-cmd-home-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
