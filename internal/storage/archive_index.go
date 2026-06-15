package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

type archiveFileRef struct {
	fileName string
	path     string
	order    int
}

func archiveIndexPath(project *models.Project) string {
	return filepath.Join(project.Path, archiveIndexFileName)
}

func listArchiveFiles(projectPath string) ([]archiveFileRef, error) {
	var files []archiveFileRef

	mainPath := filepath.Join(projectPath, archiveFileName)
	if _, err := os.Stat(mainPath); err == nil {
		files = append(files, archiveFileRef{fileName: archiveFileName, path: mainPath, order: -1})
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat archive file: %w", err)
	}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, fmt.Errorf("failed to read archive directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "completed.") || !strings.HasSuffix(name, ".md") {
			continue
		}
		numStr := strings.TrimPrefix(name, "completed.")
		numStr = strings.TrimSuffix(numStr, ".md")
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		files = append(files, archiveFileRef{
			fileName: name,
			path:     filepath.Join(projectPath, name),
			order:    n,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].order != files[j].order {
			return files[i].order < files[j].order
		}
		return files[i].fileName < files[j].fileName
	})

	return files, nil
}

// LoadArchivedTaskSummaries returns compact summaries for archived tasks.
// Missing, corrupt, old-version, and stale indexes are rebuilt from canonical
// archive markdown before summaries are returned.
func LoadArchivedTaskSummaries(project *models.Project) ([]ArchiveTaskSummary, error) {
	index, err := readArchiveIndex(project)
	if err == nil && archiveIndexIsFresh(project, index) {
		return append([]ArchiveTaskSummary(nil), index.Tasks...), nil
	}

	index, rebuildErr := rebuildArchiveIndex(project)
	if rebuildErr != nil {
		if err != nil {
			return nil, fmt.Errorf("failed to load archive index: %w; rebuild failed: %w", err, rebuildErr)
		}
		return nil, rebuildErr
	}

	return append([]ArchiveTaskSummary(nil), index.Tasks...), nil
}

// RebuildArchiveIndex rebuilds the derived archive summary index from
// completed.md and completed.N.md. The archive markdown remains canonical.
func RebuildArchiveIndex(project *models.Project) error {
	_, err := rebuildArchiveIndex(project)
	return err
}

// InspectArchiveIndex reports whether the derived archive summary index is
// present and fresh without rebuilding or parsing full archive task bodies.
func InspectArchiveIndex(project *models.Project) (ArchiveIndexStatus, error) {
	status := ArchiveIndexStatus{IndexPath: archiveIndexPath(project)}

	archiveFiles, err := listArchiveFiles(project.Path)
	if err != nil {
		return status, err
	}
	status.ArchiveFileCount = len(archiveFiles)
	status.HasArchives = len(archiveFiles) > 0

	index, err := readArchiveIndex(project)
	if err != nil {
		if os.IsNotExist(err) {
			if !status.HasArchives {
				status.Fresh = true
				return status, nil
			}
			status.NeedsRebuild = true
			status.Issue = ArchiveIndexIssueMissing
			status.Detail = "archive index is missing"
			return status, nil
		}
		status.NeedsRebuild = true
		status.Issue = ArchiveIndexIssueCorrupt
		status.Detail = err.Error()
		return status, nil
	}

	status.IndexExists = true
	status.SourceCount = len(index.Sources)
	status.TaskCount = len(index.Tasks)
	if archiveIndexIsFresh(project, index) {
		status.Fresh = true
		return status, nil
	}

	status.NeedsRebuild = true
	status.Issue = ArchiveIndexIssueStale
	status.Detail = "archive files changed since the index was generated"
	return status, nil
}

func readArchiveIndex(project *models.Project) (*ArchiveIndex, error) {
	data, err := os.ReadFile(archiveIndexPath(project))
	if err != nil {
		return nil, err
	}

	var index ArchiveIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	return &index, nil
}

func rebuildArchiveIndex(project *models.Project) (*ArchiveIndex, error) {
	archiveFiles, err := listArchiveFiles(project.Path)
	if err != nil {
		return nil, err
	}

	index := &ArchiveIndex{
		Version:     archiveIndexVersion,
		GeneratedAt: time.Now(),
		Sources:     make([]ArchiveIndexSource, 0, len(archiveFiles)),
	}

	for _, archiveFile := range archiveFiles {
		source, sourceErr := archiveIndexSource(archiveFile)
		if sourceErr != nil {
			if os.IsNotExist(sourceErr) {
				continue
			}
			return nil, sourceErr
		}

		tasks, headingLines, parseErr := parseArchiveFileWithHeadingLines(archiveFile.path)
		if parseErr != nil {
			logging.Warn("skipping corrupt archive %s while rebuilding index: %v", archiveFile.path, parseErr)
			index.Sources = append(index.Sources, source)
			continue
		}

		source.TaskCount = len(tasks)
		index.Sources = append(index.Sources, source)
		for i, task := range tasks {
			headingLine := 0
			if i < len(headingLines) {
				headingLine = headingLines[i]
			}
			index.Tasks = append(index.Tasks, summarizeArchivedTask(task, archiveFile.fileName, headingLine))
		}
	}

	if mkdirErr := os.MkdirAll(project.Path, DirPermissions); mkdirErr != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", mkdirErr)
	}

	payload, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode archive index: %w", err)
	}
	payload = append(payload, '\n')

	if err := writeFileAtomic(archiveIndexPath(project), payload, FilePermissions); err != nil {
		return nil, fmt.Errorf("failed to write archive index: %w", err)
	}

	return index, nil
}

func archiveIndexSource(archiveFile archiveFileRef) (ArchiveIndexSource, error) {
	info, err := os.Stat(archiveFile.path)
	if err != nil {
		return ArchiveIndexSource{}, fmt.Errorf("failed to stat archive source %s: %w", archiveFile.path, err)
	}
	return ArchiveIndexSource{
		FileName:  archiveFile.fileName,
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
	}, nil
}

func archiveIndexIsFresh(project *models.Project, index *ArchiveIndex) bool {
	if index == nil || index.Version != archiveIndexVersion {
		return false
	}

	archiveFiles, err := listArchiveFiles(project.Path)
	if err != nil || len(archiveFiles) != len(index.Sources) {
		return false
	}

	taskCounts := make(map[string]int, len(index.Sources))
	sourceNames := make(map[string]bool, len(index.Sources))
	for _, source := range index.Sources {
		if source.FileName == "" || source.TaskCount < 0 {
			return false
		}
		sourceNames[source.FileName] = true
	}
	for _, task := range index.Tasks {
		if task.ID == "" || task.Source.FileName == "" {
			return false
		}
		if !sourceNames[task.Source.FileName] {
			return false
		}
		taskCounts[task.Source.FileName]++
	}

	for i, archiveFile := range archiveFiles {
		current, err := archiveIndexSource(archiveFile)
		if err != nil {
			return false
		}
		cached := index.Sources[i]
		if cached.FileName != current.FileName ||
			cached.SizeBytes != current.SizeBytes ||
			!cached.ModTime.Equal(current.ModTime) ||
			cached.TaskCount != taskCounts[cached.FileName] {
			return false
		}
	}

	return true
}

func summarizeArchivedTask(task *models.Task, fileName string, headingLine int) ArchiveTaskSummary {
	summary := ArchiveTaskSummary{
		ID:             task.ID,
		ShortID:        task.ShortID,
		Type:           task.Type,
		Title:          task.Title,
		Status:         task.Status,
		Priority:       task.Priority,
		CreatedAt:      task.CreatedAt,
		CompletedAt:    completedAtForSummary(task),
		FirstStartedAt: firstStartedAt(task),
		EstimatedDays:  task.EstimatedDays,
		Effort:         taskEffort(task),
		StoryPoints:    taskStoryPoints(task),
		Source: ArchiveTaskSource{
			FileName:    fileName,
			HeadingLine: headingLine,
		},
	}
	return summary
}

func completedAtForSummary(task *models.Task) *time.Time {
	if task.CompletedAt != nil {
		return task.CompletedAt
	}
	if !task.Status.IsComplete() {
		return nil
	}
	fallback := task.UpdatedAt
	if fallback.IsZero() {
		fallback = task.CreatedAt
	}
	if fallback.IsZero() {
		return nil
	}
	return &fallback
}

func firstStartedAt(task *models.Task) *time.Time {
	var first *time.Time
	for _, entry := range task.Activity {
		if entry == nil || entry.Field != "status" || entry.NewValue != string(models.StatusInProgress) {
			continue
		}
		if first == nil || entry.Timestamp.Before(*first) {
			ts := entry.Timestamp
			first = &ts
		}
	}
	return first
}

func taskEffort(task *models.Task) models.EffortEstimate {
	if task.TaskAttrs != nil {
		return task.TaskAttrs.Effort
	}
	if task.IdeaAttrs != nil {
		return task.IdeaAttrs.Effort
	}
	return ""
}

func taskStoryPoints(task *models.Task) int {
	if task.StoryAttrs == nil {
		return 0
	}
	return task.StoryAttrs.StoryPoints
}

// FindArchivedTask resolves an archived task by short ID, exact ID, or ID
// prefix, then parses only that task's source archive file for full details.
func FindArchivedTask(project *models.Project, idOrPrefix string) (*models.Task, error) {
	query := strings.TrimSpace(idOrPrefix)
	if query == "" {
		return nil, tterrors.NewTaskNotFoundError(idOrPrefix)
	}

	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		return nil, err
	}

	summary, err := findArchivedSummary(summaries, query)
	if err != nil {
		return nil, err
	}

	task, err := loadArchivedTaskFromSummary(project, summary)
	if err == nil {
		return task, nil
	}

	// The stat-based index can race with external archive edits. Rebuild once
	// and retry before surfacing the lookup miss.
	if rebuildErr := RebuildArchiveIndex(project); rebuildErr != nil {
		return nil, fmt.Errorf("archived task source lookup failed: %w; rebuild failed: %w", err, rebuildErr)
	}
	summaries, err = LoadArchivedTaskSummaries(project)
	if err != nil {
		return nil, err
	}
	summary, err = findArchivedSummary(summaries, query)
	if err != nil {
		return nil, err
	}
	return loadArchivedTaskFromSummary(project, summary)
}

func findArchivedSummary(summaries []ArchiveTaskSummary, query string) (ArchiveTaskSummary, error) {
	if shortID, err := strconv.Atoi(query); err == nil {
		var matches []ArchiveTaskSummary
		for _, summary := range summaries {
			if summary.ShortID == shortID {
				matches = append(matches, summary)
			}
		}
		if len(matches) > 0 {
			return singleArchivedSummaryMatch(query, matches)
		}
	}

	lowerQuery := strings.ToLower(query)
	var exactMatches []ArchiveTaskSummary
	var prefixMatches []ArchiveTaskSummary
	for _, summary := range summaries {
		lowerID := strings.ToLower(summary.ID)
		if lowerID == lowerQuery {
			exactMatches = append(exactMatches, summary)
		}
		if strings.HasPrefix(lowerID, lowerQuery) {
			prefixMatches = append(prefixMatches, summary)
		}
	}
	if len(exactMatches) > 0 {
		return singleArchivedSummaryMatch(query, exactMatches)
	}
	return singleArchivedSummaryMatch(query, prefixMatches)
}

func singleArchivedSummaryMatch(query string, matches []ArchiveTaskSummary) (ArchiveTaskSummary, error) {
	switch len(matches) {
	case 0:
		return ArchiveTaskSummary{}, tterrors.NewTaskNotFoundError(query)
	case 1:
		return matches[0], nil
	default:
		return ArchiveTaskSummary{}, tterrors.NewAmbiguousIdentifierError(query, formatArchivedSummaryCandidates(matches))
	}
}

func formatArchivedSummaryCandidates(matches []ArchiveTaskSummary) string {
	limit := min(len(matches), 5)
	parts := make([]string, limit)
	for i := range limit {
		parts[i] = fmt.Sprintf("#%d %s (%s)", matches[i].ShortID, matches[i].Title, shortArchiveID(matches[i].ID))
	}
	candidates := strings.Join(parts, ", ")
	if len(matches) > limit {
		candidates += fmt.Sprintf(", ... (%d more)", len(matches)-limit)
	}
	return candidates
}

func shortArchiveID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func loadArchivedTaskFromSummary(project *models.Project, summary ArchiveTaskSummary) (*models.Task, error) {
	if summary.Source.FileName == "" {
		return nil, fmt.Errorf("archived task %s has no source file", summary.ID)
	}

	tasks, err := parseArchiveFile(filepath.Join(project.Path, filepath.Base(summary.Source.FileName)))
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if strings.EqualFold(task.ID, summary.ID) {
			return task, nil
		}
	}

	return nil, tterrors.NewTaskNotFoundError(summary.ID)
}
