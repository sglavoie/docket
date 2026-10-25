package app

import (
	"fmt"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/util"
)

// TaskNameNamespace names the one vocabulary a rename acts in. Tags and the
// singleton task context share a canonical spelling but are independent axes:
// a rename never reads or writes the other namespace, so renaming the tag
// "work" leaves every "work" context untouched and the reverse.
type TaskNameNamespace string

const (
	// TaskTagNamespace renames memberships in a task's tag list.
	TaskTagNamespace TaskNameNamespace = "tag"
	// TaskContextNamespace renames the singleton task context field.
	TaskContextNamespace TaskNameNamespace = "context"
)

// reservedContextName is the context spelling the edit grammar permanently
// owns: `##0` clears the field, so no task may store or be renamed onto it.
const reservedContextName = "0"

// RenameTaskNameRequest asks for one exact-name change inside one namespace,
// across the whole workspace. From and To are normalized to their canonical
// spelling before anything is scanned, so the request names the same values a
// filter would have matched.
type RenameTaskNameRequest struct {
	Namespace TaskNameNamespace
	From      string
	To        string
}

// RenameTaskNamePreview is the frozen plan for one rename: what it will change,
// and the exact workspace state that answer was computed from. It is produced by
// PreviewTaskNameRename, shown to a person, and handed back to
// ApplyTaskNameRename unmodified.
//
// The preview expires rather than adapts. It carries the census it was derived
// from — every unarchived project at the revision it was read at, plus the
// catalog revision that enumerated them — and the apply commits those exact
// revisions as read-only guards. Any change to any of them between preview and
// apply, in an affected project or an unaffected one, aborts the commit with a
// stale-revision or stale-artifact conflict instead of renaming against a
// workspace nobody previewed. A refused apply mutates nothing and may simply be
// previewed again.
type RenameTaskNamePreview struct {
	// Namespace, From, and To are the normalized request this preview answers.
	Namespace TaskNameNamespace
	From      string
	To        string
	// Merge reports that at least one task already carries the destination name,
	// so the rename collapses two names into one rather than just respelling one.
	Merge bool
	// Tasks are the project-qualified identities that will change, in census
	// order. Its length is the affected task count.
	Tasks []TaskLocator
	// ProjectIDs are the projects owning those tasks, deduplicated in census
	// order. Its length is the affected project count.
	ProjectIDs []string
	// census is the frozen read set. It is unexported because a preview is only
	// meaningful as something this package produced: an adapter may hold one,
	// display it, and hand it back, but it may not assemble the guarded state a
	// commit will be validated against.
	census WorkspaceCensus
}

// RenameTaskNameResult reports what a committed rename touched. A no-op rename
// returns the zero value: no tasks, no projects, and no transaction.
type RenameTaskNameResult struct {
	Tasks             []TaskResult
	TouchedProjectIDs []string
	TransactionID     string
}

// RenameTaskName previews and immediately applies one rename. It is the whole
// operation for a caller that has already confirmed intent; a caller that shows
// what will change first calls PreviewTaskNameRename, presents it, and passes
// that same preview to ApplyTaskNameRename.
func (s *Service) RenameTaskName(req RenameTaskNameRequest) (RenameTaskNameResult, error) {
	preview, err := s.PreviewTaskNameRename(req)
	if err != nil {
		return RenameTaskNameResult{}, err
	}
	return s.ApplyTaskNameRename(preview)
}

// PreviewTaskNameRename freezes what one rename would change. It validates the
// request, takes one strict census of the workspace, and reports every task
// carrying the source name without writing anything.
func (s *Service) PreviewTaskNameRename(req RenameTaskNameRequest) (RenameTaskNamePreview, error) {
	normalized, err := normalizeRenameTaskNameRequest(req)
	if err != nil {
		return RenameTaskNamePreview{}, err
	}
	source, ok := s.repo.(WorkspaceCensusSource)
	if !ok {
		return RenameTaskNamePreview{}, fmt.Errorf("app: repository does not support a strict workspace census")
	}
	census, err := source.WorkspaceCensus()
	if err != nil {
		return RenameTaskNamePreview{}, err
	}

	preview := RenameTaskNamePreview{
		Namespace: normalized.Namespace,
		From:      normalized.From,
		To:        normalized.To,
		census:    census,
	}
	seenProjects := make(map[string]bool, len(census.Projects))
	for _, entry := range census.Projects {
		if entry.Project == nil {
			return RenameTaskNamePreview{}, fmt.Errorf("app: workspace census returned a project without state")
		}
		for _, task := range entry.Project.Tasks {
			if task == nil || task.ID == "" {
				continue
			}
			if taskCarriesName(task, normalized.Namespace, normalized.To) {
				preview.Merge = true
			}
			if !taskCarriesName(task, normalized.Namespace, normalized.From) {
				continue
			}
			preview.Tasks = append(preview.Tasks, TaskLocator{ProjectID: entry.Project.ID, TaskID: task.ID})
			if !seenProjects[entry.Project.ID] {
				seenProjects[entry.Project.ID] = true
				preview.ProjectIDs = append(preview.ProjectIDs, entry.Project.ID)
			}
		}
	}
	return preview, nil
}

// ApplyTaskNameRename commits the frozen preview as one transaction with one
// undo record. Only the projects that actually change are written; every census
// project — the written ones included — and the catalog itself ride along as
// read-only guards, so the whole workspace the preview described is proven
// unchanged under the commit locks before a single byte is staged.
func (s *Service) ApplyTaskNameRename(preview RenameTaskNamePreview) (RenameTaskNameResult, error) {
	normalized, err := normalizeRenameTaskNameRequest(RenameTaskNameRequest{
		Namespace: preview.Namespace,
		From:      preview.From,
		To:        preview.To,
	})
	if err != nil {
		return RenameTaskNameResult{}, err
	}
	if len(preview.Tasks) == 0 {
		// Nothing carries the source name. A no-op opens no transaction, so it
		// records no undo entry and cannot expire on someone else's write.
		return RenameTaskNameResult{}, nil
	}

	byProject := make(map[string]CensusProject, len(preview.census.Projects))
	for _, entry := range preview.census.Projects {
		if entry.Project == nil {
			return RenameTaskNameResult{}, fmt.Errorf("app: workspace census returned a project without state")
		}
		byProject[entry.Project.ID] = entry
	}

	// Mutate the frozen census in place and remember, in census order, which
	// projects a rename actually changed. A project whose tasks all end up
	// unchanged never becomes a write.
	changed := make(map[string]bool, len(preview.ProjectIDs))
	var results []TaskResult
	for _, locator := range preview.Tasks {
		entry, ok := byProject[locator.ProjectID]
		if !ok {
			return RenameTaskNameResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, locator.ProjectID)
		}
		task := findCensusTask(entry.Project, locator.TaskID)
		if task == nil {
			return RenameTaskNameResult{}, fmt.Errorf("%w: %s/%s", ErrTaskNotFound, locator.ProjectID, locator.TaskID)
		}
		if !renameTaskName(task, normalized.Namespace, normalized.From, normalized.To) {
			continue
		}
		changed[locator.ProjectID] = true
		results = append(results, TaskResult{
			TaskID:    task.ID,
			ShortID:   task.ShortID,
			ProjectID: entry.Project.ID,
			Title:     task.Title,
		})
	}
	if len(changed) == 0 {
		return RenameTaskNameResult{}, nil
	}

	changesets, touched := s.renameChangeSets(preview, normalized, changed)
	commitResults, err := s.repo.Commit(changesets...)
	if err != nil {
		return RenameTaskNameResult{}, err
	}
	transactionID := transactionIDAt(commitResults, 0)
	revisions := make(map[string]string, len(touched))
	for i, projectID := range touched {
		revisions[projectID] = revisionAt(commitResults, i)
	}
	for i := range results {
		results[i].Revision = revisions[results[i].ProjectID]
		results[i].TransactionID = transactionID
	}
	return RenameTaskNameResult{
		Tasks:             results,
		TouchedProjectIDs: touched,
		TransactionID:     transactionID,
	}, nil
}

// renameChangeSets renders the mutated census as one atomic commit: a changeset
// per changed project, in census order, plus the whole read set as guards. It
// deliberately does not run the post-mutation hooks — a vocabulary rename
// changes no state, no parentage, and no order, so reconciling completion or
// rewriting root order here would make an unrelated edit ride an undo entry
// nobody asked for.
func (s *Service) renameChangeSets(preview RenameTaskNamePreview, req RenameTaskNameRequest, changed map[string]bool) ([]storage.ChangeSet, []string) {
	action := renameUndoAction(req.Namespace, preview.Merge)
	context := fmt.Sprintf("%s → %s (%s in %s)", req.From, req.To, util.Count(len(preview.Tasks), "task"), util.Count(len(preview.ProjectIDs), "project"))

	var changesets []storage.ChangeSet
	var touched []string
	guards := make([]storage.ProjectRevisionPrecondition, 0, len(preview.census.Projects))
	for _, entry := range preview.census.Projects {
		// Every census project is guarded, the ones this commit rewrites included.
		// A changed project's own ExpectedRevision names the same revision but not
		// the same contract: it opts into three-way merging, which would silently
		// rename against a document nobody previewed. The guard is what makes a
		// stale affected project expire exactly like a stale unaffected one.
		guards = append(guards, storage.ProjectRevisionPrecondition{
			ProjectID:        entry.Project.ID,
			ProjectName:      entry.Project.Name,
			Path:             entry.Project.Path,
			ExpectedRevision: entry.Revision,
		})
		if !changed[entry.Project.ID] {
			continue
		}
		changesets = append(changesets, storage.ChangeSet{
			Project:          entry.Project,
			ExpectedRevision: entry.Revision,
			Base:             entry.Document,
			UndoAction:       action,
			UndoContext:      context,
		})
		touched = append(touched, entry.Project.ID)
	}

	// Every guard hangs off the first changeset: preconditions are validated for
	// the whole commit in one phase, so spreading them over changesets would only
	// obscure that they describe one read set.
	changesets[0].ProjectPreconditions = guards
	if preview.census.CatalogPath != "" {
		changesets[0].ArtifactPreconditions = []storage.ArtifactPrecondition{{
			Path:             preview.census.CatalogPath,
			ExpectedRevision: preview.census.CatalogRevision,
		}}
	}
	return changesets, touched
}

func renameUndoAction(namespace TaskNameNamespace, merge bool) string {
	switch {
	case namespace == TaskContextNamespace && merge:
		return "Merge Contexts"
	case namespace == TaskContextNamespace:
		return "Rename Context"
	case merge:
		return "Merge Tags"
	default:
		return "Rename Tag"
	}
}

// normalizeRenameTaskNameRequest validates the namespace and canonicalizes both
// names through the same rules the capture grammar and the filters use, so a
// request naming "Work" renames exactly what "#work" matches.
func normalizeRenameTaskNameRequest(req RenameTaskNameRequest) (RenameTaskNameRequest, error) {
	switch req.Namespace {
	case TaskTagNamespace, TaskContextNamespace:
	default:
		return RenameTaskNameRequest{}, invalidInput("namespace", fmt.Sprintf("%q is not a task name namespace", req.Namespace))
	}
	field := string(req.Namespace)
	from, err := normalizeTaskName(req.Namespace, req.From)
	if err != nil {
		return RenameTaskNameRequest{}, invalidInput(field, err.Error())
	}
	to, err := normalizeTaskName(req.Namespace, req.To)
	if err != nil {
		return RenameTaskNameRequest{}, invalidInput(field, err.Error())
	}
	if from == to {
		return RenameTaskNameRequest{}, invalidInput(field, fmt.Sprintf("%q already is the destination name", from))
	}
	return RenameTaskNameRequest{Namespace: req.Namespace, From: from, To: to}, nil
}

func normalizeTaskName(namespace TaskNameNamespace, raw string) (string, error) {
	if namespace == TaskContextNamespace {
		name, err := models.NormalizeTaskContext(raw)
		if err != nil {
			return "", err
		}
		if name == reservedContextName {
			return "", fmt.Errorf("context %q is reserved: ##0 clears the field", reservedContextName)
		}
		return name, nil
	}
	return models.NormalizeTaskTag(raw)
}

// taskCarriesName reports whether one task holds a canonical name in one
// namespace, using the same comparison the filters do.
func taskCarriesName(task *models.Task, namespace TaskNameNamespace, name string) bool {
	if namespace == TaskContextNamespace {
		return task.HasContext(name)
	}
	return task.HasTag(name)
}

// renameTaskName replaces one canonical name with another on one task and
// reports whether the stored value changed. It assigns the fields directly: the
// Task mutators stamp UpdatedAt, and a vocabulary rename is not activity on the
// task it re-addresses, so every timestamp — UpdatedAt and the StateChangedAt
// clock alike — is left exactly as it was.
func renameTaskName(task *models.Task, namespace TaskNameNamespace, from, to string) bool {
	if namespace == TaskContextNamespace {
		if !task.HasContext(from) {
			return false
		}
		task.Context = to
		return true
	}
	renamed, changed := models.ReplaceTag(task.Tags, from, to)
	if !changed {
		return false
	}
	task.Tags = renamed
	return true
}

func findCensusTask(project *models.Project, taskID string) *models.Task {
	for _, task := range project.Tasks {
		if task != nil && task.ID == taskID {
			return task
		}
	}
	return nil
}
