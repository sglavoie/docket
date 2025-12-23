package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
func LoadCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return nil, nil, nil, fmt.Errorf("no current project selected. Select one with: tt select")
	}

	if err := storage.LoadProject(project); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}

	return project, cfg, configManager, nil
}

// SaveProjectWithBackup saves the project after creating a backup.
// It also runs auto-unblock checks and reorders root tasks by completion status.
func SaveProjectWithBackup(project *models.Project) error {
	project.CheckAndAutoUnblock()
	project.ReorderRootTasksByCompletion()

	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return storage.SaveProject(project)
}

// FindTaskByIdentifier finds a task by short ID, UUID, or title prefix.
// Returns nil if no matching task is found.
func FindTaskByIdentifier(project *models.Project, identifier string) *models.Task {
	// Try as short ID first
	var shortID int
	if _, err := fmt.Sscanf(identifier, "%d", &shortID); err == nil {
		if task := project.GetTaskByShortID(shortID); task != nil {
			return task
		}
	}

	// Try as UUID
	if task := project.GetTaskByID(identifier); task != nil {
		return task
	}

	// Try as title prefix (case-insensitive)
	lowerID := strings.ToLower(identifier)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), lowerID) {
			return task
		}
	}

	return nil
}

// ParseTaskTypeFlags determines the task type from boolean flags or a string flag.
// Returns an error if the type flag is invalid.
func ParseTaskTypeFlags(epic, story, bug, idea, note bool, typeFlag string) (models.TaskType, error) {
	if epic {
		return models.TaskTypeEpic, nil
	}
	if story {
		return models.TaskTypeStory, nil
	}
	if bug {
		return models.TaskTypeBug, nil
	}
	if idea {
		return models.TaskTypeIdea, nil
	}
	if note {
		return models.TaskTypeNote, nil
	}
	if typeFlag != "" {
		t, ok := models.ParseTaskType(typeFlag)
		if !ok {
			return "", fmt.Errorf("invalid task type: %s", typeFlag)
		}
		return t, nil
	}
	return models.TaskTypeTask, nil
}
