package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/logging"
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
		logging.Warn("[debug] "+format, args...)
	}
}

// Pluralize returns "1 task" or "3 tasks" based on count.
func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// CommandContext bundles the commonly-needed config and project data for commands.
// This reduces duplicate loading code and makes it easier to pass context around.
type CommandContext struct {
	Project       *models.Project
	Config        *config.Config
	ConfigManager *config.Manager
}

// ConfirmYN prompts the user with a [y/N] question. Returns true if the user
// answers y or yes (case-insensitive). When force is true, skips the prompt and
// returns true immediately.
func ConfirmYN(prompt string, force bool) bool {
	if force {
		return true
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false // safe default: abort on I/O error
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// ConfirmStrict prompts the user to type "yes" to confirm a destructive action.
// Returns true only if the user types exactly "yes". When force is true, skips
// the prompt and returns true immediately.
func ConfirmStrict(prompt string, force bool) bool {
	if force {
		return true
	}
	fmt.Println(prompt)
	fmt.Print("Type 'yes' to confirm: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false // safe default: abort on I/O error
	}
	return strings.TrimSpace(strings.ToLower(response)) == "yes"
}

// LoadConfig loads the configuration file, creating it if it does not exist.
// Returns the config, manager, and any error encountered.
func LoadConfig() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	// Apply config-driven storage settings
	if cfg.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *cfg.MaxActivityEntries
	}
	return cfg, configManager, nil
}

// LoadConfigStrict loads the configuration file, returning an error if it does
// not exist. Use this instead of LoadConfig when the config must already exist
// (e.g., workflow and template commands that operate on existing config data).
func LoadConfigStrict() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *cfg.MaxActivityEntries
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
		return nil, nil, nil, fmt.Errorf("%w. Select one with: tt select, or specify with --project", tterrors.ErrNoCurrentProject)
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

	// Print warnings to stderr (data corrections always, deprecations always, validation in verbose mode)
	for _, w := range result.Warnings {
		switch w.Type {
		case format.WarningDataCorrection, format.WarningDeprecation:
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Message)
		default:
			Debugf("parser warning: %s", w.Message)
		}
	}

	return project, cfg, configManager, nil
}

// FindActionTaskByID searches all loadable projects for an Action task by short ID or UUID prefix.
// Returns the task and its project, or nil, nil if not found.
func FindActionTaskByID(cfg *config.Config, query string) (*models.Task, *models.Project) {
	var shortID int
	isShortID := false
	if _, err := fmt.Sscanf(query, "%d", &shortID); err == nil {
		isShortID = true
	}

	for _, project := range cfg.GetAllLoadableProjects() {
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
			continue
		}
		for _, task := range project.Tasks {
			if task.Type != models.TaskTypeAction {
				continue
			}
			if isShortID && task.ShortID == shortID {
				return task, project
			}
			if !isShortID && strings.HasPrefix(task.ID, query) {
				return task, project
			}
		}
	}
	return nil, nil
}

// LoadConfigAndProject loads the config and looks up a project by slug.
// Returns an error if the project is not found.
func LoadConfigAndProject(slug string) (*models.Project, *config.Config, *config.Manager, error) {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	project := cfg.GetProject(slug)
	if project == nil {
		return nil, nil, nil, tterrors.NewProjectNotFoundError(slug)
	}
	return project, cfg, mgr, nil
}

// LoadConfigAndProjectOrCurrent loads the config and looks up a project by slug
// if args is non-empty, otherwise falls back to the current project.
func LoadConfigAndProjectOrCurrent(args []string) (*models.Project, *config.Config, *config.Manager, error) {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	var project *models.Project
	if len(args) > 0 {
		project = cfg.GetProject(args[0])
		if project == nil {
			return nil, nil, nil, tterrors.NewProjectNotFoundError(args[0])
		}
	} else {
		project = cfg.GetCurrentProject()
		if project == nil {
			return nil, nil, nil, fmt.Errorf("%w. Select one with: tt select, or specify with --project", tterrors.ErrNoCurrentProject)
		}
	}
	return project, cfg, mgr, nil
}
