package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
)

// CLIWorkspace is the facade-free composition root shared by CLI commands. A
// command constructs it once, reads the cached domain snapshot, and uses the
// manager only to build revision-checked artifacts or domain commits.
type CLIWorkspace struct {
	Manager      *config.Manager
	Dependencies *config.RuntimeDependencies
	Runtime      *config.RuntimeState
}

// LoadCLIWorkspace ensures fresh workspaces have a complete split-domain set,
// then builds the cached RuntimeState and RuntimeDependencies composition once
// for the command.
func LoadCLIWorkspace() (*CLIWorkspace, error) {
	return loadCLIWorkspace(false)
}

// LoadCLIWorkspaceStrict loads an existing workspace without creating a fresh
// one. Commands whose legacy surface required an already-initialized config use
// this variant while still receiving only domain-native runtime dependencies.
func LoadCLIWorkspaceStrict() (*CLIWorkspace, error) {
	return loadCLIWorkspace(true)
}

func loadCLIWorkspace(strict bool) (*CLIWorkspace, error) {
	manager := config.NewManager(resolveConfigPath())
	var err error
	if strict {
		if recoverErr := manager.RecoverWorkspace(); recoverErr != nil {
			return nil, fmt.Errorf("failed to recover workspace before startup: %w", recoverErr)
		}
		_, err = manager.LoadDomains()
	} else {
		_, err = manager.LoadOrCreateDomains()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime state: %w", err)
	}
	applyRuntimeSettings(runtime.Settings())
	return &CLIWorkspace{
		Manager:      manager,
		Dependencies: manager.RuntimeDependencies(),
		Runtime:      runtime,
	}, nil
}

func applyRuntimeSettings(settings *config.Settings) {
	if settings == nil {
		return
	}
	if settings.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *settings.MaxActivityEntries
	}
	external.SetFzfTimeout(settings.GetFzfTimeout())
	icons.SetUseEmoji(settings.GetUseEmoji())
}

// LoadRuntimeProject resolves a project by ID or slug from the cached catalog.
func LoadRuntimeProject(projectRef string) (*models.Project, *CLIWorkspace, error) {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return nil, nil, err
	}
	project := workspace.Runtime.GetProject(projectRef)
	if project == nil {
		return nil, nil, tterrors.NewProjectNotFoundError(projectRef)
	}
	return project, workspace, nil
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
		project = workspace.Runtime.CurrentProject()
		if project == nil {
			return nil, nil, NoCurrentProjectError()
		}
	}
	return project, workspace, nil
}

// LoadRuntimeCurrentProject resolves and loads the machine-selected project,
// preserving the parser-warning behavior of LoadCurrentProject for callers that
// have migrated to the domain-native composition root.
func LoadRuntimeCurrentProject() (*models.Project, *CLIWorkspace, error) {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return nil, nil, err
	}
	project, err := loadRuntimeCurrentProject(workspace)
	if err != nil {
		return nil, nil, err
	}
	return project, workspace, nil
}

// loadRuntimeCurrentProject resolves and loads the machine-selected project
// from an already-loaded command workspace. Callers that also need to mutate a
// config domain reuse the command-start RuntimeState and its revision baseline
// rather than loading a second, potentially inconsistent snapshot.
func loadRuntimeCurrentProject(workspace *CLIWorkspace) (*models.Project, error) {
	if workspace == nil || workspace.Runtime == nil {
		return nil, fmt.Errorf("runtime state is unavailable")
	}
	project := workspace.Runtime.CurrentProject()
	if project == nil {
		return nil, NoCurrentProjectError()
	}
	projects, err := loadRuntimeProjects(workspace, []*models.Project{project})
	if err != nil {
		return nil, err
	}
	return projects[0], nil
}

// loadRuntimeActiveProjects resolves and loads every non-archived,
// non-sandbox, non-internal project from the command-start runtime snapshot.
// Ask uses this instead of the machine-local current project so contextual
// requests always describe the same active workspace scope as the TUI.
func loadRuntimeActiveProjects(workspace *CLIWorkspace) ([]*models.Project, error) {
	if workspace == nil || workspace.Runtime == nil {
		return nil, fmt.Errorf("runtime state is unavailable")
	}
	return loadRuntimeProjects(workspace, workspace.Runtime.ActiveProjects())
}

// loadRuntimeProjects preserves the parser-warning behavior of the historic
// current-project loader while loading a caller-selected catalog snapshot.
func loadRuntimeProjects(workspace *CLIWorkspace, projects []*models.Project) ([]*models.Project, error) {
	if workspace == nil || workspace.Runtime == nil {
		return nil, fmt.Errorf("runtime state is unavailable")
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		Debugf("loading project: %s (%s)", project.Name, project.Path)
		result, err := storage.LoadProjectWithWarnings(project)
		if err != nil {
			if errors.Is(err, tterrors.ErrParseError) {
				return nil, fmt.Errorf("corrupted tasks file in %s: %w", project.Name, err)
			}
			return nil, fmt.Errorf("failed to load project %s: %w", project.Name, err)
		}
		Debugf("loaded %d tasks", len(project.Tasks))
		for _, warning := range result.Warnings {
			switch warning.Type {
			case format.WarningDataCorrection, format.WarningDeprecation:
				fmt.Fprintf(os.Stderr, "warning: %s\n", warning.Message)
			default:
				Debugf("parser warning: %s", warning.Message)
			}
		}
	}
	return projects, nil
}

// cloneCLIWorkspaceDomains returns a defensive copy of the command-start
// domains. Mutations are applied to the copy so a failed checked commit never
// corrupts the cached RuntimeState used by the rest of the command.
func cloneCLIWorkspaceDomains(workspace *CLIWorkspace) (*config.Domains, error) {
	if workspace == nil || workspace.Runtime == nil {
		return nil, fmt.Errorf("runtime state is unavailable")
	}
	return workspace.Runtime.CloneDomains()
}

func commitCLIPlanning(workspace *CLIWorkspace, planning *config.PlanningState) error {
	if err := workspace.Runtime.CommitPlanning(planning); err != nil {
		return tterrors.NewSaveConfigError(err)
	}
	return nil
}

func commitCLIAutomation(workspace *CLIWorkspace, automation *config.AutomationConfig) error {
	if err := workspace.Runtime.CommitAutomation(automation); err != nil {
		return tterrors.NewSaveConfigError(err)
	}
	return nil
}

func commitCLISettings(workspace *CLIWorkspace, settings *config.Settings) error {
	if err := workspace.Runtime.CommitPortableSettings(settings); err != nil {
		return tterrors.NewSaveConfigError(err)
	}
	return nil
}
