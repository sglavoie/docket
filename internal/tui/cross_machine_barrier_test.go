package tui

import (
	"slices"
	"strings"
	"testing"

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
