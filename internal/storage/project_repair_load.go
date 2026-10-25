package storage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// LoadProjectForRepair is LoadProjectForRewrite for `tt doctor`, and for it
// alone: it admits exactly the two task defects doctor --fix repairs — a
// duplicated project-local short ID and a parent reference naming no task in
// the project — and keeps them in the loaded model, so doctor reports each one
// before it repairs it. Every other invariant is still judged in full, so a
// file this admits differs from one the strict reader admits by those two
// defects and nothing else. A healthy file loads exactly as LoadProjectForRewrite
// loads it. The returned raw revision guards the repair commit, as it does for
// any other rewrite.
//
// Normal loading stays strict: nothing but doctor may call this, because every
// other surface assumes the model the canonical writer would accept.
func LoadProjectForRepair(project *models.Project) (*string, error) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		expected, err := LoadProjectForRewrite(project)
		if err != nil {
			return nil, err
		}
		project.BuildTaskTree()
		return expected, nil
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
		return nil, fmt.Errorf("failed to read tasks file for repair: %w", err)
	}
	expected := RawArtifactRevision(content)
	strictErr := loadProjectContent(project, content)
	if strictErr == nil {
		return &expected, nil
	}
	if repairErr := loadRepairableProjectContent(project, content); repairErr != nil {
		return nil, strictErr
	}
	return &expected, nil
}

// loadRepairableProjectContent loads a document whose only defects are the
// repairable ones. The document is validated with those defects neutralized on
// a copy, while the model is built from the records as stored.
func loadRepairableProjectContent(project *models.Project, content []byte) error {
	document, err := scanCurrentTaskProjectDocument(strings.TrimPrefix(string(content), "\uFEFF"))
	if err != nil {
		return err
	}
	neutral, brokenParents := neutralizeRepairableDefects(document)
	if validateErr := validateCurrentTaskProjectDocument(neutral); validateErr != nil {
		return validateErr
	}
	neutralTasks, err := modelsFromCurrentTaskDocument(neutral, time.Local)
	if err != nil {
		return err
	}
	tasks, err := modelsFromCurrentTaskDocument(document, time.Local)
	if err != nil {
		return err
	}
	comments, err := projectCommentsFromCurrentDocument(document)
	if err != nil {
		return err
	}
	if auditErr := models.AuditProject(&models.Project{Name: project.Name, Tasks: neutralTasks, Comments: comments}); auditErr != nil {
		return auditErr
	}

	project.Tasks = tasks
	project.Comments = comments
	maxShortID := 0
	for _, task := range tasks {
		maxShortID = max(maxShortID, task.ShortID)
	}
	project.NextShortID = maxShortID + 1
	// BuildTaskTree treats a missing parent as a root by clearing the reference,
	// which would hide the defect from the check that reports it. The tree still
	// treats those tasks as roots; only the stored reference is put back.
	project.BuildTaskTree()
	for _, task := range tasks {
		if parentID, broken := brokenParents[task.ID]; broken {
			task.ParentID = &parentID
		}
	}
	return nil
}

// neutralizeRepairableDefects returns a copy of document with each duplicated
// short ID moved past the highest one in use and each missing parent cleared,
// plus the missing parent each affected task named.
func neutralizeRepairableDefects(document currentTaskProjectDocument) (currentTaskProjectDocument, map[string]string) {
	neutral := document
	neutral.Tasks = append(neutral.Tasks[:0:0], document.Tasks...)
	present := make(map[string]bool, len(neutral.Tasks))
	nextShortID := 0
	for _, task := range neutral.Tasks {
		present[task.ID] = true
		nextShortID = max(nextShortID, task.ShortID)
	}
	brokenParents := make(map[string]string)
	seenShortIDs := make(map[int]bool, len(neutral.Tasks))
	for i := range neutral.Tasks {
		task := &neutral.Tasks[i]
		if seenShortIDs[task.ShortID] {
			nextShortID++
			task.ShortID = nextShortID
		}
		seenShortIDs[task.ShortID] = true
		if task.ParentID != "" && !present[task.ParentID] {
			brokenParents[task.ID] = task.ParentID
			task.ParentID = ""
		}
	}
	return neutral, brokenParents
}
