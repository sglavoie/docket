package cmd

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

type testDeps = testutil.TestDeps[*config.RuntimeState]

func newTestConfig() *config.RuntimeState {
	domains := config.NewDomains()
	domains.Settings.FzfEnabled = false
	domains.Settings.GlowEnabled = false
	domains.Machine.CurrentProject = ""
	return config.NewRuntimeState("", domains)
}

func newTestConfigWithProject(projectName string) (*config.RuntimeState, *models.Project) {
	runtime := newTestConfig()
	project := testutil.NewTestProject(projectName)
	addTestProject(runtime, project)
	runtime.Machine().CurrentProject = project.ID
	return runtime, project
}

func setupTestDeps(t *testing.T) *testDeps {
	t.Helper()
	return testutil.SetupTestDeps(t, func(dataDir string) *config.RuntimeState {
		domains := config.NewDomains()
		domains.Settings.FzfEnabled = false
		domains.Settings.GlowEnabled = false
		return config.NewRuntimeState(dataDir, domains)
	})
}

func setupTestDepsWithProject(t *testing.T, projectName string) (*testDeps, *models.Project) {
	t.Helper()
	return testutil.SetupTestDepsWithProject(t, projectName, func(dataDir string, project *models.Project) *config.RuntimeState {
		domains := config.NewDomains()
		domains.Settings.FzfEnabled = false
		domains.Settings.GlowEnabled = false
		domains.Catalog.UpsertProject(project)
		domains.Machine.CurrentProject = project.ID
		return config.NewRuntimeState(dataDir, domains)
	})
}

func addTestProject(runtime *config.RuntimeState, project *models.Project) {
	catalog := runtime.Catalog()
	catalog.UpsertProject(project)
	runtime.SetCatalogInMemory(catalog)
}

func setTestProjects(runtime *config.RuntimeState, projects []*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		catalog.UpsertProject(project)
	}
	runtime.SetCatalogInMemory(catalog)
}

func newFileBackedTestRuntime(dataDir, currentProject string, projects ...*models.Project) *config.RuntimeState {
	domains := config.NewDomains()
	for _, project := range projects {
		domains.Catalog.UpsertProject(project)
	}
	domains.Machine.CurrentProject = currentProject
	return config.NewRuntimeState(dataDir, domains)
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
	if err := manager.CommitSettings(domains.Settings, domains.Machine); err != nil {
		return err
	}
	return manager.CommitAutomation(domains.Automation)
}

// TestMain isolates $HOME for every test in the cmd package. Command tests that
// build a config.Manager directly (rather than through setupTestDeps)
// still exercise Save/LoadOrCreate, which resolves machine-local runtime state
// (machine.json under LocalRuntimeDirForDataDir) relative to $HOME. Isolating it
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
