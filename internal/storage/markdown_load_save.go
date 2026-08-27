package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// LoadProject loads tasks from a project's tasks.md file. Malformed hierarchy
// data does not survive as a repaired tree: ParseProjectDocument rejects a
// missing project-local parent, a parent cycle, and a depth overflow before the
// tree is built, so a corrupt file is a load failure rather than a silent
// in-memory rewrite.
func LoadProject(project *models.Project) error {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		if err := DefaultRepository.Load(project); err != nil {
			return err
		}
		return auditLoadedProject(project)
	}

	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		return loadMissingProject(project)
	}
	if err != nil {
		return fmt.Errorf("failed to read tasks file: %w", err)
	}
	return loadProjectContent(project, content)
}

// LoadProjectForRewrite loads the exact tasks.md bytes that a caller is about to
// render as a full-file replacement and returns their raw optimistic-concurrency
// revision. A missing file is represented by a non-nil pointer to the empty
// revision, which means the eventual write must still observe the file as absent.
//
// Filesystem-backed loads parse the same byte slice whose hash is returned, so a
// lifecycle planner can never render one version while carrying the revision of
// another. Test repositories still load through their injected implementation;
// their revision contract reflects any physical shadow tasks.md when present and
// otherwise requires absence.
func LoadProjectForRewrite(project *models.Project) (*string, error) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		if err := LoadProject(project); err != nil {
			return nil, err
		}
		content, err := os.ReadFile(tasksFilePath(project.Path))
		if os.IsNotExist(err) {
			expected := ""
			return &expected, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tasks file for rewrite: %w", err)
		}
		expected := RawArtifactRevision(content)
		return &expected, nil
	}

	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		if loadErr := loadMissingProject(project); loadErr != nil {
			return nil, loadErr
		}
		expected := ""
		return &expected, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file for rewrite: %w", err)
	}
	if err := loadProjectContent(project, content); err != nil {
		return nil, err
	}
	expected := RawArtifactRevision(content)
	return &expected, nil
}

// loadMissingProject treats an absent tasks file as a successful load of an
// empty project.
func loadMissingProject(project *models.Project) error {
	project.Tasks = []*models.Task{}
	project.NextShortID = 1
	return auditLoadedProject(project)
}

func loadProjectContent(project *models.Project, content []byte) error {
	document, err := ParseProjectDocument(string(content))
	if err != nil {
		return fmt.Errorf("%w: schema-v6 task document required: %w", tterrors.ErrParseError, err)
	}
	project.Tasks = document.Tasks
	maxShortID := 0
	for _, task := range project.Tasks {
		if task.ShortID > maxShortID {
			maxShortID = task.ShortID
		}
	}
	project.NextShortID = maxShortID + 1
	project.BuildTaskTree()
	return auditLoadedProject(project)
}

func auditLoadedProject(project *models.Project) error {
	if err := models.AuditProject(project); err != nil {
		return fmt.Errorf("%w: project audit failed after load normalization: %w", tterrors.ErrParseError, err)
	}

	return nil
}

func auditProjectBeforeSave(project *models.Project) error {
	if err := models.AuditProject(project); err != nil {
		projectName := "<unknown>"
		if project != nil && project.Name != "" {
			projectName = project.Name
		}
		return tterrors.NewSaveProjectError(projectName, fmt.Errorf("project audit failed before save: %w", err))
	}

	return nil
}

// SaveProject saves a project's tasks to its tasks.md file.
func SaveProject(project *models.Project) error {
	if err := auditProjectBeforeSave(project); err != nil {
		return err
	}
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		return DefaultRepository.Save(project)
	}
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	tasksPath := tasksFilePath(project.Path)
	document, err := CanonicalProjectDocument(project, nil)
	if err != nil {
		return fmt.Errorf("failed to render schema-v6 tasks file: %w", err)
	}
	content, err := document.WriteCurrent()
	if err != nil {
		return fmt.Errorf("failed to render schema-v6 tasks file: %w", err)
	}
	if err := writeFileAtomic(tasksPath, []byte(content), FilePermissions); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// SaveProjectLocked saves a project's tasks to its tasks.md file, acquiring
// an exclusive file lock first. Use this for call sites that don't already
// hold their own lock.
func SaveProjectLocked(project *models.Project) error {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		return SaveProject(project)
	}
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	lockPath := filepath.Join(project.Path, LockFileName)
	return util.WithFileLock(lockPath, true, func() error {
		return SaveProject(project)
	})
}

// projectExists checks if a project directory and tasks file exist.
func projectExists(projectPath string) bool {
	_, err := os.Stat(tasksFilePath(projectPath))
	return err == nil
}

// CreateProjectDir creates the project directory structure.
func CreateProjectDir(project *models.Project) error {
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	return SaveProject(project)
}

// deleteProjectDir removes a project's directory.
func deleteProjectDir(projectPath string) error {
	return os.RemoveAll(projectPath)
}
