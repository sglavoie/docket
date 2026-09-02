package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// taskContextEdit is an optional singleton-context write carried by a tag
// mutation. Riding the same mutate closure keeps a simultaneous tag and context
// change inside one transaction and one undo entry.
type taskContextEdit struct {
	Set   bool
	Value string
}

func (edit taskContextEdit) normalize() (taskContextEdit, error) {
	if !edit.Set || edit.Value == "" {
		return taskContextEdit{Set: edit.Set}, nil
	}
	normalized, err := models.NormalizeTaskContext(edit.Value)
	if err != nil {
		return taskContextEdit{}, invalidInput("context", err.Error())
	}
	return taskContextEdit{Set: true, Value: normalized}, nil
}

func (edit taskContextEdit) apply(t *models.Task) {
	if edit.Set {
		t.Context = edit.Value
	}
}

// bulkSetTags replaces the tag set on every selected task with tags, committing
// every touched project as ONE transaction. The tags are normalized, validated,
// and de-duplicated once up front — they are uniform across the batch — so a
// malformed tag aborts before any snapshot is mutated. An empty tags slice clears
// every task's tags. Tasks may span projects.
func (s *Service) bulkSetTags(
	locators []TaskLocator,
	tags []string,
	context taskContextEdit,
) (BulkResult, error) {
	normalized, err := normalizeBulkTags(tags)
	if err != nil {
		return BulkResult{}, err
	}
	contextEdit, err := context.normalize()
	if err != nil {
		return BulkResult{}, err
	}
	return s.bulkAttribute(locators, "Set Tags", "tags", nil, func(t *models.Task) {
		t.Tags = append([]string{}, normalized...)
		contextEdit.apply(t)
		t.UpdatedAt = s.clock.Now()
	})
}

// bulkPatchTags applies a tri-state tag edit to every selected task. Tags in
// add are made present everywhere, tags in remove are made absent everywhere,
// and all other memberships are preserved independently. The two sets are
// validated before any task is touched and every project commits in the same
// transaction.
func (s *Service) bulkPatchTags(
	locators []TaskLocator,
	add []string,
	remove []string,
	context taskContextEdit,
) (BulkResult, error) {
	normalizedAdd, err := normalizeBulkTags(add)
	if err != nil {
		return BulkResult{}, err
	}
	contextEdit, err := context.normalize()
	if err != nil {
		return BulkResult{}, err
	}
	normalizedRemove, err := normalizeTagList(remove)
	if err != nil {
		return BulkResult{}, err
	}
	removeSet := make(map[string]bool, len(normalizedRemove))
	for _, tag := range normalizedRemove {
		removeSet[tag] = true
	}
	for _, tag := range normalizedAdd {
		if removeSet[tag] {
			return BulkResult{}, invalidInput("tag", fmt.Sprintf("%q cannot be added and removed", tag))
		}
	}
	addSet := make(map[string]bool, len(normalizedAdd))
	for _, tag := range normalizedAdd {
		addSet[tag] = true
	}
	patchedTags := func(current []string) ([]string, error) {
		next := make([]string, 0, len(current)+len(normalizedAdd))
		for _, existing := range models.DedupeTags(current) {
			if !removeSet[existing] {
				next = append(next, existing)
			}
		}
		next = append(next, normalizedAdd...)
		return models.NormalizeTaskTags(next)
	}
	validate := func(ref TaskRef) error {
		if _, err := patchedTags(ref.Task.Tags); err != nil {
			return invalidInput("tag", err.Error())
		}
		return nil
	}
	return s.bulkAttribute(locators, "Set Tags", "tags", validate, func(t *models.Task) {
		t.Tags, _ = patchedTags(t.Tags)
		contextEdit.apply(t)
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkToggleTag applies one all-or-none toggle to a frozen target
// set. If every task already carries tag, it is removed everywhere; otherwise
// it is added everywhere. The membership decision and mutation share the same
// repository snapshot, so concurrent presentation state cannot split a bulk
// toggle into mixed results.
func (s *Service) BulkToggleTag(locators []TaskLocator, tag string) (BulkResult, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return BulkResult{}, invalidInput("tag", err.Error())
	}
	if len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture bulk tag toggle rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	remove := true
	for _, ref := range refs {
		remove = remove && ref.Task.HasTag(normalized)
	}
	now := s.clock.Now()
	for _, ref := range refs {
		if remove {
			ref.Task.RemoveTag(normalized)
		} else {
			ref.Task.AddTag(normalized)
		}
		ref.Task.UpdatedAt = now
	}

	action := "Add Tag"
	if remove {
		action = "Remove Tag"
	}
	commits, index := distinctProjectCommits(refs, snap)
	results, err := s.commitProjects(commits, action, normalized)
	if err != nil {
		return BulkResult{}, err
	}
	committed = true
	out := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(results.results, 0),
	}
	for _, ref := range refs {
		out.Tasks = append(out.Tasks, newTaskResult(
			ref,
			revisionAt(results.results, index[ref.Project.ID]),
			transactionIDAt(results.results, index[ref.Project.ID]),
		))
	}
	for _, commit := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, commit.project.ID)
	}
	return out, nil
}

func normalizeBulkTags(tags []string) ([]string, error) {
	normalized, err := models.NormalizeTaskTags(tags)
	if err != nil {
		return nil, invalidInput("tag", err.Error())
	}
	return normalized, nil
}
