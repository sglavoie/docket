package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// withPersister sets a custom Persister for the AppModel. It exists only for
// tests that need a mock implementation, which is why it lives here rather than
// beside the other AppOptions in app_init.go.
func withPersister(p coordinator.Persister) AppOption {
	return func(app *AppModel) {
		app.domain.persister = p
	}
}

func commitPlanningToRuntime(runtime *config.RuntimeState, mutate func(*config.PlanningState)) error {
	if runtime == nil || runtime.DataDir() == "" {
		return nil
	}
	planning := coordinator.ClonePlanningState(runtime.Planning())
	mutate(planning)
	return runtime.CommitPlanning(planning)
}

func commitCatalogToRuntime(runtime *config.RuntimeState, mutate func(*config.WorkspaceCatalog) error) error {
	if runtime == nil {
		return nil
	}
	catalog := coordinator.CloneWorkspaceCatalog(runtime.Catalog())
	if err := mutate(catalog); err != nil {
		return err
	}
	if runtime.DataDir() == "" {
		runtime.SetCatalogInMemory(catalog)
		return nil
	}
	return runtime.CommitCatalog(catalog)
}

func nextOrderContains(planning *config.PlanningState, identity taskref.TaskIdentity) bool {
	for _, item := range planning.GetNext() {
		if item.TaskID == identity.TaskID && item.ProjectID == identity.ProjectID {
			return true
		}
	}
	return false
}

func todayContains(planning *config.PlanningState, identity taskref.TaskIdentity) bool {
	for _, item := range planning.GetToday() {
		if item.TaskID == identity.TaskID && item.ProjectID == identity.ProjectID {
			return true
		}
	}
	return false
}
