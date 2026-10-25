package tui

import (
	"fmt"
	"path/filepath"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
)

func newTestConfig() *config.RuntimeState {
	return newTestConfigAt("")
}

func newTestConfigAt(dataDir string) *config.RuntimeState {
	return config.NewRuntimeState(dataDir, config.NewDomains())
}

func testConfigWithProjects(projects ...*models.Project) *config.RuntimeState {
	runtime := newTestConfig()
	setTestProjects(runtime, projects)
	return runtime
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

func testRuntimeAt(dataDir string, runtime *config.RuntimeState) *config.RuntimeState {
	if runtime == nil {
		return config.EmptyRuntimeState(dataDir)
	}
	domains := &config.Domains{
		Catalog:  runtime.Catalog(),
		Planning: runtime.Planning(),
		Machine:  runtime.Machine(),
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
	if err := manager.CommitMachine(domains.Machine); err != nil {
		return err
	}
	return nil
}
