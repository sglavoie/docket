package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// LoadProject loads tasks from a project's tasks.md file.
func LoadProject(project *models.Project) error {
	_, err := LoadProjectWithWarnings(project)
	return err
}

// LoadProjectWithWarnings loads tasks from a project's tasks.md file and returns any warnings.
func LoadProjectWithWarnings(project *models.Project) (*ProjectLoadResult, error) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		if err := DefaultRepository.Load(project); err != nil {
			return nil, err
		}
		if err := auditLoadedProject(project); err != nil {
			return nil, err
		}
		return &ProjectLoadResult{}, nil
	}

	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		return loadMissingProject(project)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
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
		if _, loadErr := loadMissingProject(project); loadErr != nil {
			return nil, loadErr
		}
		expected := ""
		return &expected, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file for rewrite: %w", err)
	}
	// Full-file lifecycle rewrites must retain every parsed cross-project
	// reference until the caller deliberately rewrites it. Ordinary loads may
	// prune a legacy slug whose project directory is gone, but that validation
	// cannot resolve stable project-ID qualifiers from filesystem paths and would
	// otherwise hide them from identity-based rename/delete cleanup.
	if _, err := loadProjectContentForRewrite(project, content); err != nil {
		return nil, err
	}
	expected := RawArtifactRevision(content)
	return &expected, nil
}

func loadProjectContentForRewrite(project *models.Project, content []byte) (*ProjectLoadResult, error) {
	return loadProjectContentWithCrossRefValidation(project, content, false)
}

func loadMissingProject(project *models.Project) (*ProjectLoadResult, error) {
	project.Tasks = []*models.Task{}
	project.NextShortID = 1
	project.Sections = []string{models.SectionNext}
	project.Tags = []string{}
	project.PinnedTags = []string{}
	if auditErr := auditLoadedProject(project); auditErr != nil {
		return nil, auditErr
	}
	// A missing tasks file is a successful load of an empty project; return a
	// non-nil (empty) migration audit so callers can rely on it being present
	// whenever the load succeeds.
	return &ProjectLoadResult{MigrationAudit: NewMigrationAudit(project.ID, nil)}, nil
}

func loadProjectContent(project *models.Project, content []byte) (*ProjectLoadResult, error) {
	return loadProjectContentWithCrossRefValidation(project, content, true)
}

func loadProjectContentWithCrossRefValidation(project *models.Project, content []byte, validateCrossRefs bool) (*ProjectLoadResult, error) {
	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", tterrors.ErrParseError, err)
	}

	project.Tasks = result.Tasks
	project.Sections = result.Sections
	project.Tags = result.Tags
	project.PinnedTags = result.PinnedTags
	project.SectionColors = result.SectionColors

	maxShortID := 0
	for _, task := range result.Tasks {
		if task.ShortID > maxShortID {
			maxShortID = task.ShortID
		}
	}
	project.NextShortID = maxShortID + 1

	project.EnsureDefaultSections()
	project.BuildTaskTree()

	if validateCrossRefs {
		validateCrossProjectRefs(project, result)
	}
	populateProjectWarnings(project, result.Warnings)
	if err := auditLoadedProject(project); err != nil {
		return nil, err
	}

	return &ProjectLoadResult{
		Warnings:       result.Warnings,
		MigrationAudit: NewMigrationAudit(project.ID, result.MigrationAudit),
	}, nil
}

func validateCrossProjectRefs(project *models.Project, result *ParseResult) {
	dataDir := filepath.Dir(filepath.Dir(project.Path))
	for _, task := range project.Tasks {
		origDeps := len(task.DependsOn)
		task.DependsOn = filterProjectRefs(task.DependsOn, dataDir, "depends", task.ID, func(projectID string) string {
			return fmt.Sprintf("task %s has dependency on non-existent project %q, removing", task.ID, projectID)
		}, &result.Warnings)
		// A waiting task whose only remaining deps were cross-project refs to
		// now-deleted projects has no resolvable reason to stay waiting. Reopen
		// it so it is not silently stuck on load.
		if task.IsInState(models.StateWaiting) && origDeps > 0 && len(task.DependsOn) == 0 {
			task.Reopen()
		}

		task.RelatedTo = filterProjectRefs(task.RelatedTo, dataDir, "related", task.ID, func(projectID string) string {
			return fmt.Sprintf("task %s has related ref to non-existent project %q, removing", task.ID, projectID)
		}, &result.Warnings)
	}
}

func filterProjectRefs(refs []string, dataDir, field, taskID string, warningMessage func(string) string, warnings *[]ParseWarning) []string {
	if len(refs) == 0 {
		return refs
	}
	var kept []string
	for _, refValue := range refs {
		ref := models.ParseTaskRef(refValue)
		if ref.IsCrossProject() {
			projectDir := filepath.Join(dataDir, "projects", ref.ProjectID)
			if _, err := os.Stat(projectDir); os.IsNotExist(err) {
				*warnings = append(*warnings, ParseWarning{
					Type:    format.WarningDataCorrection,
					TaskID:  taskID,
					Field:   field,
					Message: warningMessage(ref.ProjectID),
				})
				continue
			}
		}
		kept = append(kept, refValue)
	}
	return kept
}

func populateProjectWarnings(project *models.Project, warnings []ParseWarning) {
	var repairWarnings []string
	for _, warning := range warnings {
		if warning.Type == format.WarningDataCorrection {
			repairWarnings = append(repairWarnings, warning.Message)
		}
	}
	project.Warnings = repairWarnings
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
//
// Note: There is a TOCTOU (time-of-check-time-of-use) race between the
// old-content read (for activity diffing) and the atomic write. If another
// process modifies tasks.md between those two operations, the diff may be
// inaccurate and the write will silently overwrite those changes. This is
// acceptable because tt is a single-user CLI tool — concurrent writes to
// the same project are not an expected use case. If multi-process access
// is ever needed, advisory file locking (e.g., flock) should be added.
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
	if oldContent, err := os.ReadFile(tasksPath); err == nil {
		if oldResult, parseErr := ParseTasksMarkdownWithSections(string(oldContent)); parseErr == nil {
			diffAndRecordActivity(oldResult.Tasks, project.Tasks, MaxActivityEntries)
		} else {
			logging.Warn("could not diff activity for %s: %v", project.Name, parseErr)
		}
	}

	content := WriteTasksMarkdownWithFullProjectMetadata(project.Name, project.Tasks, project.GetSections(), project.SectionColors, project.Tags, project.PinnedTags)
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
