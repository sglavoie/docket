package storage

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/storage/format"
)

func newV5TaskCandidate(
	projectID string,
	sourceID string,
	sourcePath string,
	ordinal int,
	record format.V5TaskRecord,
	idRemap map[string]string,
) (*v5TaskCandidate, error) {
	task := cloneV5TaskRecord(record)
	taskID := strings.TrimSpace(task.ID)
	if taskID == "" {
		return nil, fmt.Errorf("v5 task reintegration project %q source %q task %d has an empty ID", projectID, sourceID, ordinal+1)
	}
	task.ID = taskID
	candidate := &v5TaskCandidate{
		projectID:  projectID,
		sourceID:   sourceID,
		sourcePath: sourcePath,
		ordinal:    ordinal,
		oldShortID: ordinal + 1,
		task:       task,
	}

	if remapped := resolveV5StableID(task.ID, idRemap); remapped != task.ID {
		candidate.remapEntries = append(candidate.remapEntries, V5TaskRepairEntry{
			ProjectID: projectID,
			TaskID:    remapped,
			Action:    "remapped",
			Field:     "stable_id",
			Source:    sourceID,
			From:      task.ID,
			To:        remapped,
			Reason:    "caller supplied a validated stable-ID mapping",
		})
		candidate.task.ID = remapped
	}
	if candidate.task.ParentID != "" {
		if remapped := resolveV5StableID(candidate.task.ParentID, idRemap); remapped != candidate.task.ParentID {
			candidate.remapEntries = append(candidate.remapEntries, V5TaskRepairEntry{
				ProjectID: projectID,
				TaskID:    candidate.task.ID,
				Action:    "remapped",
				Field:     "parent_id",
				Source:    sourceID,
				From:      candidate.task.ParentID,
				To:        remapped,
				Reason:    "parent reference follows the validated stable-ID mapping",
			})
			candidate.task.ParentID = remapped
		}
	}
	for index, dependencyRef := range candidate.task.DependsOn {
		qualifier, dependencyID := splitV5TaskReference(dependencyRef)
		if remapped := resolveV5StableID(dependencyID, idRemap); remapped != dependencyID {
			remappedRef := joinV5TaskReference(qualifier, remapped)
			candidate.remapEntries = append(candidate.remapEntries, V5TaskRepairEntry{
				ProjectID: projectID,
				TaskID:    candidate.task.ID,
				Action:    "remapped",
				Field:     "depends_on",
				Source:    sourceID,
				From:      dependencyRef,
				To:        remappedRef,
				Reason:    "dependency reference follows the validated stable-ID mapping",
			})
			candidate.task.DependsOn[index] = remappedRef
		}
	}
	return candidate, nil
}

func splitV5TaskReference(ref string) (string, string) {
	index := strings.Index(ref, ":")
	if index <= 0 || index == len(ref)-1 {
		return "", ref
	}
	return ref[:index], ref[index+1:]
}

func joinV5TaskReference(qualifier, taskID string) string {
	if qualifier == "" {
		return taskID
	}
	return qualifier + ":" + taskID
}

func canonicalV5DependencyReference(ownerProjectID, targetProjectID, taskID string) string {
	if ownerProjectID == targetProjectID {
		return taskID
	}
	return joinV5TaskReference(targetProjectID, taskID)
}

func validateV5StableIDRemap(input map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	remap := make(map[string]string, len(input))
	for oldID, newID := range input {
		oldID = strings.TrimSpace(oldID)
		newID = strings.TrimSpace(newID)
		if oldID == "" || newID == "" {
			return nil, fmt.Errorf("v5 task reintegration stable-ID remap contains an empty ID")
		}
		if existing, ok := remap[oldID]; ok && existing != newID {
			return nil, fmt.Errorf("v5 task reintegration stable-ID %q has conflicting remaps", oldID)
		}
		remap[oldID] = newID
	}
	for oldID := range remap {
		if _, err := resolveV5StableIDChecked(oldID, remap); err != nil {
			return nil, err
		}
	}
	return remap, nil
}

func resolveV5StableID(id string, remap map[string]string) string {
	resolved, _ := resolveV5StableIDChecked(id, remap)
	return resolved
}

func resolveV5StableIDChecked(id string, remap map[string]string) (string, error) {
	seen := make(map[string]bool)
	current := id
	for {
		next, ok := remap[current]
		if !ok || next == current {
			return current, nil
		}
		if seen[current] {
			return "", fmt.Errorf("v5 task reintegration stable-ID remap contains a cycle at %q", current)
		}
		seen[current] = true
		current = next
	}
}

func validateV5ArchiveCleanupPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("archive cleanup path is empty")
	}
	cleanPath := filepath.Clean(path)
	name := filepath.Base(cleanPath)
	if name != archiveFileName && !legacyV4RotatedArchiveName.MatchString(name) {
		return "", fmt.Errorf("archive cleanup path %q is not completed.md or completed.N.md", path)
	}
	return cleanPath, nil
}

func validateV5ArchiveIndexCleanupPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("archive index cleanup path is empty")
	}
	cleanPath := filepath.Clean(path)
	if filepath.Base(cleanPath) != archiveIndexFileName {
		return "", fmt.Errorf("archive index cleanup path %q is not %s", path, archiveIndexFileName)
	}
	return cleanPath, nil
}

func cloneV5TaskRecord(task format.V5TaskRecord) format.V5TaskRecord {
	cloned := task
	cloned.Tags = append([]string(nil), task.Tags...)
	cloned.DependsOn = append([]string(nil), task.DependsOn...)
	cloned.Attachments = append([]format.V5TaskAttachment(nil), task.Attachments...)
	cloned.Checklist = append([]format.V5ChecklistItem(nil), task.Checklist...)
	cloned.Comments = append([]format.V5TaskComment(nil), task.Comments...)
	cloned.Decisions = append([]format.V5TaskDecision(nil), task.Decisions...)
	if task.Recurrence != nil {
		recurrence := *task.Recurrence
		cloned.Recurrence = &recurrence
	}
	if task.CompletedAt != nil {
		completedAt := *task.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	if task.PinnedAt != nil {
		pinnedAt := *task.PinnedAt
		cloned.PinnedAt = &pinnedAt
	}
	return cloned
}
