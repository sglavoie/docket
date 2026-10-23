package app_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// uuidIDs mints canonical lowercase UUIDs from a counter, so a project-comment
// test gets deterministic identities that models.ValidateProjectComment accepts.
// The shared seqIDs generator cannot be used here: it produces gen-task-N, which
// the canonical writer refuses for a comment.
type uuidIDs struct{ n int }

func (u *uuidIDs) NewID() string {
	u.n++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", u.n)
}

// newProjectCommentService wires a Service whose generator mints comment-legal
// identities and whose clock is fixed, so a committed comment can be compared
// field by field.
func newProjectCommentService(repo *fakeRepo, now time.Time) *app.Service {
	return app.NewServiceWithArtifacts(repo, fixedClock{t: now}, &uuidIDs{}, repo.artifactPort())
}

func projectCommentFixtureTime() time.Time {
	return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
}

func TestAddProjectCommentCommitsOneComment(t *testing.T) {
	now := projectCommentFixtureTime()
	project := newProject("proj-alpha", "Alpha", newTask("task-1", 1, "Ship it"))
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	result, err := svc.AddProjectComment("proj-alpha", "  Kickoff is Monday  ")
	if err != nil {
		t.Fatalf("AddProjectComment: %v", err)
	}
	if result.ProjectID != "proj-alpha" {
		t.Errorf("ProjectID = %q, want proj-alpha", result.ProjectID)
	}
	if result.CommentID == "" || result.Revision == "" || result.TransactionID == "" {
		t.Fatalf("result is missing identity, revision, or transaction: %+v", result)
	}

	committed, ok := repo.lastCommit()
	if !ok {
		t.Fatal("the comment did not reach a single-project ChangeSet")
	}
	comments := committed.Project.Comments
	if len(comments) != 1 {
		t.Fatalf("committed comment count = %d, want 1", len(comments))
	}
	if comments[0].ID != result.CommentID {
		t.Errorf("committed comment id = %q, want %q", comments[0].ID, result.CommentID)
	}
	if comments[0].Text != "Kickoff is Monday" {
		t.Errorf("committed text = %q, want the trimmed text", comments[0].Text)
	}
	if !comments[0].CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %s, want the operation clock %s", comments[0].CreatedAt, now)
	}
	if got := len(repo.commits); got != 1 {
		t.Errorf("commit count = %d, want exactly one transaction", got)
	}
}

// A comment is a project-level value, so nothing about the project's tasks may
// move: not their touch times, not their completion state, not their root order.
//
// The fixture is deliberately one the post-mutation hooks would rewrite. A
// terminal parent over an open child is exactly what ReconcileParentCompletionAt
// reopens, and a done root ahead of an open one is exactly what
// ReorderRootTasksByCompletion swaps, so this test fails the moment the commit
// stops declaring its tasks untouched.
func TestAddProjectCommentLeavesTasksUntouched(t *testing.T) {
	now := projectCommentFixtureTime()
	stamp := now.Add(-48 * time.Hour)
	parent := newTask("task-parent", 1, "Done parent")
	parent.ApplyWorkflowState(models.StateDone)
	parent.StateChangedAt = stamp
	child := newTask("task-child", 2, "Open child")
	parentID := parent.ID
	child.ParentID = &parentID
	trailing := newTask("task-open-root", 3, "Open root")
	for _, task := range []*models.Task{parent, child, trailing} {
		task.CreatedAt = stamp
		task.UpdatedAt = stamp
	}
	project := newProject("proj-alpha", "Alpha", parent, child, trailing)
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	if _, err := svc.AddProjectComment("proj-alpha", "Note to self"); err != nil {
		t.Fatalf("AddProjectComment: %v", err)
	}
	committed, ok := repo.lastCommit()
	if !ok {
		t.Fatal("no ChangeSet was committed")
	}
	order := make([]string, 0, len(committed.Project.Tasks))
	for _, task := range committed.Project.Tasks {
		order = append(order, task.ID)
		if !task.UpdatedAt.Equal(stamp) {
			t.Errorf("task %s UpdatedAt = %s, want the untouched %s", task.ID, task.UpdatedAt, stamp)
		}
		if task.ID == parent.ID && task.State() != models.StateDone {
			t.Errorf("parent state = %s, want the untouched %s", task.State(), models.StateDone)
		}
	}
	want := []string{parent.ID, child.ID, trailing.ID}
	if !slicesEqualForTest(order, want) {
		t.Errorf("committed task order = %v, want the untouched %v", order, want)
	}
}

func slicesEqualForTest(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestAddProjectCommentAppendsBesideExistingOnes(t *testing.T) {
	now := projectCommentFixtureTime()
	project := newProject("proj-alpha", "Alpha")
	project.Comments = []*models.ProjectComment{{
		ID:        "11111111-1111-4111-8111-111111111111",
		Text:      "Older",
		CreatedAt: now.Add(-time.Hour),
	}}
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	if _, err := svc.AddProjectComment("proj-alpha", "Newer"); err != nil {
		t.Fatalf("AddProjectComment: %v", err)
	}
	committed, _ := repo.lastCommit()
	if len(committed.Project.Comments) != 2 {
		t.Fatalf("committed comment count = %d, want 2", len(committed.Project.Comments))
	}
	if committed.Project.Comments[0].Text != "Older" || committed.Project.Comments[1].Text != "Newer" {
		t.Errorf("committed order = %q, %q; want creation order",
			committed.Project.Comments[0].Text, committed.Project.Comments[1].Text)
	}
}

// Unicode and text the capture grammar would otherwise claim are stored
// verbatim: a quick comment is prose, not a capture line.
func TestAddProjectCommentStoresTextVerbatim(t *testing.T) {
	now := projectCommentFixtureTime()
	for name, text := range map[string]string{
		"unicode":  "Café ☕ — naïve 日本語 🚀",
		"sigils":   "Blocked on #auth and $done, see ~12 @today",
		"newlines": "First line\nSecond line\n\nFourth",
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo(newProject("proj-alpha", "Alpha"))
			svc := newProjectCommentService(repo, now)
			if _, err := svc.AddProjectComment("proj-alpha", text); err != nil {
				t.Fatalf("AddProjectComment: %v", err)
			}
			committed, _ := repo.lastCommit()
			if got := committed.Project.Comments[0].Text; got != text {
				t.Errorf("stored text = %q, want %q", got, text)
			}
		})
	}
}

// A parked project is an ordinary writable owner: parking hides a project from
// the base task workspace rather than sealing it. The Unfiled half of that rule
// is enforced a layer up, by the absent RejectSystemUnfiled requirement, because
// the application layer has no notion of which project the catalog protects.
func TestAddProjectCommentAdmitsAParkedProject(t *testing.T) {
	parked := newProject("proj-parked", "Parked")
	parked.Tags = []string{models.ParkedProjectTag}
	repo := newFakeRepo(parked)
	svc := newProjectCommentService(repo, projectCommentFixtureTime())

	if _, err := svc.AddProjectComment("proj-parked", "still writable"); err != nil {
		t.Errorf("AddProjectComment on a parked project: %v", err)
	}
}

func TestAddProjectCommentRefusesUnwritableOwners(t *testing.T) {
	now := projectCommentFixtureTime()
	archived := newProject("proj-archived", "Archived")
	archived.Archived = true
	repo := newFakeRepo(newProject("proj-alpha", "Alpha"))
	repo.withUnloaded(archived)
	// The archived project is deliberately reachable through the snapshot too, so
	// the app layer's own defensive refusal is exercised rather than the absence
	// the production repository would already have produced.
	repo.projects = append(repo.projects, archived)
	svc := newProjectCommentService(repo, now)

	if _, err := svc.AddProjectComment("proj-archived", "nope"); err == nil {
		t.Error("an archived project accepted a comment")
	} else if !strings.Contains(err.Error(), "archived") {
		t.Errorf("archived refusal = %v, want it to name archival", err)
	}

	_, err := svc.AddProjectComment("proj-missing", "nope")
	if !errors.Is(err, app.ErrProjectNotFound) {
		t.Errorf("missing project error = %v, want ErrProjectNotFound", err)
	}

	if _, err := svc.AddProjectComment("", "nope"); err == nil {
		t.Error("an empty project id was accepted")
	}

	for name, text := range map[string]string{"empty": "", "blank": "   \n\t  "} {
		if _, err := svc.AddProjectComment("proj-alpha", text); err == nil {
			t.Errorf("%s comment text was accepted", name)
		}
	}

	if len(repo.commits) != 0 {
		t.Errorf("commit count = %d, want no refused request to reach storage", len(repo.commits))
	}
}

// A generator that cannot mint a canonical UUID fails the request rather than
// staging a document the canonical writer would refuse.
func TestAddProjectCommentRefusesUnmintableIdentity(t *testing.T) {
	repo := newFakeRepo(newProject("proj-alpha", "Alpha"))
	svc := app.NewServiceWithArtifacts(
		repo, fixedClock{t: projectCommentFixtureTime()}, &seqIDs{}, repo.artifactPort(),
	)
	if _, err := svc.AddProjectComment("proj-alpha", "text"); err == nil {
		t.Fatal("a non-UUID comment identity was accepted")
	}
	if len(repo.commits) != 0 {
		t.Errorf("commit count = %d, want none", len(repo.commits))
	}
}

// A writer that moved the project between snapshot and commit loses the write,
// which is what lets the surface put the typed text back rather than silently
// clobbering the other machine's document.
func TestAddProjectCommentFailsClosedOnStaleRevision(t *testing.T) {
	now := projectCommentFixtureTime()
	project := newProject("proj-alpha", "Alpha")
	repo := newFakeRepo(project)
	repo.preCommit = func() { repo.revisions["proj-alpha"] = "someone-else-wrote-first" }
	svc := newProjectCommentService(repo, now)

	_, err := svc.AddProjectComment("proj-alpha", "mine")
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("error = %v, want a stale-revision conflict", err)
	}
}

// The committed comments are detached from the snapshot's own collection, so a
// refused or retried commit cannot leave the in-memory project mutated through a
// shared slice.
func TestAddProjectCommentDetachesTheCommittedCollection(t *testing.T) {
	now := projectCommentFixtureTime()
	existing := &models.ProjectComment{
		ID:        "11111111-1111-4111-8111-111111111111",
		Text:      "Older",
		CreatedAt: now.Add(-time.Hour),
	}
	project := newProject("proj-alpha", "Alpha")
	project.Comments = []*models.ProjectComment{existing}
	repo := newFakeRepo(project)
	svc := newProjectCommentService(repo, now)

	if _, err := svc.AddProjectComment("proj-alpha", "Newer"); err != nil {
		t.Fatalf("AddProjectComment: %v", err)
	}
	committed, _ := repo.lastCommit()
	if committed.Project.Comments[0] == existing {
		t.Error("the committed collection aliases the snapshot's own comment pointer")
	}
	if existing.Text != "Older" {
		t.Errorf("the snapshot's comment was mutated: %q", existing.Text)
	}
}
