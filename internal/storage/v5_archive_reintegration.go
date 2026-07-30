package storage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/storage/format"
)

var legacyV4RotatedArchiveName = regexp.MustCompile(`^completed\.[1-9][0-9]*\.md$`)

const (
	archiveFileName      = "completed.md"
	archiveIndexFileName = ".completed.index.json"
)

// V5ArchiveTaskSource is one converted legacy archive document. SourceID is a
// stable, workspace-relative label used for deterministic ordering and audit
// output; Path is the artifact removed when the migration transaction publishes
// the reintegrated active document.
type V5ArchiveTaskSource struct {
	SourceID   string
	Path       string
	Conversion format.V5TaskConversionResult
}

// V5ProjectTaskReintegrationInput carries every converted task document for one
// retained project. Active is authoritative when its stable task IDs collide
// with archived records. ArchiveIndexPath is optional because older workspaces
// may never have built the derived index.
type V5ProjectTaskReintegrationInput struct {
	ProjectID        string
	Active           format.V5TaskConversionResult
	Archives         []V5ArchiveTaskSource
	ArchiveIndexPath string
}

// V5TaskReintegrationInput is the complete workspace candidate. StableIDRemap
// is intentionally optional: the v4 corpus needs no invented full-ID changes,
// but a caller that has an independently justified old-to-new mapping can apply
// it consistently to task IDs, parents, and cross-project dependencies.
type V5TaskReintegrationInput struct {
	Projects      []V5ProjectTaskReintegrationInput
	StableIDRemap map[string]string
}

// V5ProjectTaskCandidate is the loss-accounted active task set for one project.
// Tasks retain active document order, followed by retained archive tasks in
// stable SourceID order. ShortIDs are assigned once across that combined set.
type V5ProjectTaskCandidate struct {
	ProjectID   string                `json:"project_id"`
	ProjectName string                `json:"project_name"`
	Tasks       []format.V5TaskRecord `json:"tasks"`
}

// V5TaskRepairEntry is one deterministic identity or reference repair.
type V5TaskRepairEntry struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
	Action    string `json:"action"`
	Field     string `json:"field"`
	Source    string `json:"source,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Reason    string `json:"reason"`
}

// V5TaskReintegrationReport accounts for every active and archived input task.
// OmittedTasks are stale full-ID duplicates represented by explicit entries.
type V5TaskReintegrationReport struct {
	InputTasks    int                 `json:"input_tasks"`
	RetainedTasks int                 `json:"retained_tasks"`
	OmittedTasks  int                 `json:"omitted_tasks"`
	Entries       []V5TaskRepairEntry `json:"entries"`
}

// V5TaskReintegrationPlan is pure migration output. Cleanup contains only
// archive Markdown and derived index deletes; the caller must commit these
// artifacts atomically with the canonical active task documents after all
// workspace candidates validate.
type V5TaskReintegrationPlan struct {
	Projects []V5ProjectTaskCandidate
	Cleanup  []ArtifactChange
	Report   V5TaskReintegrationReport
}

type v5TaskCandidate struct {
	projectIndex int
	projectID    string
	sourceID     string
	sourcePath   string
	ordinal      int
	oldShortID   int
	task         format.V5TaskRecord
	remapEntries []V5TaskRepairEntry
}

type v5ProjectCandidates struct {
	projectID   string
	projectName string
	active      []*v5TaskCandidate
	archives    []*v5TaskCandidate
}

// PlanV5ArchiveReintegration merges converted active and archive documents
// without touching the filesystem or mutating its input. Active records win
// stable-ID collisions workspace-wide. Archive-only collisions retain the
// lexicographically first stable source. Parents are project-local, dependencies
// resolve workspace-wide, and all archive/index removals remain deferred
// ArtifactChanges for the later migration commit.
//
//nolint:gocyclo // Reintegration is a single deterministic validation and identity-repair pass.
func PlanV5ArchiveReintegration(input V5TaskReintegrationInput) (V5TaskReintegrationPlan, error) {
	idRemap, err := validateV5StableIDRemap(input.StableIDRemap)
	if err != nil {
		return V5TaskReintegrationPlan{}, err
	}

	projects := make([]v5ProjectCandidates, len(input.Projects))
	projectIDs := make(map[string]bool, len(input.Projects))
	activeIDs := make(map[string]*v5TaskCandidate)
	cleanupPaths := make(map[string]bool)
	var cleanup []ArtifactChange
	report := V5TaskReintegrationReport{}

	for projectIndex, projectInput := range input.Projects {
		projectID := strings.TrimSpace(projectInput.ProjectID)
		if projectID == "" {
			return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration project %d has an empty ID", projectIndex)
		}
		if projectIDs[projectID] {
			return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration has duplicate project ID %q", projectID)
		}
		projectIDs[projectID] = true
		if strings.TrimSpace(projectInput.Active.ProjectName) == "" {
			return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration project %q has an empty active project name", projectID)
		}

		project := v5ProjectCandidates{
			projectID:   projectID,
			projectName: projectInput.Active.ProjectName,
		}
		for ordinal, record := range projectInput.Active.Tasks {
			candidate, candidateErr := newV5TaskCandidate(
				projectIndex,
				projectID,
				"active",
				"",
				ordinal,
				record,
				idRemap,
			)
			if candidateErr != nil {
				return V5TaskReintegrationPlan{}, candidateErr
			}
			if winner := activeIDs[candidate.task.ID]; winner != nil {
				return V5TaskReintegrationPlan{}, fmt.Errorf(
					"v5 task reintegration has duplicate active task ID %q in projects %q and %q",
					candidate.task.ID,
					winner.projectID,
					projectID,
				)
			}
			activeIDs[candidate.task.ID] = candidate
			project.active = append(project.active, candidate)
			report.InputTasks++
		}

		archiveSources := append([]V5ArchiveTaskSource(nil), projectInput.Archives...)
		sort.Slice(archiveSources, func(i, j int) bool {
			if archiveSources[i].SourceID != archiveSources[j].SourceID {
				return archiveSources[i].SourceID < archiveSources[j].SourceID
			}
			return filepath.Clean(archiveSources[i].Path) < filepath.Clean(archiveSources[j].Path)
		})
		sourceIDs := make(map[string]bool, len(archiveSources))
		for _, archive := range archiveSources {
			sourceID := strings.TrimSpace(archive.SourceID)
			if sourceID == "" {
				return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration project %q has an archive with an empty source ID", projectID)
			}
			if sourceIDs[sourceID] {
				return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration project %q has duplicate archive source ID %q", projectID, sourceID)
			}
			sourceIDs[sourceID] = true
			if archive.Conversion.ProjectName != project.projectName {
				return V5TaskReintegrationPlan{}, fmt.Errorf(
					"v5 task reintegration archive %q project name %q does not match active project name %q",
					sourceID,
					archive.Conversion.ProjectName,
					project.projectName,
				)
			}
			cleanPath, pathErr := validateV5ArchiveCleanupPath(archive.Path)
			if pathErr != nil {
				return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration archive %q: %w", sourceID, pathErr)
			}
			if cleanupPaths[cleanPath] {
				return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration repeats cleanup path %q", cleanPath)
			}
			cleanupPaths[cleanPath] = true
			cleanup = append(cleanup, ArtifactChange{Path: cleanPath, Delete: true})

			for ordinal, record := range archive.Conversion.Tasks {
				if record.State != "done" && record.State != "canceled" {
					return V5TaskReintegrationPlan{}, fmt.Errorf(
						"v5 task reintegration archive %q task %q has nonterminal state %q",
						sourceID,
						record.ID,
						record.State,
					)
				}
				candidate, candidateErr := newV5TaskCandidate(
					projectIndex,
					projectID,
					sourceID,
					cleanPath,
					ordinal,
					record,
					idRemap,
				)
				if candidateErr != nil {
					return V5TaskReintegrationPlan{}, candidateErr
				}
				project.archives = append(project.archives, candidate)
				report.InputTasks++
			}
		}
		if projectInput.ArchiveIndexPath != "" {
			cleanPath, pathErr := validateV5ArchiveIndexCleanupPath(projectInput.ArchiveIndexPath)
			if pathErr != nil {
				return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration project %q: %w", projectID, pathErr)
			}
			if cleanupPaths[cleanPath] {
				return V5TaskReintegrationPlan{}, fmt.Errorf("v5 task reintegration repeats cleanup path %q", cleanPath)
			}
			cleanupPaths[cleanPath] = true
			cleanup = append(cleanup, ArtifactChange{Path: cleanPath, Delete: true})
		}
		projects[projectIndex] = project
	}

	archiveIDs := make(map[string]*v5TaskCandidate)
	var archiveCandidates []*v5TaskCandidate
	for index := range projects {
		archiveCandidates = append(archiveCandidates, projects[index].archives...)
	}
	sort.Slice(archiveCandidates, func(i, j int) bool {
		if archiveCandidates[i].projectID != archiveCandidates[j].projectID {
			return archiveCandidates[i].projectID < archiveCandidates[j].projectID
		}
		if archiveCandidates[i].sourceID != archiveCandidates[j].sourceID {
			return archiveCandidates[i].sourceID < archiveCandidates[j].sourceID
		}
		if archiveCandidates[i].sourcePath != archiveCandidates[j].sourcePath {
			return archiveCandidates[i].sourcePath < archiveCandidates[j].sourcePath
		}
		return archiveCandidates[i].ordinal < archiveCandidates[j].ordinal
	})
	for _, candidate := range archiveCandidates {
		if winner := activeIDs[candidate.task.ID]; winner != nil {
			report.OmittedTasks++
			report.Entries = append(report.Entries, V5TaskRepairEntry{
				ProjectID: candidate.projectID,
				TaskID:    candidate.task.ID,
				Action:    "omitted",
				Field:     "task",
				Source:    candidate.sourceID,
				From:      "archive",
				To:        "active",
				Reason:    "active task with the same stable ID takes precedence",
			})
			continue
		}
		if winner := archiveIDs[candidate.task.ID]; winner != nil {
			report.OmittedTasks++
			report.Entries = append(report.Entries, V5TaskRepairEntry{
				ProjectID: candidate.projectID,
				TaskID:    candidate.task.ID,
				Action:    "omitted",
				Field:     "task",
				Source:    candidate.sourceID,
				From:      candidate.sourceID,
				To:        winner.sourceID,
				Reason:    "earlier deterministic archive source retains the duplicate stable ID",
			})
			continue
		}
		archiveIDs[candidate.task.ID] = candidate
	}

	// Rebuild each retained archive slice after collision selection. The
	// collection above intentionally considered every project together so an
	// active stable ID wins even when its stale archive copy is cross-project.
	for index := range projects {
		retained := projects[index].archives[:0]
		for _, candidate := range projects[index].archives {
			if archiveIDs[candidate.task.ID] == candidate {
				retained = append(retained, candidate)
			}
		}
		projects[index].archives = retained
		sort.Slice(projects[index].archives, func(i, j int) bool {
			left, right := projects[index].archives[i], projects[index].archives[j]
			if left.sourceID != right.sourceID {
				return left.sourceID < right.sourceID
			}
			if left.sourcePath != right.sourcePath {
				return left.sourcePath < right.sourcePath
			}
			return left.ordinal < right.ordinal
		})
	}

	retainedTaskProjects := make(map[string]string, len(activeIDs)+len(archiveIDs))
	projectRetainedIDs := make([]map[string]bool, len(projects))
	for projectIndex := range projects {
		projectRetainedIDs[projectIndex] = make(map[string]bool)
		for _, candidate := range append(
			append([]*v5TaskCandidate(nil), projects[projectIndex].active...),
			projects[projectIndex].archives...,
		) {
			retainedTaskProjects[candidate.task.ID] = projects[projectIndex].projectID
			projectRetainedIDs[projectIndex][candidate.task.ID] = true
		}
	}

	plan := V5TaskReintegrationPlan{
		Projects: make([]V5ProjectTaskCandidate, len(projects)),
		Report:   report,
	}
	for projectIndex, project := range projects {
		candidates := append(append([]*v5TaskCandidate(nil), project.active...), project.archives...)
		output := V5ProjectTaskCandidate{
			ProjectID:   project.projectID,
			ProjectName: project.projectName,
			Tasks:       make([]format.V5TaskRecord, 0, len(candidates)),
		}
		for index, candidate := range candidates {
			task := cloneV5TaskRecord(candidate.task)
			task.ShortID = index + 1
			plan.Report.Entries = append(plan.Report.Entries, candidate.remapEntries...)
			if candidate.oldShortID != task.ShortID {
				plan.Report.Entries = append(plan.Report.Entries, V5TaskRepairEntry{
					ProjectID: project.projectID,
					TaskID:    task.ID,
					Action:    "reassigned",
					Field:     "short_id",
					Source:    candidate.sourceID,
					From:      strconv.Itoa(candidate.oldShortID),
					To:        strconv.Itoa(task.ShortID),
					Reason:    "ShortIDs are assigned once across active and reintegrated archive tasks",
				})
			}
			if task.ParentID != "" && !projectRetainedIDs[projectIndex][task.ParentID] {
				plan.Report.Entries = append(plan.Report.Entries, V5TaskRepairEntry{
					ProjectID: project.projectID,
					TaskID:    task.ID,
					Action:    "cleared",
					Field:     "parent_id",
					Source:    candidate.sourceID,
					From:      task.ParentID,
					Reason:    "parent does not exist in the retained project task set",
				})
				task.ParentID = ""
			}
			if len(task.DependsOn) > 0 {
				dependencies := task.DependsOn[:0]
				seen := make(map[string]bool, len(task.DependsOn))
				for _, dependencyRef := range task.DependsOn {
					_, dependencyID := splitV5TaskReference(dependencyRef)
					targetProjectID, retained := retainedTaskProjects[dependencyID]
					if !retained {
						plan.Report.Entries = append(plan.Report.Entries, V5TaskRepairEntry{
							ProjectID: project.projectID,
							TaskID:    task.ID,
							Action:    "cleared",
							Field:     "depends_on",
							Source:    candidate.sourceID,
							From:      dependencyRef,
							Reason:    "dependency does not exist in the retained workspace task set",
						})
						continue
					}
					if seen[dependencyID] {
						plan.Report.Entries = append(plan.Report.Entries, V5TaskRepairEntry{
							ProjectID: project.projectID,
							TaskID:    task.ID,
							Action:    "deduplicated",
							Field:     "depends_on",
							Source:    candidate.sourceID,
							From:      dependencyRef,
							To:        canonicalV5DependencyReference(project.projectID, targetProjectID, dependencyID),
							Reason:    "stable-ID remapping converged multiple dependency references",
						})
						continue
					}
					seen[dependencyID] = true
					canonicalRef := canonicalV5DependencyReference(project.projectID, targetProjectID, dependencyID)
					if canonicalRef != dependencyRef {
						plan.Report.Entries = append(plan.Report.Entries, V5TaskRepairEntry{
							ProjectID: project.projectID,
							TaskID:    task.ID,
							Action:    "qualified",
							Field:     "depends_on",
							Source:    candidate.sourceID,
							From:      dependencyRef,
							To:        canonicalRef,
							Reason:    "dependency reference uses the retained target project identity",
						})
					}
					dependencies = append(dependencies, canonicalRef)
				}
				task.DependsOn = dependencies
			}
			output.Tasks = append(output.Tasks, task)
		}
		plan.Projects[projectIndex] = output
		plan.Report.RetainedTasks += len(output.Tasks)
	}

	if plan.Report.InputTasks != plan.Report.RetainedTasks+plan.Report.OmittedTasks {
		return V5TaskReintegrationPlan{}, fmt.Errorf(
			"v5 task reintegration accounting mismatch: %d inputs != %d retained + %d omitted",
			plan.Report.InputTasks,
			plan.Report.RetainedTasks,
			plan.Report.OmittedTasks,
		)
	}

	sort.Slice(plan.Report.Entries, func(i, j int) bool {
		left, right := plan.Report.Entries[i], plan.Report.Entries[j]
		leftKey := []string{left.ProjectID, left.TaskID, left.Action, left.Field, left.Source, left.From, left.To, left.Reason}
		rightKey := []string{right.ProjectID, right.TaskID, right.Action, right.Field, right.Source, right.From, right.To, right.Reason}
		return slices.Compare(leftKey, rightKey) < 0
	})
	sort.Slice(cleanup, func(i, j int) bool {
		return cleanup[i].Path < cleanup[j].Path
	})
	plan.Cleanup = cleanup
	return plan, nil
}

func newV5TaskCandidate(
	projectIndex int,
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
		projectIndex: projectIndex,
		projectID:    projectID,
		sourceID:     sourceID,
		sourcePath:   sourcePath,
		ordinal:      ordinal,
		oldShortID:   ordinal + 1,
		task:         task,
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
