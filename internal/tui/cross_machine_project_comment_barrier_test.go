package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
