package tui

import (
	"fmt"
	"path/filepath"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

const testWorkspaceLedgerDepth = 1000

func newTestConfig() *config.RuntimeState {
	return newTestConfigAt("")
}

func newTestConfigAt(dataDir string) *config.RuntimeState {
	domains := config.NewDomains()
	domains.Settings.FzfEnabled = false
	domains.Settings.GlowEnabled = false
	return config.NewRuntimeState(dataDir, domains)
}

func newTestConfigWithProject(projectName string) (*config.RuntimeState, *models.Project) {
	cfg := newTestConfig()
	project := testutil.NewTestProject(projectName)
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID
	return cfg, project
}

func testConfigWithProjects(projects ...*models.Project) *config.RuntimeState {
	runtime := newTestConfig()
	setTestProjects(runtime, projects)
	return runtime
}

func setTestProjects(runtime *config.RuntimeState, projects []*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		catalog.UpsertProject(project)
	}
	runtime.SetCatalogInMemory(catalog)
}

func addTestProject(runtime *config.RuntimeState, project *models.Project) {
	catalog := runtime.Catalog()
	catalog.UpsertProject(project)
	runtime.SetCatalogInMemory(catalog)
}

func testRuntimeAt(dataDir string, runtime *config.RuntimeState) *config.RuntimeState {
	if runtime == nil {
		return config.EmptyRuntimeState(dataDir)
	}
	domains := &config.Domains{
		Catalog:    runtime.Catalog(),
		Planning:   runtime.Planning(),
		Settings:   runtime.Settings(),
		Automation: runtime.Automation(),
		Machine:    runtime.Machine(),
	}
	if dataDir != "" {
		// Checked non-catalog commits now require the authoritative catalog
		// identity. Persist that one foundation artifact for real-directory test
		// runtimes instead of letting each test mint an unrelated domain ID.
		manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
		if err := manager.CommitCatalog(domains.Catalog); err != nil {
			panic(fmt.Sprintf("seed test workspace catalog: %v", err))
		}
	}
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
