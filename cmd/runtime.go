package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// CLIWorkspace is the facade-free composition root shared by CLI commands. A
// command constructs it once, reads the cached domain snapshot, and uses the
// manager only to build revision-checked artifacts or domain commits.
type CLIWorkspace struct {
	Manager *config.Manager
	Runtime *config.RuntimeState
}

// LoadCLIWorkspace ensures fresh workspaces have a complete split-domain set,
// then builds the cached RuntimeState and RuntimeDependencies composition once
// for the command.
func LoadCLIWorkspace() (*CLIWorkspace, error) {
	return loadCLIWorkspace()
}

func loadCLIWorkspace() (*CLIWorkspace, error) {
	manager := config.NewManager(resolveConfigPath())
	_, err := manager.LoadOrCreateDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace: %w", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime state: %w", err)
	}
	return &CLIWorkspace{
		Manager: manager,
		Runtime: runtime,
	}, nil
}

// LoadRuntimeProject resolves one project by slug or ID from the cached runtime
// domains.
func LoadRuntimeProject(ref string) (*models.Project, *CLIWorkspace, error) {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return nil, nil, err
	}
	project := workspace.Runtime.GetProject(ref)
	if project == nil {
		return nil, nil, tterrors.NewProjectNotFoundError(ref)
	}
	return project, workspace, nil
}
