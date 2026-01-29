package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
)

// Dependencies holds injectable dependencies for commands.
// This allows swapping implementations for testing.
type Dependencies struct {
	Selector   external.FuzzySelector
	Renderer   external.MarkdownRenderer
	Editor     external.Editor
	Repository storage.ProjectRepository
}

// Deps is the global dependencies instance used by commands.
// Tests can replace this with mock implementations.
var Deps = Dependencies{
	Selector:   external.DefaultSelector,
	Renderer:   external.DefaultRenderer,
	Editor:     external.DefaultEditor,
	Repository: storage.DefaultRepository,
}

// Debugf prints a formatted message to stderr if verbose mode is enabled.
func Debugf(format string, args ...any) {
	if IsVerbose() {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

// CommandContext bundles the commonly-needed config and project data for commands.
// This reduces duplicate loading code and makes it easier to pass context around.
type CommandContext struct {
	Project       *models.Project
	Config        *config.Config
	ConfigManager *config.Manager
}

// LoadConfig loads the configuration file.
// Returns the config, manager, and any error encountered.
func LoadConfig() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, configManager, nil
}

// LoadCurrentProject loads the config and current project with its tasks.
// Returns an error if no project is selected or if loading fails.
// Parser warnings are printed to stderr (always for data corrections, verbose mode for others).
func LoadCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return nil, nil, nil, fmt.Errorf("%w. Select one with: tt select", tterrors.ErrNoCurrentProject)
	}

	Debugf("loading project: %s (%s)", project.Name, project.Path)

	result, err := storage.LoadProjectWithWarnings(project)
	if err != nil {
		if errors.Is(err, tterrors.ErrParseError) {
			return nil, nil, nil, fmt.Errorf("corrupted tasks file in %s: %w", project.Name, err)
		}
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}

	Debugf("loaded %d tasks", len(project.Tasks))

	// Print warnings - always show data correction warnings, others only in verbose mode
	for _, w := range result.Warnings {
		if w.Type == format.WarningDataCorrection {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Message)
		} else {
			Debugf("parser warning: %s", w.Message)
		}
	}

	return project, cfg, configManager, nil
}
