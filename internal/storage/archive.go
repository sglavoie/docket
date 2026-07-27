package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/util"
)

const archiveFileName = "completed.md"
const archiveRotationThreshold int64 = 5 * 1024 * 1024 // 5MB

// getArchiveFilePath returns the path to the project's archive file.
func getArchiveFilePath(project *models.Project) string {
	return filepath.Join(project.Path, archiveFileName)
}

// LoadArchivedTasks reads completed.md (and any rotated files) for a project
// and returns all parsed tasks. Returns an empty slice if no archive exists.
func LoadArchivedTasks(project *models.Project) []*models.Task {
	var allTasks []*models.Task

	archiveFiles, err := listArchiveFiles(project.Path)
	if err != nil {
		logging.Warn("failed to list archive files for %s: %v", project.Path, err)
		return allTasks
	}

	for _, archiveFile := range archiveFiles {
		tasks, err := parseArchiveFile(archiveFile.path)
		if err != nil {
			logging.Warn("skipping corrupt archive %s: %v", archiveFile.path, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks
}

// parseArchiveFile reads a single archive file and returns the tasks found in it.
func parseArchiveFile(path string) ([]*models.Task, error) {
	tasks, _, err := parseArchiveFileWithHeadingLines(path)
	return tasks, err
}

// parseArchiveFileWithHeadingLines reads a single archive file and returns the
// parsed tasks plus the 1-based source line for each task heading.
func parseArchiveFileWithHeadingLines(path string) ([]*models.Task, []int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read archive file: %w", err)
	}
	return parseArchiveContentWithHeadingLines(string(content))
}

// parseArchiveContentWithHeadingLines parses already-read archive bytes so the
// index builder can hash and parse a source from a single read, and so the
// transactional archive path can index the exact content it is about to write.
func parseArchiveContentWithHeadingLines(content string) ([]*models.Task, []int, error) {
	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse archive: %w", err)
	}

	return result.Tasks, archiveTaskHeadingLines(content), nil
}

func archiveTaskHeadingLines(content string) []int {
	lines := strings.Split(content, "\n")
	headingLines := make([]int, 0)
	for i, line := range lines {
		if headingRegex.MatchString(line) {
			headingLines = append(headingLines, i+1)
		}
	}
	return headingLines
}

// ArchiveTasks moves completed tasks from the project to the archive file. It
// routes canonical filesystem projects (<data>/projects/<slug>) through the
// durable transaction journal so active-task removal, archive append/rotation,
// attachment retention, and index replacement commit as one atomic, crash-safe,
// undoable transaction. Non-filesystem repositories (test mocks) and projects
// outside that layout fall back to the legacy compensating-save path.
// Returns the number of tasks archived and any error encountered.
func ArchiveTasks(project *models.Project) (int, error) {
	if svc, ok := archiveCommitService(project); ok {
		return ArchiveTasksTx(svc, project)
	}
	return archiveTasksLegacy(project)
}

// archiveCommitService returns a CommitService for a project stored on the real
// filesystem in the canonical <data>/projects/<slug> layout. It returns ok=false for
// non-filesystem repositories and any project outside that layout so callers fall
// back to the legacy archive path. The service is ledger-less; production undo of
// archiving still flows through the caller's pre-mutation undo snapshot until the
// application-service sessions wire the workspace ledger into this boundary.
func archiveCommitService(project *models.Project) (*CommitService, bool) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		return nil, false
	}
	projectsDir := filepath.Dir(project.Path)
	if filepath.Base(projectsDir) != "projects" {
		return nil, false
	}
	dataDir := filepath.Dir(projectsDir)
	return NewCommitService(archiveTransactionsDir(dataDir), []string{dataDir}), true
}

// archiveTransactionsDir mirrors config.TransactionsDir without importing config
// (storage must not depend on config): the durable transactions directory lives
// under the workspace's internal ".tt" directory, matching the layout the config
// package derives and the ".tt" convention already used elsewhere in storage.
func archiveTransactionsDir(dataDir string) string {
	return filepath.Join(dataDir, ".tt", "transactions")
}

// ArchiveTasksTx archives a project's completed tasks through the durable
// transaction journal carried by svc. Active-task removal (the project's
// tasks.md rewrite), archive append with rotation, attachment retention, and the
// derived index replacement are assembled into ONE ChangeSet and committed
// atomically: a crash or failure leaves the project fully un-archived, and — when
// svc carries a workspace ledger — undo restores the active task while removing
// the archived copy, and redo re-archives. A parse failure in any existing
// archive source aborts the archive before any write so corruption is surfaced,
// not compounded. A nil svc falls back to the legacy compensating-save path.
func ArchiveTasksTx(svc *CommitService, project *models.Project) (int, error) {
	if svc == nil {
		return archiveTasksLegacy(project)
	}

	plan, err := PlanArchiveTasks(project)
	if err != nil {
		return 0, err
	}
	if plan.Count == 0 {
		return 0, nil
	}

	cs := ChangeSet{
		Project:       project,
		Artifacts:     plan.Artifacts,
		AttachmentOps: plan.AttachmentOps,
		UndoAction:    "Archive Tasks",
		UndoContext:   pluralizeArchivedContext(plan.Count),
	}
	if _, err := svc.Commit(cs); err != nil {
		return 0, fmt.Errorf("failed to commit archive transaction: %w", err)
	}

	return plan.Count, nil
}

// ArchivePlan carries the durable effects of archiving a project's completed
// tasks: the archive file rewrites and refreshed index (Artifacts), the archived
// tasks' attachment-retention deletes (AttachmentOps), and how many tasks were
// archived (Count). It is built by PlanArchiveTasks so an application-layer use
// case can fold it — together with durable config cleanup and the workspace undo
// ledger — into one atomic ChangeSet, rather than invoking the ledger-less
// storage wrapper directly.
type ArchivePlan struct {
	Artifacts     []ArtifactChange
	AttachmentOps []AttachmentOp
	Count         int
}

// PlanArchiveTasks computes the complete archive effect for a project and mutates
// the in-memory project to its post-archive shape (completed tasks removed,
// orphaned parent/dependency references cleared), returning the archive file
// rewrites, the refreshed archive index, and the attachment-retention deletes as
// an ArchivePlan. It performs NO commit: the caller assembles the plan (plus any
// durable config artifacts) into a single ChangeSet and commits it through the
// transactional boundary. A parse failure in an existing archive source aborts
// before the project is mutated so corruption is surfaced, not compounded. When
// the project has no completed tasks the plan's Count is 0 and the project is
// left untouched.
func PlanArchiveTasks(project *models.Project) (ArchivePlan, error) {
	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		return ArchivePlan{}, nil
	}

	// Model archive append and rotation as full-file rewrites so both, plus the
	// index refresh, ride the same transaction. This reads the current archive
	// files; the CommitService serializes the actual write under the project lock.
	archiveWrites, err := planArchiveWrites(project, completedTasks)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("failed to plan archive writes: %w", err)
	}

	// Build the derived index over the exact bytes about to be written. A parse
	// failure in any archive source aborts before the project is mutated.
	_, indexPayload, err := buildArchiveIndexFromContents(archiveWrites)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("failed to build archive index: %w", err)
	}

	// Active-task removal: drop completed tasks from the in-memory project and
	// repair references to them; the tasks.md write in the changeset carries this.
	archivedIDs := make(map[string]bool, len(completedTasks))
	for _, t := range completedTasks {
		archivedIDs[t.ID] = true
	}
	removeCompletedTasks(project)
	clearOrphanedParentRefs(project, archivedIDs)

	artifacts := make([]ArtifactChange, 0, len(archiveWrites)+1)
	for _, f := range archiveWrites {
		artifacts = append(artifacts, ArtifactChange{Path: filepath.Join(project.Path, f.fileName), Data: f.content})
	}
	artifacts = append(artifacts, ArtifactChange{Path: archiveIndexPath(project), Data: indexPayload})

	// Attachment retention: archived tasks' attachments are removed in the same
	// transaction, so undo restores their bytes alongside the task metadata.
	attachmentOps := make([]AttachmentOp, 0, len(completedTasks))
	for _, t := range completedTasks {
		attachmentOps = append(attachmentOps, AttachmentOp{Kind: AttachmentDelete, Task: t, SourceProject: project})
	}

	return ArchivePlan{Artifacts: artifacts, AttachmentOps: attachmentOps, Count: len(completedTasks)}, nil
}

// pluralizeArchivedContext renders the undo-context task count for an archive.
func pluralizeArchivedContext(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}

// planArchiveWrites returns the full byte content of every archive file the
// archive will write, in canonical index order (completed.md first, then rotated
// completed.N.md). When the current completed.md is under the rotation threshold
// the new archive block is appended to it; otherwise rotation shifts every
// existing file up by one and completed.md starts fresh, all modeled as full-file
// rewrites so the whole rotation is atomic within the commit transaction.
func planArchiveWrites(project *models.Project, completedTasks []*models.Task) ([]inMemoryArchiveFile, error) {
	archivePath := getArchiveFilePath(project)
	newlyArchived := []byte(WriteArchivedTasks(project.Name, completedTasks))

	existing, existed, err := readFileMaybe(archivePath)
	if err != nil {
		return nil, err
	}

	if !existed || int64(len(existing)) < archiveRotationThreshold {
		content := make([]byte, 0, len(existing)+len(newlyArchived))
		content = append(content, existing...)
		content = append(content, newlyArchived...)
		return []inMemoryArchiveFile{{fileName: archiveFileName, content: content}}, nil
	}

	// Rotation: the current completed.md becomes completed.1.md, existing
	// completed.N.md shift up to completed.(N+1).md, and completed.md restarts with
	// only the new block.
	files := []inMemoryArchiveFile{
		{fileName: archiveFileName, content: newlyArchived},
		{fileName: "completed.1.md", content: existing},
	}
	maxRotation, err := maxArchiveRotation(project.Path)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= maxRotation; i++ {
		rotated, _, rerr := readFileMaybe(filepath.Join(project.Path, fmt.Sprintf("completed.%d.md", i)))
		if rerr != nil {
			return nil, rerr
		}
		files = append(files, inMemoryArchiveFile{fileName: fmt.Sprintf("completed.%d.md", i+1), content: rotated})
	}
	return files, nil
}

// maxArchiveRotation returns the highest existing completed.N.md rotation number,
// or 0 when none exist. It breaks on the first gap and on any unexpected stat
// error so a persistent non-IsNotExist error cannot spin the scan forever.
func maxArchiveRotation(projectPath string) (int, error) {
	maxRotation := 0
	for i := 1; ; i++ {
		_, statErr := os.Stat(filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i)))
		if statErr != nil {
			if !os.IsNotExist(statErr) {
				return 0, fmt.Errorf("failed to stat rotation candidate: %w", statErr)
			}
			break
		}
		maxRotation = i
	}
	return maxRotation, nil
}

// archiveTasksLegacy is the pre-transaction archive path retained for
// non-filesystem repositories and non-canonical project layouts. It appends to
// the archive with manual rollback compensation rather than one atomic
// transaction.
func archiveTasksLegacy(project *models.Project) (int, error) {
	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		return 0, nil
	}

	archivePath := getArchiveFilePath(project)

	// Rotate archive if needed before appending
	if err := rotateArchiveIfNeeded(project.Path, archiveRotationThreshold); err != nil {
		return 0, fmt.Errorf("failed to rotate archive: %w", err)
	}

	// Record archive size before appending so we can rollback on failure.
	// archiveExistedBefore tracks whether the file pre-existed so that, on
	// rollback, we remove a freshly created archive rather than truncating
	// it to zero bytes (which would leave a stray empty completed.md).
	var archiveSizeBefore int64
	var archiveExistedBefore bool
	if info, err := os.Stat(archivePath); err == nil {
		archiveSizeBefore = info.Size()
		archiveExistedBefore = true
	}

	// Generate markdown content for archived tasks
	content := WriteArchivedTasks(project.Name, completedTasks)

	// Append to archive file
	if err := appendToArchiveFile(archivePath, content); err != nil {
		return 0, fmt.Errorf("failed to append to archive: %w", err)
	}

	// Remove completed tasks from project and fix orphaned children
	archivedIDs := make(map[string]bool, len(completedTasks))
	for _, t := range completedTasks {
		archivedIDs[t.ID] = true
	}
	removeCompletedTasks(project)
	clearOrphanedParentRefs(project, archivedIDs)

	// Save the modified project; rollback archive append on failure
	if err := SaveProject(project); err != nil {
		// Rollback the archive append. If the file did not exist before this
		// call, remove it entirely; otherwise truncate it back to the original
		// size. Truncating a previously absent file would leave a stray empty
		// completed.md behind even though no tasks were ever archived.
		if archiveExistedBefore {
			if truncErr := os.Truncate(archivePath, archiveSizeBefore); truncErr != nil {
				logging.Warn("failed to rollback archive after save failure: %v", truncErr)
			}
		} else {
			if rmErr := os.Remove(archivePath); rmErr != nil && !os.IsNotExist(rmErr) {
				logging.Warn("failed to remove rolled-back archive after save failure: %v", rmErr)
			}
		}
		// Reload project from disk to avoid in-memory/on-disk divergence
		if loadErr := LoadProject(project); loadErr != nil {
			logging.Warn("failed to reload project after save failure, restoring in-memory tasks: %v", loadErr)
			project.Tasks = append(project.Tasks, completedTasks...)
		}
		return 0, fmt.Errorf("failed to save project after archiving: %w", err)
	}

	if err := RebuildArchiveIndex(project); err != nil {
		logging.Warn("archive index is stale after archiving %d tasks in %s; rebuild required: %v", len(completedTasks), project.Name, err)
	}

	// Clean up attachment files only after both archive and save succeeded
	for _, task := range completedTasks {
		if err := CleanupTaskAttachments(project, task); err != nil {
			logging.Warn("failed to clean up attachments for task %s: %v", task.ID, err)
		}
	}

	return len(completedTasks), nil
}

// rotateArchiveIfNeeded checks if the archive file exceeds the threshold and rotates it.
// Rotation: completed.md -> completed.1.md, completed.1.md -> completed.2.md, etc.
func rotateArchiveIfNeeded(projectPath string, threshold int64) error {
	archivePath := filepath.Join(projectPath, archiveFileName)

	info, err := os.Stat(archivePath)
	if os.IsNotExist(err) {
		return nil // No archive file to rotate
	}
	if err != nil {
		return fmt.Errorf("failed to stat archive file: %w", err)
	}

	if info.Size() < threshold {
		return nil // File is under threshold
	}

	// Find the next available rotation number. Break on any non-existence
	// signal AND on any unexpected stat error (e.g. permission denied), since
	// the original "only IsNotExist breaks" loop would spin forever if Stat
	// kept returning a non-IsNotExist error for the same path.
	maxRotation := 0
	for i := 1; ; i++ {
		rotatedPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i))
		_, statErr := os.Stat(rotatedPath)
		if statErr != nil {
			if !os.IsNotExist(statErr) {
				return fmt.Errorf("failed to stat rotation candidate %s: %w", rotatedPath, statErr)
			}
			break
		}
		maxRotation = i
	}

	// Track completed renames for rollback on failure
	type rename struct{ from, to string }
	var completed []rename

	rollback := func() {
		// Reverse renames in LIFO order (best-effort)
		for i := len(completed) - 1; i >= 0; i-- {
			r := completed[i]
			if rbErr := os.Rename(r.to, r.from); rbErr != nil {
				logging.Warn("archive rotation rollback failed: %s -> %s: %v", r.to, r.from, rbErr)
			}
		}
	}

	// Shift existing rotated files up by one
	for i := maxRotation; i >= 1; i-- {
		oldPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i))
		newPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i+1))
		if err := os.Rename(oldPath, newPath); err != nil {
			rollback()
			return fmt.Errorf("failed to rotate %s to %s: %w", oldPath, newPath, err)
		}
		completed = append(completed, rename{from: oldPath, to: newPath})
	}

	// Move current archive to completed.1.md
	rotatedPath := filepath.Join(projectPath, "completed.1.md")
	if err := os.Rename(archivePath, rotatedPath); err != nil {
		rollback()
		return fmt.Errorf("failed to rotate archive to %s: %w", rotatedPath, err)
	}

	return nil
}

// WriteArchivedTasks generates markdown content for archived tasks.
// Uses a simplified format without project sections metadata.
func WriteArchivedTasks(projectName string, tasks []*models.Task) string {
	if len(tasks) == 0 {
		return ""
	}

	var b strings.Builder

	// Write archive header with timestamp
	fmt.Fprintf(&b, "\n## Archived: %s\n", time.Now().Format(util.DateTimeFormat))
	fmt.Fprintf(&b, "<!-- project: %s, count: %d -->\n\n", projectName, len(tasks))

	// Write each task in a flat format (no hierarchy in archive)
	for _, task := range tasks {
		b.WriteString(writeArchivedTask(task))
		b.WriteString("\n")
	}

	return b.String()
}

// writeArchivedTask generates markdown for a single archived task.
func writeArchivedTask(task *models.Task) string {
	var b strings.Builder

	// Write heading (always level 3 for archived tasks)
	fmt.Fprintf(&b, "### %s: %s\n", task.Type.HeadingPrefix(), task.Title)

	// Write metadata using the canonical metadata writer; non-metadata blocks
	// (description, activity, comments, decisions) are preserved separately below.
	b.WriteString(format.WriteMetadataV2(task))
	b.WriteString("\n")

	// Write description (escape heading-like lines to prevent parser corruption)
	if task.Description != "" {
		b.WriteString("<!-- DESCRIPTION -->\n")
		b.WriteString(escapeDescription(task.Description))
		b.WriteString("\n<!-- /DESCRIPTION -->\n")
	}

	// Preserve activity logs for future metrics (cycle time, blocked)
	if len(task.Activity) > 0 {
		b.WriteString("\n")
		b.WriteString(format.WriteActivity(task))
	}

	// Preserve comments to prevent data loss
	if len(task.Comments) > 0 {
		b.WriteString("\n")
		b.WriteString(format.WriteComments(task))
	}

	// Preserve decisions to prevent data loss when archiving
	if len(task.Decisions) > 0 {
		b.WriteString("\n")
		b.WriteString(format.WriteDecisions(task))
	}

	return b.String()
}

// appendToArchiveFile appends content to the archive file, creating it if necessary.
func appendToArchiveFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePermissions)
	if err != nil {
		return fmt.Errorf("failed to open archive file: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to archive file: %w", err)
	}

	// Sync to disk before returning so that a subsequent crash doesn't
	// lose the archived data (the caller truncates on save failure).
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync archive file: %w", err)
	}

	// Surface Close errors: on NFS / disk-full, buffered write failures may
	// only be reported here, and a silent drop would prevent the caller's
	// rollback path from running, losing the archived tasks.
	if err := f.Close(); err != nil {
		closed = true
		return fmt.Errorf("failed to close archive file: %w", err)
	}
	closed = true
	return nil
}

// removeCompletedTasks removes all completed tasks from the project's task list.
func removeCompletedTasks(project *models.Project) {
	var remaining []*models.Task
	for _, task := range project.Tasks {
		if !task.IsComplete() {
			remaining = append(remaining, task)
		}
	}
	project.Tasks = remaining
}

// clearOrphanedParentRefs clears ParentID on any remaining task whose parent
// was archived (removed). Without this, children of archived parents would have
// dangling ParentID references and be silently promoted to root by BuildTaskTree.
// Also removes DependsOn and RelatedTo references to archived tasks.
func clearOrphanedParentRefs(project *models.Project, removedIDs map[string]bool) {
	for _, task := range project.Tasks {
		if task.ParentID != nil && removedIDs[*task.ParentID] {
			task.ParentID = nil
		}
		if len(task.DependsOn) > 0 {
			clean := task.DependsOn[:0]
			for _, depID := range task.DependsOn {
				if !removedIDs[depID] {
					clean = append(clean, depID)
				}
			}
			task.DependsOn = clean
		}
		if len(task.RelatedTo) > 0 {
			clean := task.RelatedTo[:0]
			for _, relID := range task.RelatedTo {
				if !removedIDs[relID] {
					clean = append(clean, relID)
				}
			}
			task.RelatedTo = clean
		}
	}
}
