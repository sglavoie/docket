package storage

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// performRestore implements the shared undo/redo algorithm:
// read the most recent source snapshot, capture the current file state as a
// reverse snapshot, apply the restore (targeted or full-file), then clean up.
func performRestore(project *models.Project, meta *UndoMeta, p restoreParams) (string, error) {
	if len(*p.sourceEntries) == 0 {
		return "", fmt.Errorf("no %s history available", p.label)
	}

	// Sort by num descending to get most recent
	sort.Slice(*p.sourceEntries, func(i, j int) bool {
		return (*p.sourceEntries)[i].Num > (*p.sourceEntries)[j].Num
	})

	latest := (*p.sourceEntries)[0]
	originalSourceEntries := cloneUndoEntrySlice(*p.sourceEntries)
	originalReverseEntries := cloneUndoEntrySlice(*p.reverseEntries)

	// Read and validate the snapshot
	snapPath := p.snapshotPath(project, latest.Num)
	snapBytes, err := os.ReadFile(snapPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s snapshot %d: %w", p.label, latest.Num, err)
	}
	snapContent := string(snapBytes)

	// Capture current state as a reverse snapshot
	tasksPath := GetTasksFilePath(project)
	currentContent, err := os.ReadFile(tasksPath)
	if err != nil {
		return "", fmt.Errorf("failed to read current state for %s reverse snapshot: %w", p.label, err)
	}

	nextNum := 1
	for _, e := range *p.reverseEntries {
		if e.Num >= nextNum {
			nextNum = e.Num + 1
		}
	}
	revPath := p.reversePath(project, nextNum)
	if writeErr := util.AtomicWriteFile(revPath, currentContent, FilePermissions); writeErr != nil {
		return "", fmt.Errorf("failed to write %s reverse snapshot: %w", p.label, writeErr)
	}
	*p.reverseEntries = append(*p.reverseEntries, newReverseUndoEntry(latest, nextNum))

	// Determine what to write: targeted restore or full-file restore
	restoreContent, err := buildRestoreContent(p.label, string(currentContent), snapContent, latest)
	if err != nil {
		*p.reverseEntries = originalReverseEntries
		_ = os.Remove(revPath)
		return "", err
	}

	if err := util.AtomicWriteFile(tasksPath, []byte(restoreContent), FilePermissions); err != nil {
		*p.reverseEntries = originalReverseEntries
		_ = os.Remove(revPath)
		return "", fmt.Errorf("failed to restore from %s snapshot: %w", p.label, err)
	}

	// Remove consumed entry and save meta BEFORE deleting the snapshot file.
	*p.sourceEntries = (*p.sourceEntries)[1:]
	if err := saveUndoMeta(project, meta); err != nil {
		*p.sourceEntries = originalSourceEntries
		*p.reverseEntries = originalReverseEntries
		if rbErr := util.AtomicWriteFile(tasksPath, currentContent, FilePermissions); rbErr != nil {
			logging.Warn("failed to rollback %s restore after metadata save failure: %v", p.label, rbErr)
		}
		_ = os.Remove(revPath)
		return "", err
	}

	// Delete consumed snapshot (cleanup — safe to fail)
	_ = os.Remove(snapPath)

	return undoActionDescription(latest), nil
}

// PerformUndo restores the most recent snapshot and updates the history.
// Returns the action description of what was undone.
func PerformUndo(project *models.Project) (string, error) {
	return PerformUndoForProjects(project, nil)
}

// PerformUndoForProjects restores the most recent undo entry for project. If
// the newest entry is grouped, every grouped project must be present in
// projects and have the same grouped entry at the top of its undo stack.
func PerformUndoForProjects(project *models.Project, projects []*models.Project) (string, error) {
	return performRestoreForProjects(project, projects, groupedRestoreParams{
		label:          "undo",
		sourceEntries:  func(meta *UndoMeta) *[]UndoEntry { return &meta.Entries },
		reverseEntries: func(meta *UndoMeta) *[]UndoEntry { return &meta.RedoEntries },
		snapshotPath:   getSnapshotPath,
		reversePath:    getRedoSnapshotPath,
	})
}

// PerformRedo restores the most recent redo snapshot and pushes current state onto the undo stack.
// Returns the action description of what was redone.
func PerformRedo(project *models.Project) (string, error) {
	return PerformRedoForProjects(project, nil)
}

// PerformRedoForProjects restores the most recent redo entry for project. If
// the newest entry is grouped, every grouped project must be present in
// projects and have the same grouped entry at the top of its redo stack.
func PerformRedoForProjects(project *models.Project, projects []*models.Project) (string, error) {
	return performRestoreForProjects(project, projects, groupedRestoreParams{
		label:          "redo",
		sourceEntries:  func(meta *UndoMeta) *[]UndoEntry { return &meta.RedoEntries },
		reverseEntries: func(meta *UndoMeta) *[]UndoEntry { return &meta.Entries },
		snapshotPath:   getRedoSnapshotPath,
		reversePath:    getSnapshotPath,
	})
}

type groupedRestoreParams struct {
	label          string
	sourceEntries  func(meta *UndoMeta) *[]UndoEntry
	reverseEntries func(meta *UndoMeta) *[]UndoEntry
	snapshotPath   func(project *models.Project, num int) string
	reversePath    func(project *models.Project, num int) string
}

type groupedRestorePlan struct {
	project             *models.Project
	originalMeta        *UndoMeta
	updatedMeta         *UndoMeta
	sourceSnapshotPath  string
	reverseSnapshotPath string
	currentContent      []byte
	restoreContent      string
}

func performRestoreForProjects(project *models.Project, projects []*models.Project, p groupedRestoreParams) (string, error) {
	if project == nil {
		return "", fmt.Errorf("project is required")
	}

	meta, err := loadUndoMeta(project)
	if err != nil {
		return "", err
	}

	sourceEntries := p.sourceEntries(meta)
	latest, ok := latestUndoEntry(*sourceEntries)
	if !ok {
		return "", fmt.Errorf("no %s history available", p.label)
	}

	if !isGroupedUndoEntry(latest) {
		return performRestore(project, meta, restoreParams{
			label:          p.label,
			sourceEntries:  p.sourceEntries(meta),
			reverseEntries: p.reverseEntries(meta),
			snapshotPath:   p.snapshotPath,
			reversePath:    p.reversePath,
		})
	}

	return performGroupedRestore(project, projects, latest, p)
}

func performGroupedRestore(anchor *models.Project, projects []*models.Project, anchorEntry UndoEntry, p groupedRestoreParams) (string, error) {
	groupProjectIDs, err := validateGroupEntry(anchorEntry)
	if err != nil {
		return "", err
	}

	anchorID := undoProjectID(anchor)
	if !slices.Contains(groupProjectIDs, anchorID) {
		return "", fmt.Errorf("%s group %q does not include anchor project %q", p.label, anchorEntry.GroupID, anchorID)
	}

	projectsByID := indexUndoProjects(anchor, projects)
	plans := make([]groupedRestorePlan, 0, len(groupProjectIDs))
	for _, projectID := range groupProjectIDs {
		project := projectsByID[projectID]
		if project == nil {
			return "", fmt.Errorf("%s group %q missing project %q", p.label, anchorEntry.GroupID, projectID)
		}
		plan, err := preflightGroupedRestoreProject(project, anchorEntry, p)
		if err != nil {
			return "", err
		}
		plans = append(plans, plan)
	}

	if err := writeGroupedReverseSnapshots(plans); err != nil {
		cleanupGroupedReverseSnapshots(plans)
		return "", err
	}
	if err := writeGroupedTaskRestores(plans); err != nil {
		rollbackGroupedRestore(plans, false)
		return "", err
	}
	if err := saveGroupedRestoreMetas(plans); err != nil {
		rollbackGroupedRestore(plans, true)
		return "", err
	}

	for _, plan := range plans {
		_ = os.Remove(plan.sourceSnapshotPath)
	}
	return undoActionDescription(anchorEntry), nil
}

func preflightGroupedRestoreProject(project *models.Project, anchorEntry UndoEntry, p groupedRestoreParams) (groupedRestorePlan, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return groupedRestorePlan{}, fmt.Errorf("failed to load %s metadata for project %q: %w", p.label, undoProjectLabel(project), err)
	}

	sourceEntries := p.sourceEntries(meta)
	latest, ok := latestUndoEntry(*sourceEntries)
	if !ok {
		return groupedRestorePlan{}, fmt.Errorf("%s group %q missing %s history for project %q", p.label, anchorEntry.GroupID, p.label, undoProjectLabel(project))
	}
	if validateErr := validateSameGroupEntry(anchorEntry, latest); validateErr != nil {
		return groupedRestorePlan{}, fmt.Errorf("%s group %q diverged for project %q: %w", p.label, anchorEntry.GroupID, undoProjectLabel(project), validateErr)
	}

	snapPath := p.snapshotPath(project, latest.Num)
	snapBytes, err := os.ReadFile(snapPath)
	if err != nil {
		return groupedRestorePlan{}, fmt.Errorf("failed to read %s snapshot %d for project %q: %w", p.label, latest.Num, undoProjectLabel(project), err)
	}

	tasksPath := GetTasksFilePath(project)
	currentContent, err := os.ReadFile(tasksPath)
	if err != nil {
		return groupedRestorePlan{}, fmt.Errorf("failed to read current state for %s reverse snapshot in project %q: %w", p.label, undoProjectLabel(project), err)
	}

	restoreContent, err := buildRestoreContent(p.label, string(currentContent), string(snapBytes), latest)
	if err != nil {
		return groupedRestorePlan{}, fmt.Errorf("project %q: %w", undoProjectLabel(project), err)
	}

	nextReverseNum := nextUndoEntryNum(*p.reverseEntries(meta))
	reverseEntry := newReverseUndoEntry(latest, nextReverseNum)
	updatedMeta := cloneUndoMeta(meta)
	*p.sourceEntries(updatedMeta) = removeUndoEntryByNum(*p.sourceEntries(updatedMeta), latest.Num)
	*p.reverseEntries(updatedMeta) = append(*p.reverseEntries(updatedMeta), reverseEntry)

	return groupedRestorePlan{
		project:             project,
		originalMeta:        cloneUndoMeta(meta),
		updatedMeta:         updatedMeta,
		sourceSnapshotPath:  snapPath,
		reverseSnapshotPath: p.reversePath(project, nextReverseNum),
		currentContent:      append([]byte(nil), currentContent...),
		restoreContent:      restoreContent,
	}, nil
}

func writeGroupedReverseSnapshots(plans []groupedRestorePlan) error {
	for _, plan := range plans {
		if err := util.AtomicWriteFile(plan.reverseSnapshotPath, plan.currentContent, FilePermissions); err != nil {
			return fmt.Errorf("failed to write grouped reverse snapshot for project %q: %w", undoProjectLabel(plan.project), err)
		}
	}
	return nil
}

func writeGroupedTaskRestores(plans []groupedRestorePlan) error {
	for _, plan := range plans {
		tasksPath := GetTasksFilePath(plan.project)
		if err := util.AtomicWriteFile(tasksPath, []byte(plan.restoreContent), FilePermissions); err != nil {
			return fmt.Errorf("failed to restore grouped snapshot for project %q: %w", undoProjectLabel(plan.project), err)
		}
	}
	return nil
}

func saveGroupedRestoreMetas(plans []groupedRestorePlan) error {
	for _, plan := range plans {
		if err := saveUndoMeta(plan.project, plan.updatedMeta); err != nil {
			return fmt.Errorf("failed to save grouped restore metadata for project %q: %w", undoProjectLabel(plan.project), err)
		}
	}
	return nil
}

func rollbackGroupedRestore(plans []groupedRestorePlan, restoreMeta bool) {
	for _, plan := range plans {
		if err := util.AtomicWriteFile(GetTasksFilePath(plan.project), plan.currentContent, FilePermissions); err != nil {
			logging.Warn("failed to rollback grouped restore for %s: %v", undoProjectLabel(plan.project), err)
		}
		if restoreMeta {
			if err := saveUndoMeta(plan.project, plan.originalMeta); err != nil {
				logging.Warn("failed to rollback grouped restore metadata for %s: %v", undoProjectLabel(plan.project), err)
			}
		}
	}
	cleanupGroupedReverseSnapshots(plans)
}

func cleanupGroupedReverseSnapshots(plans []groupedRestorePlan) {
	for _, plan := range plans {
		_ = os.Remove(plan.reverseSnapshotPath)
	}
}

func buildRestoreContent(label, currentContent, snapContent string, latest UndoEntry) (string, error) {
	snapResult, err := ParseTasksMarkdownWithSections(snapContent)
	if err != nil {
		return "", fmt.Errorf("%s snapshot is invalid: %w", label, err)
	}

	restoreContent := snapContent
	if len(latest.TaskIDs) > 0 {
		targeted, tErr := buildTargetedRestore(currentContent, snapResult, latest.TaskIDs)
		if tErr != nil {
			logging.Warn("targeted %s failed, falling back to full restore: %v", label, tErr)
		} else {
			restoreContent = targeted
		}
	}
	return restoreContent, nil
}

func newReverseUndoEntry(latest UndoEntry, num int) UndoEntry {
	return UndoEntry{
		Num:             num,
		Action:          latest.Action,
		Context:         latest.Context,
		Timestamp:       time.Now(),
		TaskIDs:         cloneUndoStringSlice(latest.TaskIDs),
		GroupID:         latest.GroupID,
		GroupProjectIDs: cloneUndoStringSlice(latest.GroupProjectIDs),
	}
}

func undoActionDescription(entry UndoEntry) string {
	if entry.Context != "" {
		return fmt.Sprintf("%s: %s", entry.Action, entry.Context)
	}
	return entry.Action
}

func latestUndoEntry(entries []UndoEntry) (UndoEntry, bool) {
	if len(entries) == 0 {
		return UndoEntry{}, false
	}
	latest := entries[0]
	for _, entry := range entries[1:] {
		if entry.Num > latest.Num {
			latest = entry
		}
	}
	return latest, true
}

func nextUndoEntryNum(entries []UndoEntry) int {
	nextNum := 1
	for _, entry := range entries {
		if entry.Num >= nextNum {
			nextNum = entry.Num + 1
		}
	}
	return nextNum
}

func removeUndoEntryByNum(entries []UndoEntry, num int) []UndoEntry {
	filtered := make([]UndoEntry, 0, max(len(entries)-1, 0))
	removed := false
	for _, entry := range entries {
		if !removed && entry.Num == num {
			removed = true
			continue
		}
		filtered = append(filtered, cloneUndoEntry(entry))
	}
	return filtered
}

func isGroupedUndoEntry(entry UndoEntry) bool {
	return entry.GroupID != "" || len(entry.GroupProjectIDs) > 0
}

func validateGroupEntry(entry UndoEntry) ([]string, error) {
	if entry.GroupID == "" {
		return nil, fmt.Errorf("grouped restore entry is missing group id")
	}
	if len(entry.GroupProjectIDs) == 0 {
		return nil, fmt.Errorf("group %q is missing project ids", entry.GroupID)
	}

	seen := make(map[string]bool, len(entry.GroupProjectIDs))
	ids := make([]string, 0, len(entry.GroupProjectIDs))
	for _, id := range entry.GroupProjectIDs {
		if id == "" {
			return nil, fmt.Errorf("group %q includes an empty project id", entry.GroupID)
		}
		if seen[id] {
			return nil, fmt.Errorf("group %q includes duplicate project id %q", entry.GroupID, id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

func validateSameGroupEntry(want, got UndoEntry) error {
	if got.GroupID != want.GroupID {
		return fmt.Errorf("top entry group id %q does not match %q", got.GroupID, want.GroupID)
	}
	wantIDs, err := validateGroupEntry(want)
	if err != nil {
		return err
	}
	gotIDs, err := validateGroupEntry(got)
	if err != nil {
		return err
	}
	slices.Sort(wantIDs)
	slices.Sort(gotIDs)
	if !slices.Equal(wantIDs, gotIDs) {
		return fmt.Errorf("top entry project ids %v do not match %v", gotIDs, wantIDs)
	}
	return nil
}

func indexUndoProjects(anchor *models.Project, projects []*models.Project) map[string]*models.Project {
	projectsByID := make(map[string]*models.Project, len(projects)+1)
	if anchor != nil {
		projectsByID[undoProjectID(anchor)] = anchor
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		projectsByID[undoProjectID(project)] = project
	}
	return projectsByID
}

func undoProjectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.ID != "" {
		return project.ID
	}
	return project.Slug
}

func undoProjectLabel(project *models.Project) string {
	if project == nil {
		return "<nil>"
	}
	if project.Name != "" {
		return project.Name
	}
	if project.Slug != "" {
		return project.Slug
	}
	return undoProjectID(project)
}
