package tui

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestCrossMachineHelperSetsResumeNote is run twice at once, each side naming
// its own task and its own note. The gate pairs it with two tasks in one
// project, so both processes rewrite one tasks.md having each read it before the
// other committed: the hazard is a lost update, not a lost race. Both sides may
// legitimately commit — a three-way merge resolves edits to distinct tasks — and
// a side that does not has three admissible outcomes: it may expire, it may
// record a conflict bundle, or it may find the note already equal. What it may
// never do is report a commit whose note is not the one on disk, which is what a
// lost update looks like from inside the process that lost it.
//
// The barrier is installed inside SetResumeNote rather than wrapped around it.
// The locator resolved out here reads a snapshot the mutation then discards, so
// a pair released before the call would prove only that both processes had found
// a task: one of them could still read, commit and exit before the other read
// anything, and the "race" would be two sequential writes. Held at
// app.CommitBarrier, both processes are demonstrably holding the same
// document version with nothing yet written, which is what the revision each one
// publishes says.
func TestCrossMachineHelperSetsResumeNote(t *testing.T) {
	env := openHelperWorkspace(t)
	note := helperEnv(t, helperNoteEnv)
	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	defer helperHoldAtCommit(t, "Set Resume Note", locator.ProjectID, locator.TaskID)()

	result, err := env.service.SetResumeNote(locator, note)
	if err != nil {
		helperRecordRefusal(t, "SetResumeNote", err)
		return
	}
	if !result.Changed {
		helperRecord(t, "unchanged")
		return
	}
	// A commit that reported success has to be readable as this process's own
	// note, or "committed" would be a claim about nothing.
	stored := helperStoredResumeNote(t, env, locator)
	if stored != note {
		helperRecord(t, "lost: "+stored)
		t.Fatalf("stored resume note = %q, want the note this process committed", stored)
	}
	helperRecord(t, "committed")
}

// TestCrossMachineHelperAppliesTaskMutation is the resume-note role generalized
// to the three mutations this release added a key for: appending a comment,
// acknowledging a review, and creating a fresh task. All three are single-task
// writes into one shared tasks.md, so all three carry the same lost-update
// hazard and all three are held at the same seam; only the call and the
// on-disk claim differ, which is why they are one role selected by
// TT_HELPER_MUTATION rather than three near-copies.
//
// Creation is the sharpest of the three. The other two rewrite a task each
// machine already resolved, while two creations mint two identities and two
// short IDs against one document that named neither, so a merge that dropped
// one would be invisible to the machine that lost it. It is also the only one of
// the three whose loser is guaranteed to lose: both machines mint the same short
// id, so the second commit's merge is textually clean and semantically invalid,
// and the contract it proves is the classified refusal plus the retry.
func TestCrossMachineHelperAppliesTaskMutation(t *testing.T) {
	env := openHelperWorkspace(t)
	kind := helperEnv(t, helperMutationEnv)
	text := strings.TrimSpace(os.Getenv(helperTextEnv))
	title := helperEnv(t, helperTitleEnv)

	switch kind {
	case helperMutationComment:
		locator := helperTaskLocator(t, env, title)
		defer helperHoldAtCommit(t, "Add Comment", locator.ProjectID, locator.TaskID)()
		if _, err := env.service.AddComment(locator, text); err != nil {
			helperRecordRefusal(t, "AddComment", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			task := helperStoredTask(t, env, locator)
			return task != nil && slices.ContainsFunc(task.Comments, func(c *models.Comment) bool {
				return c != nil && c.Text == text
			})
		})
	case helperMutationReview:
		locator := helperTaskLocator(t, env, title)
		defer helperHoldAtCommit(t, "Mark Reviewed", locator.ProjectID, locator.TaskID)()
		if _, err := env.service.MarkReviewed(locator); err != nil {
			helperRecordRefusal(t, "MarkReviewed", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			task := helperStoredTask(t, env, locator)
			return task != nil && !task.Review
		})
	case helperMutationCreate:
		// The title names the project's existing task the new one joins, so the
		// creation lands in the document the other machine is rewriting.
		project := helperTaskLocator(t, env, title).ProjectID
		create := func() error {
			_, err := env.service.AddTask(app.AddTaskRequest{
				ProjectID: project,
				Title:     text,
				State:     models.StateBacklog,
			})
			return err
		}
		// No task axis: the identity this commit will carry is minted by the
		// commit the barrier is holding, so the project is all a caller can name.
		release := helperHoldAtCommit(t, "Add Task", project, "")
		err := create()
		// Released before the retry rather than deferred: the retry is a second
		// commit of the same undo action, and a barrier still installed would hold
		// it at a rendezvous the partner has already left.
		release()
		if err != nil {
			if !errors.Is(err, storage.ErrMergeConflict) {
				helperRecordRefusal(t, "AddTask", err)
				return
			}
			// Two machines minting an identity against one document produce the one
			// merge that is textually clean and still an invalid project: both
			// additions are non-overlapping, so both survive, and both answer to the
			// short id that was next free in the document neither had seen the other
			// write. The loser's whole commit is refused and recorded as a retry-only
			// bundle, so its task is nowhere yet and the retry — which reads what the
			// winner left — is what actually completes this creation.
			recorded := helperConflictIDs(err)
			if helperStoredTaskTitled(t, env, project, text) != nil {
				helperRecord(t, "lost: a refused creation wrote its task anyway")
				t.Fatal("a creation refused as a conflict left partial task data on disk")
			}
			if retryErr := create(); retryErr != nil {
				helperRecord(t, "failed: "+retryErr.Error())
				t.Fatalf("retrying a creation refused as %s: %v", recorded, retryErr)
			}
			helperRequireStored(t, kind, func() bool {
				return helperStoredTaskTitled(t, env, project, text) != nil
			})
			helperRecord(t, "retried: "+recorded)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return helperStoredTaskTitled(t, env, project, text) != nil
		})
	default:
		t.Fatalf("%s=%q names no helper mutation", helperMutationEnv, kind)
	}
	helperRecord(t, "committed")
}

// TestCrossMachineHelperAppliesProjectCommentMutation is the project-scoped
// sibling of the task-mutation role: two machines adding, editing, or deleting
// comments on one project, each held at the same commit seam its partner is.
//
// It is a role of its own rather than a fourth TT_HELPER_MUTATION value for two
// reasons. A project comment names no task, so it resolves its target through a
// comment identity rather than a locator and its barrier target carries no task
// id at all. And the two roles have to be able to be the two sides of one race —
// a project comment and a task edit rewriting one tasks.md is a hazard neither
// role can express alone — which they could not be if selecting one meant not
// selecting the other.
//
// The three calls differ in what a refusal has to prove. An addition and an edit
// each introduce text, so the claim is that a refused one left that text nowhere;
// a removal introduces none, so the claim is that the comment it addressed is
// still there, checked by identity rather than by text because the machine that
// beat it may have been an edit.
func TestCrossMachineHelperAppliesProjectCommentMutation(t *testing.T) {
	env := openHelperWorkspace(t)
	kind := helperEnv(t, helperProjectCommentEnv)
	projectID := helperProjectID(t, env, helperEnv(t, helperProjectEnv))
	text := strings.TrimSpace(os.Getenv(helperTextEnv))

	switch kind {
	case helperProjectCommentAdd:
		defer helperHoldAtCommit(t, "Add Project Comment", projectID, "")()
		if _, err := env.service.AddProjectComment(projectID, text); err != nil {
			helperRefusedWithoutWriting(t, env, projectID, "AddProjectComment", text, err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return helperProjectCommentCount(t, env, projectID, text) == 1
		})
	case helperProjectCommentEdit:
		target := helperProjectCommentTarget(t, env, projectID, helperEnv(t, helperTargetEnv))
		defer helperHoldAtCommit(t, "Edit Project Comment", projectID, "")()
		if _, err := env.service.EditProjectComment(target, text); err != nil {
			helperRefusedWithoutWriting(t, env, projectID, "EditProjectComment", text, err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			stored := helperProjectCommentByID(t, env, projectID, target.CommentID)
			return stored != nil && stored.Text == text
		})
	case helperProjectCommentRemove:
		target := helperProjectCommentTarget(t, env, projectID, helperEnv(t, helperTargetEnv))
		defer helperHoldAtCommit(t, "Remove Project Comment", projectID, "")()
		if _, err := env.service.RemoveProjectComment(target); err != nil {
			helperRequireStored(t, kind, func() bool {
				return helperProjectCommentByID(t, env, projectID, target.CommentID) != nil
			})
			helperRecordRefusal(t, "RemoveProjectComment", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return helperProjectCommentByID(t, env, projectID, target.CommentID) == nil
		})
	default:
		t.Fatalf("%s=%q names no project-comment mutation", helperProjectCommentEnv, kind)
	}
	helperRecord(t, "committed")
}

// helperProjectID finds one project by name. A name is what the gate script and
// the seed can both spell; the identity is minted by the seed.
func helperProjectID(t *testing.T, env helperEnvironment, name string) string {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		if project.Name == name {
			return project.ID
		}
	}
	t.Fatalf("no project named %q in %s", name, env.dataDir)
	return ""
}

// helperStoredProjectComments reads one project's comments back off disk, for
// the same reason every other stored claim reloads: the question is what the
// bytes say after the other machine's write merged into them.
func helperStoredProjectComments(t *testing.T, env helperEnvironment, projectID string) []*models.ProjectComment {
	t.Helper()
	loaded := helperReloadProject(t, env, projectID)
	if loaded == nil {
		return nil
	}
	return loaded.Comments
}

func helperProjectCommentByID(t *testing.T, env helperEnvironment, projectID, commentID string) *models.ProjectComment {
	t.Helper()
	for _, comment := range helperStoredProjectComments(t, env, projectID) {
		if comment != nil && comment.ID == commentID {
			return comment
		}
	}
	return nil
}

// helperProjectCommentCount is how "exactly once" is asserted. A merge that kept
// two machines' additions is correct; a merge that kept one of them twice is the
// duplication an identity-keyed reconciliation exists to prevent, and counting is
// the only way to tell the two apart from the outside.
func helperProjectCommentCount(t *testing.T, env helperEnvironment, projectID, text string) int {
	t.Helper()
	count := 0
	for _, comment := range helperStoredProjectComments(t, env, projectID) {
		if comment != nil && comment.Text == text {
			count++
		}
	}
	return count
}

// helperProjectCommentTarget addresses one stored comment by its current text,
// resolving the identity and the precondition the application layer re-checks.
// It is read before the barrier is installed, so the target is the document as it
// stood before either machine moved.
func helperProjectCommentTarget(t *testing.T, env helperEnvironment, projectID, text string) app.ProjectCommentTarget {
	t.Helper()
	for _, comment := range helperStoredProjectComments(t, env, projectID) {
		if comment != nil && comment.Text == text {
			return app.ProjectCommentTarget{ProjectID: projectID, CommentID: comment.ID, Original: comment.Text}
		}
	}
	t.Fatalf("no project comment %q on project %s", text, projectID)
	return app.ProjectCommentTarget{}
}

// helperRefusedWithoutWriting classifies a refusal only after proving it left
// nothing behind. A conflict that recorded a bundle and still wrote half its
// document would satisfy every outcome check the gate makes, so the absence is
// asserted here — inside the process that was refused — rather than inferred.
func helperRefusedWithoutWriting(t *testing.T, env helperEnvironment, projectID, call, unwritten string, err error) {
	t.Helper()
	if count := helperProjectCommentCount(t, env, projectID, unwritten); count != 0 {
		helperRecord(t, "lost: a refused project comment wrote its text anyway")
		t.Fatalf("%s was refused with %v but left %q on disk %d time(s)", call, err, unwritten, count)
	}
	helperRecordRefusal(t, call, err)
}
