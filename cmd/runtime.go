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
		return nil, fmt.Errorf("failed to load config: %w", err)
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

// LoadRuntimeProjectOrCurrent resolves an explicit project argument or the
// machine-local current project from the cached runtime domains.
func LoadRuntimeProjectOrCurrent(args []string) (*models.Project, *CLIWorkspace, error) {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return nil, nil, err
	}
	var project *models.Project
	if len(args) > 0 {
		project = workspace.Runtime.GetProject(args[0])
		if project == nil {
			return nil, nil, tterrors.NewProjectNotFoundError(args[0])
		}
	} else {
		return nil, nil, NoCurrentProjectError()
	}
	return project, workspace, nil
}
