package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

const (
	firstCommentID  = "11111111-1111-4111-8111-111111111111"
	secondCommentID = "22222222-2222-4222-8222-222222222222"
)

// twoCommentProject is the fixture both mutations are measured against: two
// comments with distinct identities and distinct instants, so a change that
// addressed the wrong one is visible in the committed document.
func twoCommentProject(now time.Time) *models.Project {
	project := newProject("proj-alpha", "Alpha")
	project.Comments = []*models.ProjectComment{
		{ID: firstCommentID, Text: "Older", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: secondCommentID, Text: "Newer", CreatedAt: now.Add(-time.Hour)},
	}
	return project
}

func editTarget(commentID, original string) app.ProjectCommentTarget {
	return app.ProjectCommentTarget{ProjectID: "proj-alpha", CommentID: commentID, Original: original}
}

func TestEditProjectCommentPreservesIdentityAndInstant(t *testing.T) {
	now := projectCommentFixtureTime()
	project := twoCommentProject(now)
	createdAt := project.Comments[0].CreatedAt
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	result, err := svc.EditProjectComment(editTarget(firstCommentID, "Older"), "  Older, revised  ")
	if err != nil {
		t.Fatalf("EditProjectComment: %v", err)
	}
	if result.CommentID != firstCommentID {
		t.Errorf("CommentID = %q, want the edited comment's own id", result.CommentID)
	}
	if result.Revision == "" || result.TransactionID == "" {
		t.Fatalf("result is missing revision or transaction: %+v", result)
	}

	committed, ok := repo.lastCommit()
	if !ok {
		t.Fatal("the edit did not reach a single-project ChangeSet")
	}
	comments := committed.Project.Comments
	if len(comments) != 2 {
		t.Fatalf("committed comment count = %d, want 2", len(comments))
	}
	if comments[0].ID != firstCommentID {
		t.Errorf("edited comment id = %q, want it preserved", comments[0].ID)
	}
	if !comments[0].CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %s, want the original instant %s", comments[0].CreatedAt, createdAt)
	}
	if comments[0].Text != "Older, revised" {
		t.Errorf("edited text = %q, want the trimmed replacement", comments[0].Text)
	}
	if comments[1].Text != "Newer" || comments[1].ID != secondCommentID {
		t.Errorf("the untouched comment moved: %+v", comments[1])
	}
	if len(repo.commits) != 1 {
		t.Errorf("commit count = %d, want exactly one transaction", len(repo.commits))
	}
}

// The mutation addresses its comment by identity, so the position it happens to
// occupy is irrelevant. This is the whole reason a target carries no index: the
// panel presents these newest-first, which is the reverse of the stored order.
func TestEditProjectCommentAddressesByIdentityNotPosition(t *testing.T) {
	now := projectCommentFixtureTime()
	repo := newFakeRepo(twoCommentProject(now))
	svc := newProjectCommentService(repo, now)

	if _, err := svc.EditProjectComment(editTarget(secondCommentID, "Newer"), "Newer, revised"); err != nil {
		t.Fatalf("EditProjectComment: %v", err)
	}
	committed, _ := repo.lastCommit()
	if committed.Project.Comments[0].Text != "Older" {
		t.Errorf("the first comment changed: %q", committed.Project.Comments[0].Text)
	}
	if committed.Project.Comments[1].Text != "Newer, revised" {
		t.Errorf("edited text = %q, want the second comment rewritten", committed.Project.Comments[1].Text)
	}
}

// A target whose original text no longer matches is refused whole. The user
// aimed their replacement at text that is no longer there, so applying it would
// silently discard whatever replaced it.
func TestProjectCommentMutationsRefuseAChangedTarget(t *testing.T) {
	now := projectCommentFixtureTime()
	repo := newFakeRepo(twoCommentProject(now))
	svc := newProjectCommentService(repo, now)

	_, err := svc.EditProjectComment(editTarget(firstCommentID, "what the panel drew"), "replacement")
	if err == nil {
		t.Fatal("an edit against stale original text was accepted")
	}
	if !strings.Contains(err.Error(), "changed") {
		t.Errorf("refusal = %v, want it to name the change", err)
	}

	if _, err = svc.RemoveProjectComment(editTarget(secondCommentID, "what the panel drew")); err == nil {
		t.Fatal("a delete against stale original text was accepted")
	}
	if len(repo.commits) != 0 {
		t.Errorf("commit count = %d, want no refused request to reach storage", len(repo.commits))
	}
}

// A comment deleted by another surface, an undo, or another machine is gone
// rather than replaced by whatever now sits at its position.
func TestProjectCommentMutationsRefuseAMissingComment(t *testing.T) {
	now := projectCommentFixtureTime()
	repo := newFakeRepo(twoCommentProject(now))
	svc := newProjectCommentService(repo, now)
	missing := "33333333-3333-4333-8333-333333333333"

	_, err := svc.EditProjectComment(editTarget(missing, "Older"), "replacement")
	if err == nil {
		t.Fatal("an edit named a comment the project does not hold")
	}
	if !strings.Contains(err.Error(), "no longer") {
		t.Errorf("refusal = %v, want it to name the absent comment", err)
	}
	if _, err = svc.RemoveProjectComment(editTarget(missing, "Older")); err == nil {
		t.Fatal("a delete named a comment the project does not hold")
	}
	if len(repo.commits) != 0 {
		t.Errorf("commit count = %d, want none", len(repo.commits))
	}
}

func TestProjectCommentMutationsRefuseUnwritableOwners(t *testing.T) {
	now := projectCommentFixtureTime()
	archived := twoCommentProject(now)
	archived.ID, archived.Name, archived.Archived = "proj-archived", "Archived", true
	repo := newFakeRepo(twoCommentProject(now))
	repo.withUnloaded(archived)
	repo.projects = append(repo.projects, archived)
	svc := newProjectCommentService(repo, now)

	archivedTarget := app.ProjectCommentTarget{ProjectID: "proj-archived", CommentID: firstCommentID, Original: "Older"}
	if _, err := svc.EditProjectComment(archivedTarget, "nope"); err == nil {
		t.Error("an archived project accepted a comment edit")
	} else if !strings.Contains(err.Error(), "archived") {
		t.Errorf("archived refusal = %v, want it to name archival", err)
	}
	if _, err := svc.RemoveProjectComment(archivedTarget); err == nil {
		t.Error("an archived project accepted a comment removal")
	}

	missingProject := app.ProjectCommentTarget{ProjectID: "proj-missing", CommentID: firstCommentID, Original: "Older"}
	if _, err := svc.EditProjectComment(missingProject, "nope"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Errorf("missing project error = %v, want ErrProjectNotFound", err)
	}
	if _, err := svc.RemoveProjectComment(missingProject); !errors.Is(err, app.ErrProjectNotFound) {
		t.Errorf("missing project error = %v, want ErrProjectNotFound", err)
	}

	for name, target := range map[string]app.ProjectCommentTarget{
		"no project":  {CommentID: firstCommentID, Original: "Older"},
		"no comment":  {ProjectID: "proj-alpha", Original: "Older"},
		"no original": {ProjectID: "proj-alpha", CommentID: firstCommentID},
	} {
		if _, err := svc.EditProjectComment(target, "replacement"); err == nil {
			t.Errorf("an edit with %s was accepted", name)
		}
		if _, err := svc.RemoveProjectComment(target); err == nil {
			t.Errorf("a delete with %s was accepted", name)
		}
	}

	for name, text := range map[string]string{"empty": "", "blank": "   \n\t  "} {
		if _, err := svc.EditProjectComment(editTarget(firstCommentID, "Older"), text); err == nil {
			t.Errorf("%s replacement text was accepted", name)
		}
	}

	if len(repo.commits) != 0 {
		t.Errorf("commit count = %d, want no refused request to reach storage", len(repo.commits))
	}
}

func TestRemoveProjectCommentRemovesOnlyItsOwn(t *testing.T) {
	now := projectCommentFixtureTime()
	repo := newFakeRepo(twoCommentProject(now))
	svc := newProjectCommentService(repo, now)

	result, err := svc.RemoveProjectComment(editTarget(firstCommentID, "Older"))
	if err != nil {
		t.Fatalf("RemoveProjectComment: %v", err)
	}
	if result.CommentID != firstCommentID {
		t.Errorf("CommentID = %q, want the removed comment's own id", result.CommentID)
	}
	committed, _ := repo.lastCommit()
	if len(committed.Project.Comments) != 1 {
		t.Fatalf("committed comment count = %d, want 1", len(committed.Project.Comments))
	}
	if committed.Project.Comments[0].ID != secondCommentID {
		t.Errorf("surviving comment = %q, want the one that was not addressed", committed.Project.Comments[0].ID)
	}
	if len(repo.commits) != 1 {
		t.Errorf("commit count = %d, want exactly one transaction", len(repo.commits))
	}
}

// Removing the last comment leaves an empty collection rather than a document
// the canonical writer would have to render an empty block for.
func TestRemoveProjectCommentEmptiesTheCollection(t *testing.T) {
	now := projectCommentFixtureTime()
	project := newProject("proj-alpha", "Alpha")
	project.Comments = []*models.ProjectComment{{ID: firstCommentID, Text: "Only", CreatedAt: now}}
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	if _, err := svc.RemoveProjectComment(editTarget(firstCommentID, "Only")); err != nil {
		t.Fatalf("RemoveProjectComment: %v", err)
	}
	committed, _ := repo.lastCommit()
	if len(committed.Project.Comments) != 0 {
		t.Errorf("committed comments = %+v, want none", committed.Project.Comments)
	}
}

// Both mutations are project-level values, so the tasks beside them may not move
// — not their touch times, not their completion state, not their root order. The
// fixture is one the post-mutation hooks would rewrite if they ran.
func TestProjectCommentMutationsLeaveTasksUntouched(t *testing.T) {
	now := projectCommentFixtureTime()
	build := func() *models.Project {
		project := twoCommentProject(now)
		stamp := now.Add(-72 * time.Hour)
		parent := newTask("task-parent", 1, "Parent")
		parent.ApplyWorkflowState(models.StateDone)
		parent.StateChangedAt = stamp
		child := newTask("task-child", 2, "Child")
		parentID := parent.ID
		child.ParentID = &parentID
		for _, task := range []*models.Task{parent, child} {
			task.CreatedAt = stamp
			task.UpdatedAt = stamp
		}
		project.Tasks = []*models.Task{parent, child}
		project.BuildTaskTree()
		return project
	}

	for name, mutate := range map[string]func(*app.Service) error{
		"edit": func(svc *app.Service) error {
			_, err := svc.EditProjectComment(editTarget(firstCommentID, "Older"), "revised")
			return err
		},
		"remove": func(svc *app.Service) error {
			_, err := svc.RemoveProjectComment(editTarget(firstCommentID, "Older"))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo(build())
			svc := newProjectCommentService(repo, now)
			if err := mutate(svc); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			committed, _ := repo.lastCommit()
			for _, task := range committed.Project.Tasks {
				if !task.UpdatedAt.Equal(now.Add(-72 * time.Hour)) {
					t.Errorf("task %s moved its touch time to %s", task.ID, task.UpdatedAt)
				}
			}
			if committed.Project.Tasks[0].State() != models.StateDone {
				t.Error("the terminal parent was reconciled by a comment mutation")
			}
		})
	}
}

// A writer that moved the project between snapshot and commit loses the write,
// which is what lets the panel put the user's replacement back rather than
// silently clobbering the other machine's document.
func TestProjectCommentMutationsFailClosedOnStaleRevision(t *testing.T) {
	now := projectCommentFixtureTime()
	for name, mutate := range map[string]func(*app.Service) error{
		"edit": func(svc *app.Service) error {
			_, err := svc.EditProjectComment(editTarget(firstCommentID, "Older"), "mine")
			return err
		},
		"remove": func(svc *app.Service) error {
			_, err := svc.RemoveProjectComment(editTarget(firstCommentID, "Older"))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo(twoCommentProject(now))
			repo.preCommit = func() { repo.revisions["proj-alpha"] = "someone-else-wrote-first" }
			if err := mutate(newProjectCommentService(repo, now)); !errors.Is(err, storage.ErrStaleRevision) {
				t.Fatalf("%s error = %v, want a stale-revision conflict", name, err)
			}
		})
	}
}

// The committed collection is detached from the snapshot's own, so a refused or
// retried commit cannot leave the in-memory project mutated through a shared
// pointer.
func TestProjectCommentMutationsDetachTheCommittedCollection(t *testing.T) {
	now := projectCommentFixtureTime()
	project := twoCommentProject(now)
	existing := project.Comments[0]
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	if _, err := svc.EditProjectComment(editTarget(firstCommentID, "Older"), "revised"); err != nil {
		t.Fatalf("EditProjectComment: %v", err)
	}
	committed, _ := repo.lastCommit()
	if committed.Project.Comments[0] == existing {
		t.Error("the committed collection aliases the snapshot's own comment pointer")
	}
	if existing.Text != "Older" {
		t.Errorf("the snapshot's comment was mutated: %q", existing.Text)
	}
}
