package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const (
	// resumeBarrierNoteA and resumeBarrierNoteB are the two notes the race
	// commits. They have to be distinguishable in one tasks.md, because "machine
	// A's note survived" is checked by reading the file both machines rewrote.
	resumeBarrierNoteA = "picking the migration plan back up"
	resumeBarrierNoteB = "waiting on legal to answer the vendor"
)

// TestCrossMachineResumeBarrierFreezesMutationSnapshots is the in-suite proof
// that the cross-machine resume-note case is the lost-update race it claims to
// be rather than two sequential writes.
//
// The distinction is the whole point. SetResumeNote loads its own snapshot, so a
// barrier placed around the call releases two processes that have resolved a
// task and nothing more: one of them may then read, commit and exit before the
// other has read the document it is about to rewrite, and every assertion the
// gate makes still passes. Holding both processes at app.CommitBarrier —
// after each snapshot, before either commit — makes the overlap observable, and
// the base revision each side publishes from inside that seam is the evidence:
// two equal revisions, both equal to the document as it stood before either
// process started, mean neither had written when the other read.
//
// It forks the real thing. The two mutating parties are this package's own test
// binary re-entered as TestCrossMachineHelperSetsResumeNote, which is the role
// scripts/cross_machine_gate.sh forks, so the coordination proven here is the
// coordination the gate depends on. internal/tui's TestMain gives each process
// its own $HOME, which is exactly what makes them two machines.
func TestCrossMachineResumeBarrierFreezesMutationSnapshots(t *testing.T) {
	runCrossMachineRace(t, []crossMachineRole{
		{
			name: "A",
			role: "TestCrossMachineHelperSetsResumeNote",
			env:  []string{helperTitleEnv + "=" + helperRaceTitleA, helperNoteEnv + "=" + resumeBarrierNoteA},
			confirm: func(t *testing.T, project *models.Project) {
				requireCrossMachineResumeNote(t, project, helperRaceTitleA, resumeBarrierNoteA)
			},
		},
		{
			name: "B",
			role: "TestCrossMachineHelperSetsResumeNote",
			env:  []string{helperTitleEnv + "=" + helperRaceTitleB, helperNoteEnv + "=" + resumeBarrierNoteB},
			confirm: func(t *testing.T, project *models.Project) {
				requireCrossMachineResumeNote(t, project, helperRaceTitleB, resumeBarrierNoteB)
			},
		},
	})
}

// TestCrossMachineMutationBarrierFreezesNewMutations puts the three mutations
// this release added a key for through the same seam, one race each: two
// machines appending comments, two acknowledging reviews, and two creating tasks
// into one project.
//
// They are three races and not one because the hazard is not the same shape in
// each. A comment appends to a list a merge can interleave, an acknowledgement
// rewrites a flag one of the two writes must win, and a creation mints an
// identity and a short ID against a document that named neither — so a merge
// that dropped one creation would leave the losing machine with a committed
// task that is nowhere, which is precisely the lost update the barrier exists to
// expose. Each race gets its own workspace: a shared one would let an earlier
// race's commits decide a later one's merge.
func TestCrossMachineMutationBarrierFreezesNewMutations(t *testing.T) {
	const (
		commentA = "checked this against the vendor thread"
		commentB = "legal wants the indemnity clause first"
		createdA = "Draft the rollout announcement"
		createdB = "Book the contract review slot"
	)
	mutationRole := func(name, kind, title, text string, confirm func(*testing.T, *models.Project)) crossMachineRole {
		return crossMachineRole{
			name: name,
			role: "TestCrossMachineHelperAppliesTaskMutation",
			env: []string{
				helperMutationEnv + "=" + kind,
				helperTitleEnv + "=" + title,
				helperTextEnv + "=" + text,
			},
			confirm: confirm,
		}
	}

	for _, tc := range []struct {
		name  string
		roles []crossMachineRole
		// judge is the claim only this mutation can make. The shared race already
		// checks that nobody lost an update and that somebody committed; creation is
		// the one case whose loser is guaranteed and whose refusal has a shape worth
		// naming.
		judge func(*testing.T, crossMachineRaceResult)
	}{
		{
			name: "comment",
			roles: []crossMachineRole{
				mutationRole("A", helperMutationComment, helperRaceTitleA, commentA, func(t *testing.T, project *models.Project) {
					requireCrossMachineComment(t, project, helperRaceTitleA, commentA)
				}),
				mutationRole("B", helperMutationComment, helperRaceTitleB, commentB, func(t *testing.T, project *models.Project) {
					requireCrossMachineComment(t, project, helperRaceTitleB, commentB)
				}),
			},
		},
		{
			name: "review",
			roles: []crossMachineRole{
				mutationRole("A", helperMutationReview, helperRaceTitleA, "", func(t *testing.T, project *models.Project) {
					requireCrossMachineAcknowledged(t, project, helperRaceTitleA)
				}),
				mutationRole("B", helperMutationReview, helperRaceTitleB, "", func(t *testing.T, project *models.Project) {
					requireCrossMachineAcknowledged(t, project, helperRaceTitleB)
				}),
			},
		},
		{
			// Both machines name the same anchor task, so both creations land in the
			// one document, which is what makes this a race and not two appends to
			// two files.
			name: "create",
			roles: []crossMachineRole{
				mutationRole("A", helperMutationCreate, helperRaceTitleA, createdA, func(t *testing.T, project *models.Project) {
					requireCrossMachineCreated(t, project, createdA)
				}),
				mutationRole("B", helperMutationCreate, helperRaceTitleA, createdB, func(t *testing.T, project *models.Project) {
					requireCrossMachineCreated(t, project, createdB)
				}),
			},
			judge: requireCrossMachineCreationWasClassifiedAndRetried,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			race := runCrossMachineRace(t, tc.roles)
			requireCrossMachineDistinctShortIDs(t, race.merged)
			if tc.judge != nil {
				tc.judge(t, race)
			}
		})
	}
}

// The five project-comment races below all rewrite one project's own comment
// collection, so the texts have to be distinguishable in the one tasks.md every
// machine rewrote.
const (
	projectCommentAddedA     = "Machine A logged the migration review"
	projectCommentAddedB     = "Machine B heard back from the vendor"
	projectCommentEditedA    = "Kickoff notes moved to the team wiki"
	projectCommentEditedB    = "The contract review slipped to the 19th"
	projectCommentRivalEditA = "Kickoff notes are in the shared drive still"
	projectCommentTaskText   = "checked this against the vendor thread"
)

// TestCrossMachineProjectCommentBarrierFreezesCommentMutations puts project
// comments through the same commit seam the task mutations go through, one race
// per way two machines can collide over one comment collection.
//
// They are five races and not one because the merge resolves them by five
// different rules, and only two of those rules can be read off a passing task
// race. Independent additions must both survive and survive exactly once, which
// is the claim identity-keyed reconciliation exists to make and the one an
// index-keyed merge would break silently. Edits to different comments must both
// land, which proves the identity key is doing the work rather than the ordering.
// Divergent edits to one comment, and an edit against a deletion, must each be
// refused whole with the divergence recorded — a merge that guessed would lose a
// user's words with nothing to show. And a project comment racing a task comment
// proves the collection and the task list reconcile independently inside one
// document, which is the case no single-axis race can reach.
//
// Each race gets its own workspace, for the reason the task races do: a shared
// one would let an earlier race's merge decide a later one's.
func TestCrossMachineProjectCommentBarrierFreezesCommentMutations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		roles []crossMachineRole
		judge func(*testing.T, crossMachineRaceResult)
	}{
		{
			// Two additions carry two minted identities, so the merge has nothing to
			// reconcile and both must land. The claim is deliberately "exactly once":
			// a merge that appended a comment it had already kept would leave both
			// machines' words on disk and still be wrong.
			name: "independent-additions",
			roles: []crossMachineRole{
				projectCommentRole("A", helperProjectCommentAdd, "", projectCommentAddedA),
				projectCommentRole("B", helperProjectCommentAdd, "", projectCommentAddedB),
			},
			judge: func(t *testing.T, race crossMachineRaceResult) {
				requireBothMachinesCommitted(t, race)
				requireCrossMachineProjectComments(t, race.merged,
					helperSeedCommentA, helperSeedCommentB, projectCommentAddedA, projectCommentAddedB)
			},
		},
		{
			// Edits to two different comments are independent writes wearing an
			// edit's clothes: the identities differ, so both are adopted.
			name: "distinct-comment-edits",
			roles: []crossMachineRole{
				projectCommentRole("A", helperProjectCommentEdit, helperSeedCommentA, projectCommentEditedA),
				projectCommentRole("B", helperProjectCommentEdit, helperSeedCommentB, projectCommentEditedB),
			},
			judge: func(t *testing.T, race crossMachineRaceResult) {
				requireBothMachinesCommitted(t, race)
				requireCrossMachineProjectComments(t, race.merged, projectCommentEditedA, projectCommentEditedB)
			},
		},
		{
			// One comment, two rewrites: the merge cannot choose, so the loser is
			// refused whole and the divergence is recorded against the comment's own
			// identity rather than against a task nobody touched.
			name: "same-comment-edits",
			roles: []crossMachineRole{
				projectCommentRole("A", helperProjectCommentEdit, helperSeedCommentA, projectCommentEditedA),
				projectCommentRole("B", helperProjectCommentEdit, helperSeedCommentA, projectCommentRivalEditA),
			},
			judge: func(t *testing.T, race crossMachineRaceResult) {
				requireOneMachineRefused(t, race, projectCommentTextField)
				requireCrossMachineProjectCommentCount(t, race.merged, 2)
			},
		},
		{
			// A presence race: one machine rewrote the comment the other deleted, so
			// neither answer can be inferred from the other's and the whole value is
			// what the recorded conflict names.
			name: "edit-versus-delete",
			roles: []crossMachineRole{
				projectCommentRole("A", helperProjectCommentEdit, helperSeedCommentA, projectCommentEditedA),
				projectCommentRole("B", helperProjectCommentRemove, helperSeedCommentA, ""),
			},
			judge: func(t *testing.T, race crossMachineRaceResult) {
				requireOneMachineRefused(t, race, projectCommentWholeField)
				// The survivor decides the count, and both survivors are legitimate:
				// the edit winning leaves two comments, the deletion winning leaves one.
				if got := len(race.merged.Comments); got != 1 && got != 2 {
					t.Fatalf("edit-versus-delete left %d comment(s): %v", got, crossMachineProjectCommentTexts(race.merged))
				}
			},
		},
		{
			// The mixed race. A project comment and a task comment rewrite one
			// tasks.md from two machines, and the two axes reconcile independently:
			// the comment collection and the task list are merged separately, so
			// neither write costs the other.
			name: "comment-versus-task",
			roles: []crossMachineRole{
				projectCommentRole("A", helperProjectCommentAdd, "", projectCommentAddedA),
				{
					name: "B",
					role: "TestCrossMachineHelperAppliesTaskMutation",
					env: []string{
						helperMutationEnv + "=" + helperMutationComment,
						helperTitleEnv + "=" + helperRaceTitleB,
						helperTextEnv + "=" + projectCommentTaskText,
					},
					confirm: func(t *testing.T, project *models.Project) {
						requireCrossMachineComment(t, project, helperRaceTitleB, projectCommentTaskText)
					},
				},
			},
			judge: func(t *testing.T, race crossMachineRaceResult) {
				requireBothMachinesCommitted(t, race)
				requireCrossMachineProjectComments(t, race.merged,
					helperSeedCommentA, helperSeedCommentB, projectCommentAddedA)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			race := runCrossMachineRace(t, tc.roles)
			requireCrossMachineDistinctShortIDs(t, race.merged)
			tc.judge(t, race)
		})
	}
}

// The two conflict fields a project-comment race can record. internal/storage
// owns the spellings; they are repeated here rather than exported because a test
// in another package asserting on them is exactly the coupling an exported
// constant would make permanent.
const (
	projectCommentWholeField = "comment"
	projectCommentTextField  = "comment.text"
)

// projectCommentRole names one forked machine running the project-comment role.
// Every project-comment race is over one project — the collection lives in its
// document — so the project is fixed here rather than passed at each call.
func projectCommentRole(name, kind, target, text string) crossMachineRole {
	return crossMachineRole{
		name: name,
		role: "TestCrossMachineHelperAppliesProjectCommentMutation",
		env: []string{
			helperProjectEnv + "=" + helperRaceProjectName,
			helperProjectCommentEnv + "=" + kind,
			helperTargetEnv + "=" + target,
			helperTextEnv + "=" + text,
		},
		confirm: func(t *testing.T, project *models.Project) {
			switch kind {
			case helperProjectCommentRemove:
				requireCrossMachineProjectCommentAbsent(t, project, target)
			default:
				requireCrossMachineProjectComment(t, project, text)
			}
		},
	}
}

// requireBothMachinesCommitted is the claim a race with no legitimate loser
// makes. runCrossMachineRace admits a loser by design, so a race whose merge
// rules resolve every collision has to say so itself or a silent regression to
// "one of them conflicted" would keep passing.
func requireBothMachinesCommitted(t *testing.T, race crossMachineRaceResult) {
	t.Helper()
	for index, outcome := range race.outcomes {
		if outcome != "committed" {
			t.Fatalf("machine %d reported %q; this race has no legitimate loser (%v)", index, outcome, race.outcomes)
		}
	}
}

// requireOneMachineRefused is the claim of the two races the merge cannot
// resolve: exactly one machine commits, and the other is refused with the
// divergence durably recorded against the comment's identity.
//
// The bundle is read off disk rather than taken from the loser's word, and the
// field it names is checked, because "a conflict happened" is the weak half of
// the contract — a comment race recorded as a task conflict would leave `tt
// conflict` naming a task title nobody edited.
func requireOneMachineRefused(t *testing.T, race crossMachineRaceResult, field string) {
	t.Helper()
	committed, conflicted := 0, 0
	for _, outcome := range race.outcomes {
		switch {
		case outcome == "committed":
			committed++
		case strings.HasPrefix(outcome, "conflicted:"):
			conflicted++
		}
	}
	if committed != 1 || conflicted != 1 {
		t.Fatalf("race outcomes = %v, want one committed and one conflicted", race.outcomes)
	}

	records, err := storage.NewConflictStore(config.ConflictsDir(race.dataDir)).List()
	if err != nil {
		t.Fatalf("list recorded conflicts: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("recorded conflict bundles = %d, want the one the refused comment wrote", len(records))
	}
	found := false
	for _, conflict := range records[0].Conflicts {
		if conflict.Kind != storage.MergeConflictProjectMetadata || conflict.Field != field {
			continue
		}
		if conflict.CommentID == "" {
			t.Fatalf("the recorded conflict names no comment: %+v", conflict)
		}
		if conflict.TaskTitle != "" {
			t.Fatalf("a comment conflict was recorded as a task conflict: %+v", conflict)
		}
		found = true
	}
	if !found {
		t.Fatalf("recorded bundle does not name a %q divergence: %+v", field, records[0].Conflicts)
	}
}

func crossMachineProjectCommentTexts(project *models.Project) []string {
	texts := make([]string, 0, len(project.Comments))
	for _, comment := range project.Comments {
		if comment != nil {
			texts = append(texts, comment.Text)
		}
	}
	return texts
}

// requireCrossMachineProjectComment is one machine's own claim: the comment it
// committed is on disk, exactly once. The count is the point — a merge that kept
// a comment twice would satisfy a presence check and would have duplicated a
// user's words.
func requireCrossMachineProjectComment(t *testing.T, project *models.Project, text string) {
	t.Helper()
	count := 0
	for _, stored := range crossMachineProjectCommentTexts(project) {
		if stored == text {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("project comment %q appears %d time(s) after the race: %v",
			text, count, crossMachineProjectCommentTexts(project))
	}
}

func requireCrossMachineProjectCommentAbsent(t *testing.T, project *models.Project, text string) {
	t.Helper()
	for _, stored := range crossMachineProjectCommentTexts(project) {
		if stored == text {
			t.Fatalf("project comment %q survived the removal that committed", text)
		}
	}
}

// requireCrossMachineProjectComments is the whole-collection claim, for the races
// whose outcome is settled: every expected text present exactly once and nothing
// else stored. Per-machine confirmations cannot make it, because neither machine
// knows what the other was told to write.
func requireCrossMachineProjectComments(t *testing.T, project *models.Project, want ...string) {
	t.Helper()
	for _, text := range want {
		requireCrossMachineProjectComment(t, project, text)
	}
	requireCrossMachineProjectCommentCount(t, project, len(want))
}

func requireCrossMachineProjectCommentCount(t *testing.T, project *models.Project, want int) {
	t.Helper()
	if got := len(project.Comments); got != want {
		t.Fatalf("the merged project holds %d comment(s), want %d: %v",
			got, want, crossMachineProjectCommentTexts(project))
	}
}

// requireCrossMachineCreationWasClassifiedAndRetried is the creation race's own
// contract, and the only outcome pair it admits.
//
// Both machines mint the next free short id against the same document, so the
// second commit is not merely at risk of losing — it cannot succeed. Its merge is
// textually clean (two non-overlapping additions) and the whole it produces is
// invalid (two tasks answering to one number). The commit boundary must therefore
// refuse it as a recorded conflict rather than as a bare rendering failure, which
// is what leaves the losing machine something to show and a defined next step;
// and the retry, reading the document the winner left, is what completes the
// creation. Exactly one committed and exactly one retried is the whole claim.
func requireCrossMachineCreationWasClassifiedAndRetried(t *testing.T, race crossMachineRaceResult) {
	t.Helper()
	committed, retried := 0, 0
	for _, outcome := range race.outcomes {
		switch {
		case outcome == "committed":
			committed++
		case strings.HasPrefix(outcome, "retried:"):
			retried++
		}
	}
	if committed != 1 || retried != 1 {
		t.Fatalf("creation race outcomes = %v, want one committed and one retried", race.outcomes)
	}

	// The bundle is read off disk rather than taken from the helper's word: a
	// classified refusal that recorded nothing would leave `tt conflict` with
	// nothing to list, which is the whole difference this classification makes.
	records, err := storage.NewConflictStore(config.ConflictsDir(race.dataDir)).List()
	if err != nil {
		t.Fatalf("list recorded conflicts: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("recorded conflict bundles = %d, want the one the refused creation wrote", len(records))
	}
	if records[0].Action != "Add Task" {
		t.Fatalf("recorded bundle action = %q, want the refused mutation's own %q", records[0].Action, "Add Task")
	}
	classified := false
	for _, conflict := range records[0].Conflicts {
		if conflict.Kind == storage.MergeConflictInvariant && conflict.Field == "short_id" {
			classified = true
		}
	}
	if !classified {
		t.Fatalf("recorded bundle does not name the short id invariant: %+v", records[0].Conflicts)
	}
}

// crossMachineRaceResult is what a finished race leaves a caller to judge: the
// document every machine rewrote, the workspace it lives in — a conflict bundle
// is durable state beside it, not something a helper's word can prove — and each
// machine's reported outcome in the order the roles were given.
type crossMachineRaceResult struct {
	merged   *models.Project
	dataDir  string
	outcomes []string
}

// runCrossMachineRace forks the named machines over one freshly seeded
// workspace, proves they are all held at the same untouched document, releases
// them together, and judges what each one reported against the merged file. It
// returns that merged project so a caller can make a claim about the whole
// document rather than about one machine's own write.
func runCrossMachineRace(t *testing.T, roles []crossMachineRole) crossMachineRaceResult {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("locate this test binary: %v", err)
	}

	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	project, err := seedCrossMachineWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	openCrossMachineWorkspaceOnce(t, workspace.DataDir)

	// The revision the document stands at before any process starts. Every helper
	// must report it from inside the barrier: a helper reporting anything else
	// read a document another one had already rewritten.
	baseRevision := crossMachineProjectRevision(t, project)

	barrierDir := t.TempDir()
	proceed := filepath.Join(barrierDir, "go")
	helpers := make([]*crossMachineHelper, 0, len(roles))
	for _, spec := range roles {
		helpers = append(helpers, startCrossMachineHelper(t, binary, workspace.DataDir, barrierDir, proceed, spec))
	}

	for _, helper := range helpers {
		helper.awaitBarrier(t)
	}
	for _, helper := range helpers {
		if helper.revision != baseRevision {
			t.Fatalf("machine %s reached the mutation boundary at revision %q, want the pre-race %q",
				helper.name, helper.revision, baseRevision)
		}
		// A result line is recorded only after the mutation returns, so an outcome
		// that exists while every process is held would mean one of them had
		// already finished the mutation the others are only now beginning.
		if _, statErr := os.Stat(helper.result); statErr == nil {
			t.Fatalf("machine %s recorded an outcome before the barrier opened", helper.name)
		}
	}
	// The other half of "frozen": the snapshots are taken and the document is
	// still untouched. Equal published revisions alone would also be produced by
	// a barrier that fired after both commits, since each side reports the
	// revision it captured rather than the one on disk. Reading the file while
	// both machines are held is what separates the two.
	if held := crossMachineProjectRevision(t, project); held != baseRevision {
		t.Fatalf("the document moved to %q while both machines were held; the barrier does not precede the commit", held)
	}

	if err = os.WriteFile(proceed, []byte("go\n"), storage.FilePermissions); err != nil {
		t.Fatalf("release the barrier: %v", err)
	}
	for _, helper := range helpers {
		helper.awaitExit(t)
	}

	// Loaded after every process exited, so a confirmed commit is one that
	// survived the other machines finishing rather than one that merely was on
	// disk when its own process looked.
	merged := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err = storage.LoadProject(merged); err != nil {
		t.Fatalf("reload %s after the race: %v", project.Name, err)
	}
	committed := 0
	outcomes := make([]string, 0, len(helpers))
	for _, helper := range helpers {
		outcome := helper.outcome(t)
		outcomes = append(outcomes, outcome)
		switch {
		case strings.HasPrefix(outcome, "lost"):
			t.Fatalf("machine %s reported a commit that was silently replaced: %s", helper.name, outcome)
		case outcome == "committed", strings.HasPrefix(outcome, "retried:"):
			// A retry that landed is a commit whose first attempt was refused whole,
			// so the same claim about the merged document holds for it.
			committed++
			helper.confirm(t, merged)
		case outcome == "expired", strings.HasPrefix(outcome, "conflicted:"):
			// Admissible: a machine may lose the race outright, and the loser's edit
			// is then refused or durably recorded rather than dropped.
		default:
			t.Fatalf("machine %s reported %q", helper.name, outcome)
		}
	}
	if committed == 0 {
		t.Fatal("no machine committed its mutation")
	}
	if after := crossMachineProjectRevision(t, project); after == baseRevision {
		t.Fatalf("the document still stands at %q after a reported commit", after)
	}
	return crossMachineRaceResult{merged: merged, dataDir: workspace.DataDir, outcomes: outcomes}
}

// crossMachineTask finds one task by title in the merged document, failing by
// name when the merge did not keep it.
func crossMachineTask(t *testing.T, project *models.Project, title string) *models.Task {
	t.Helper()
	for _, task := range project.Tasks {
		if task.Title == title {
			return task
		}
	}
	t.Fatalf("no task titled %q survived the race in %s", title, project.Name)
	return nil
}

func requireCrossMachineResumeNote(t *testing.T, project *models.Project, title, note string) {
	t.Helper()
	if stored := crossMachineTask(t, project, title).ResumeNote; stored != note {
		t.Fatalf("task %q holds resume note %q after the race, want the committed %q", title, stored, note)
	}
}

func requireCrossMachineComment(t *testing.T, project *models.Project, title, text string) {
	t.Helper()
	task := crossMachineTask(t, project, title)
	if !slices.ContainsFunc(task.Comments, func(c *models.Comment) bool { return c != nil && c.Text == text }) {
		t.Fatalf("task %q carries no comment %q after the race", title, text)
	}
}

func requireCrossMachineAcknowledged(t *testing.T, project *models.Project, title string) {
	t.Helper()
	if crossMachineTask(t, project, title).Review {
		t.Fatalf("task %q still awaits review after an acknowledgement committed", title)
	}
}

func requireCrossMachineCreated(t *testing.T, project *models.Project, title string) {
	t.Helper()
	crossMachineTask(t, project, title)
}

// requireCrossMachineDistinctShortIDs is the claim the creation race needs and
// the other two get for free: two machines minting identities against one
// document must not leave the project with two tasks answering to one ShortID,
// which is the reference every surface and every `#N` spelling resolves through.
func requireCrossMachineDistinctShortIDs(t *testing.T, project *models.Project) {
	t.Helper()
	seen := map[int]string{}
	for _, task := range project.Tasks {
		if previous, clash := seen[task.ShortID]; clash {
			t.Fatalf("tasks %q and %q both answer to #%d after the race", previous, task.Title, task.ShortID)
		}
		seen[task.ShortID] = task.Title
	}
}

// openCrossMachineWorkspaceOnce opens the seeded workspace the way a helper role
// does, before any helper runs. A first open may bootstrap system state, and
// letting two forked processes discover that at once would race over something
// other than the resume notes under test.
func openCrossMachineWorkspaceOnce(t *testing.T, dataDir string) {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", dataDir, err)
	}
	if _, err := manager.LoadRuntimeState(); err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", dataDir, err)
	}
}

// crossMachineProjectRevision reads the project's canonical revision straight
// from disk through the same call the application snapshot uses, so the value
// compared against a helper's published revision is the same kind of value.
func crossMachineProjectRevision(t *testing.T, project *models.Project) string {
	t.Helper()
	document, err := storage.LoadProjectDocument(&models.Project{
		ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path,
	})
	if err != nil {
		t.Fatalf("LoadProjectDocument(%q): %v", project.Name, err)
	}
	return document.Revision
}

// crossMachineHelper is one forked machine: the process, the barrier files it
// speaks through, what it published, and the claim its commit makes about the
// document both machines rewrote.
type crossMachineHelper struct {
	name     string
	command  *exec.Cmd
	output   *bytes.Buffer
	ready    string
	result   string
	exit     chan error
	revision string
	// confirm reads the merged project and fails when this machine's reported
	// commit is not in it. Each mutation makes a different claim — a note, a
	// comment, a cleared flag, a task that did not exist — so the claim travels
	// with the machine that made it rather than being re-derived by the race.
	confirm func(t *testing.T, project *models.Project)
}

// crossMachineRole names one forked machine before it exists: which helper role
// to re-enter, what extra environment that role reads, and what its commit will
// claim. One struct covers every race because the roles differ only in those
// three things.
type crossMachineRole struct {
	name    string
	role    string
	env     []string
	confirm func(t *testing.T, project *models.Project)
}

// startCrossMachineHelper forks one machine and returns before it has reached
// the barrier. Both machines are started before either is awaited, since a
// helper that had to be awaited before its partner existed could not overlap
// with it.
func startCrossMachineHelper(t *testing.T, binary, dataDir, barrierDir, proceed string, spec crossMachineRole) *crossMachineHelper {
	t.Helper()
	helper := &crossMachineHelper{
		name:    spec.name,
		output:  &bytes.Buffer{},
		ready:   filepath.Join(barrierDir, "ready-"+spec.name),
		result:  filepath.Join(barrierDir, "result-"+spec.name),
		exit:    make(chan error, 1),
		confirm: spec.confirm,
	}
	helper.command = exec.Command(binary, "-test.run", "^"+spec.role+"$", "-test.count=1")
	helper.command.Env = append(append(os.Environ(),
		helperDataDirEnv+"="+dataDir,
		helperReadyEnv+"="+helper.ready,
		helperGoEnv+"="+proceed,
		helperResultEnv+"="+helper.result,
	), spec.env...)
	helper.command.Stdout = helper.output
	helper.command.Stderr = helper.output
	if err := helper.command.Start(); err != nil {
		t.Fatalf("fork machine %s: %v", spec.name, err)
	}
	t.Cleanup(func() {
		if helper.command.Process != nil {
			_ = helper.command.Process.Kill()
		}
	})
	go func() { helper.exit <- helper.command.Wait() }()
	return helper
}

// awaitBarrier blocks until this machine publishes the revision it is holding.
// A process that exited instead of publishing is a named failure rather than a
// wait that runs to its bound, because the outcome it never reached is the whole
// evidence this test is collecting.
func (h *crossMachineHelper) awaitBarrier(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(helperBarrierTimeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-h.exit:
			t.Fatalf("machine %s exited (%v) before reaching the mutation barrier:\n%s", h.name, err, h.output)
		default:
		}
		published, err := os.ReadFile(h.ready)
		if err == nil {
			h.revision = strings.TrimSpace(string(published))
			if h.revision == "" {
				t.Fatalf("machine %s published an empty base revision", h.name)
			}
			return
		}
		time.Sleep(helperPollInterval)
	}
	t.Fatalf("machine %s did not reach the mutation barrier within %s:\n%s", h.name, helperBarrierTimeout, h.output)
}

func (h *crossMachineHelper) awaitExit(t *testing.T) {
	t.Helper()
	select {
	case err := <-h.exit:
		if err != nil {
			t.Fatalf("machine %s failed: %v\n%s", h.name, err, h.output)
		}
	case <-time.After(helperBarrierTimeout):
		t.Fatalf("machine %s never finished its mutation:\n%s", h.name, h.output)
	}
}

func (h *crossMachineHelper) outcome(t *testing.T) string {
	t.Helper()
	recorded, err := os.ReadFile(h.result)
	if err != nil {
		t.Fatalf("machine %s recorded no outcome: %v\n%s", h.name, err, h.output)
	}
	return strings.TrimSpace(string(recorded))
}
