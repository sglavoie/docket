package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// The in-suite cross-machine races for deadline stepping and checklist
// promotion. They fork the same helper roles scripts/cross_machine_gate.sh
// forks, so what is proven here is what the gate depends on.
//
// Two runners, because these mutations have two shapes of collision and only one
// of them is the shape every earlier race had. runCrossMachineRace releases all
// machines together and admits either winner, which is right when the hazard is
// symmetric. A strict read-set precondition is not symmetric: the loser is
// refused whole, before any merge, and the claim is about what it did not leave
// behind. Proving that needs a known loser, which is what
// runCrossMachineInterruptedMutation arranges — one machine held at its commit
// seam while a second machine commits to completion and exits.

const (
	// The three deadlines a step race can produce, spelled as the layout the
	// helper roles report through. Nothing here hardcodes a date: a race is run
	// against the machine's own clock, so the assertions compare the reported
	// values with each other and with the bytes.
	dueStepRaceAnchorTitle  = helperRaceTitleA
	dueStepRaceSupportTitle = "Follow up with the support queue"

	// The text an intervening writer commits. It only has to be a write into the
	// document the held batch read, so the cheapest honest one is a comment.
	interveningCommentText = "Machine B logged the vendor call"
)

// dueStepRole names one forked machine stepping a selection of deadlines. The
// titles are joined rather than passed separately because the order is the
// input: the first names the project the barrier holds the batch at.
func dueStepRole(name, retry string, titles ...string) crossMachineRole {
	env := []string{
		helperTitleEnv + "=" + strings.Join(titles, helperTitleSeparator),
		helperDueStepEnv + "=" + helperDueStepForward,
	}
	if retry != "" {
		env = append(env, helperDueRetryEnv+"="+retry)
	}
	return crossMachineRole{
		name: name,
		role: "TestCrossMachineHelperStepsDueDate",
		env:  env,
		confirm: func(t *testing.T, project *models.Project) {
			// A step's own claim is only that the task it stepped still carries a
			// deadline; which deadline is a claim about the pair, made by the judge.
			if crossMachineTask(t, project, titles[0]).DueDate == nil {
				t.Fatalf("task %q carries no deadline after a step committed", titles[0])
			}
		},
	}
}

// TestCrossMachineDueStepRaceRefusesTheStaleStepAndAdvancesOnRetry is the
// two-steps-on-one-task contract.
//
// Both machines read one task at one revision and both calculate a destination
// from it. The read-set guard is strict, so the second commit cannot merge: it is
// refused before a merge is attempted, and the retry — reading what the winner
// left — is what advances the deadline a second place. The whole point is that
// two reported successes are two steps: a stale operation that silently merged
// would leave one machine reporting a move the file never made, and the two
// reports together would describe a single day's travel.
//
// The dates are not hardcoded. The retrying machine reports the deadline it
// stepped from and the one it wrote, both read against the machine's own clock,
// and the assertions are that the first is what the winner committed, the second
// is exactly one calendar day later, and the file holds the second.
func TestCrossMachineDueStepRaceRefusesTheStaleStepAndAdvancesOnRetry(t *testing.T) {
	race := runCrossMachineRace(t, []crossMachineRole{
		dueStepRole("A", "1", dueStepRaceAnchorTitle),
		dueStepRole("B", "1", dueStepRaceAnchorTitle),
	})
	requireCrossMachineDistinctShortIDs(t, race.merged)

	committed, retried := 0, ""
	for _, outcome := range race.outcomes {
		switch {
		case outcome == "committed":
			committed++
		case strings.HasPrefix(outcome, "retried:"):
			retried = strings.TrimSpace(strings.TrimPrefix(outcome, "retried:"))
		}
	}
	if committed != 1 || retried == "" {
		t.Fatalf("deadline step outcomes = %v, want one committed and one refused then retried", race.outcomes)
	}

	from, to := crossMachineDueStepMove(t, retried)
	if from == "" {
		t.Fatalf("the retry reported stepping from no deadline; the winner committed nothing to advance from")
	}
	if want := crossMachineNextDay(t, from); to != want {
		t.Fatalf("the retry landed on %q, want one day past the winner's %q (%q)", to, from, want)
	}
	if stored := helperDeadlineValue(crossMachineTask(t, race.merged, dueStepRaceAnchorTitle).DueDate); stored != to {
		t.Fatalf("the document holds deadline %q, want the retry's %q", stored, to)
	}
}

// crossMachineDueStepMove reads the one "<before>><after>" pair a retrying step
// reports. Deadline stepping is a batch operation, so the report is a list; this
// race steps one task, and a report naming anything else is a defect in the role
// rather than an outcome to interpret.
func crossMachineDueStepMove(t *testing.T, reported string) (string, string) {
	t.Helper()
	fields := strings.Split(reported, ",")
	if len(fields) != 1 {
		t.Fatalf("the retry reported %d moves (%q), want the one task it stepped", len(fields), reported)
	}
	from, to, ok := strings.Cut(fields[0], ">")
	if !ok {
		t.Fatalf("the retry reported %q, which names no move", fields[0])
	}
	return from, to
}

// crossMachineNextDay is the calendar day after a reported deadline, computed
// the way deadline stepping computes one: through the calendar rather than by
// adding a duration, so a race run across a zone transition still names the day
// the step would have landed on.
func crossMachineNextDay(t *testing.T, deadline string) string {
	t.Helper()
	parsed, err := time.Parse(helperDeadlineLayout, deadline)
	if err != nil {
		t.Fatalf("parse reported deadline %q: %v", deadline, err)
	}
	return parsed.AddDate(0, 0, 1).Format(helperDeadlineLayout)
}

// TestCrossMachineDueStepBatchIsRefusedWholeByAnInterveningWrite is the
// multi-project half: one batch spanning two projects, held, while a third-party
// write lands in one of them.
//
// A batch is one transaction over a read set, and the guard covers every project
// it read — so a write into either one refuses the whole thing. The claim is
// three absences and one presence: no deadline moved in either project, the
// planning artifact a step onto today would have rewritten is byte-identical,
// the ledger gained exactly the interrupting write's own entry and nothing else,
// and no conflict bundle was staged. That last one is the difference a
// precondition makes: it is validated ahead of the merge, so there is nothing to
// record and nothing to retry from.
func TestCrossMachineDueStepBatchIsRefusedWholeByAnInterveningWrite(t *testing.T) {
	held := dueStepRole("held", "", dueStepRaceAnchorTitle, dueStepRaceSupportTitle)
	// The interrupting write names a different task and a different action, so
	// the held batch is the only thing that could have moved its own targets: the
	// refused process is asked to prove from the inside that none of them did.
	held.env = append(held.env, helperDueUntouchedEnv+"=1")
	race := runCrossMachineInterruptedMutation(t,
		held,
		crossMachineRole{
			name: "writer",
			role: "TestCrossMachineHelperAppliesTaskMutation",
			env: []string{
				helperMutationEnv + "=" + helperMutationComment,
				helperTitleEnv + "=" + helperRaceTitleB,
				helperTextEnv + "=" + interveningCommentText,
			},
		},
	)
	if race.held != "expired" {
		t.Fatalf("the held batch reported %q, want a staleness refusal", race.held)
	}
	if due := crossMachineTask(t, race.merged, dueStepRaceAnchorTitle).DueDate; due != nil {
		t.Fatalf("the refused batch left a deadline of %s on %q", due, dueStepRaceAnchorTitle)
	}
	requireCrossMachineSupportDeadlineUnset(t, race.dataDir)
	requireCrossMachinePlanningUnchanged(t, race)
	requireCrossMachineStagedNothing(t, race, "Add Comment")
}

// requireCrossMachineSupportDeadlineUnset reads the second project the refused
// batch named. The merged document the runner returns is the race project alone,
// and a batch refused whole has to have left every project it read untouched —
// including the one no other machine was writing to, which is the one a partial
// commit would have been free to move.
func requireCrossMachineSupportDeadlineUnset(t *testing.T, dataDir string) {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", dataDir, err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", dataDir, err)
	}
	for _, entry := range runtime.AvailableProjects() {
		loaded := &models.Project{ID: entry.ID, Name: entry.Name, Slug: entry.Slug, Path: entry.Path}
		if err = storage.LoadProject(loaded); err != nil {
			t.Fatalf("LoadProject(%q): %v", entry.Name, err)
		}
		for _, task := range loaded.Tasks {
			if task.Title == dueStepRaceSupportTitle && task.DueDate != nil {
				t.Fatalf("the refused batch left a deadline of %s on %q", task.DueDate, task.Title)
			}
		}
	}
}

// promotionRole names one forked machine promoting the seeded checklist's head
// item. The position and the text it carries are fixed rather than taken as
// arguments because every race here is a race over one item: two machines
// promoting different positions would not be contending for anything, and the
// whole question is what the loser does to the item the winner already
// converted.
func promotionRole(name string) crossMachineRole {
	return crossMachineRole{
		name: name,
		role: "TestCrossMachineHelperPromotesChecklistItem",
		env: []string{
			helperTitleEnv + "=" + helperRaceTitleA,
			helperChecklistIndexEnv + "=0",
		},
		confirm: func(t *testing.T, project *models.Project) {
			crossMachineTask(t, project, helperSeedItemA)
		},
	}
}

// checklistRole names one forked machine rewriting or reordering that same
// checklist. It is the promotion's counterparty: it commits under the ordinary
// merge-capable revision, so pairing the two puts one guard of each kind into
// one race.
func checklistRole(name, kind string, from, to int, text string) crossMachineRole {
	return crossMachineRole{
		name: name,
		role: "TestCrossMachineHelperEditsChecklist",
		env: []string{
			helperTitleEnv + "=" + helperRaceTitleA,
			helperChecklistEnv + "=" + kind,
			helperChecklistIndexEnv + "=" + fmt.Sprint(from),
			helperChecklistToEnv + "=" + fmt.Sprint(to),
			helperTextEnv + "=" + text,
		},
		confirm: func(t *testing.T, project *models.Project) {
			crossMachineTask(t, project, helperRaceTitleA)
		},
	}
}

// TestCrossMachinePromotionBarrierFreezesChecklistConversions races a promotion
// against every other machine that can touch the list it is converting from.
//
// A checklist item has no stored identity, so the promotion freezes the whole
// ordered list and is refused unless the owner still carries exactly it. That
// check is made against the promoting machine's own snapshot, which is why the
// races here are decided at the commit boundary instead: both machines freeze
// the same list, and the strict read-set guard is what refuses the second one.
//
// Four pairings, because four different things can go wrong. A rival promotion
// could convert one item twice. An edit could move the item the promotion is
// about to remove onto a different position. A reorder could do the same without
// changing any text. A concurrent creation mints the short id the promoted child
// was going to take. The claim shared by all four is the one the item and its
// child are two shapes of: exactly one of them exists when the dust settles, the
// items nobody named are still there, and no two tasks answer to one number.
func TestCrossMachinePromotionBarrierFreezesChecklistConversions(t *testing.T) {
	const (
		rewrittenItem = "Name the rollback owner before Friday"
		rivalCreation = "Draft the rollout announcement"
	)
	for _, tc := range []struct {
		name     string
		roles    []crossMachineRole
		survivor []string
	}{
		{
			// Two machines converting one item. One must win outright; the other is
			// refused by the read-set guard, so the item becomes a child exactly once.
			name: "rival-promotions",
			roles: []crossMachineRole{
				promotionRole("A"),
				promotionRole("B"),
			},
			survivor: []string{helperSeedItemB, helperSeedItemC},
		},
		{
			// An edit to a different item. Whichever side loses, the item the edit
			// never named must not have been removed, and the promoted item must not
			// come back beside its own child.
			name: "promotion-versus-edit",
			roles: []crossMachineRole{
				promotionRole("A"),
				checklistRole("B", helperChecklistEdit, 1, 1, rewrittenItem),
			},
			survivor: []string{helperSeedItemC},
		},
		{
			// A reorder changes no text at all, so an index-keyed promotion would
			// convert whatever slid into position zero and nothing in the document
			// would say so.
			name: "promotion-versus-reorder",
			roles: []crossMachineRole{
				promotionRole("A"),
				checklistRole("B", helperChecklistReorder, 2, 0, ""),
			},
			survivor: []string{helperSeedItemB, helperSeedItemC},
		},
		{
			// A promotion and a creation mint against one document, so both take the
			// same next-free short id. Whichever is refused, no project may end up
			// holding two tasks that answer to one number.
			name: "promotion-versus-creation",
			roles: []crossMachineRole{
				promotionRole("A"),
				{
					name: "B",
					role: "TestCrossMachineHelperAppliesTaskMutation",
					env: []string{
						helperMutationEnv + "=" + helperMutationCreate,
						helperTitleEnv + "=" + helperRaceTitleA,
						helperTextEnv + "=" + rivalCreation,
					},
					confirm: func(t *testing.T, project *models.Project) {
						requireCrossMachineCreated(t, project, rivalCreation)
					},
				},
			},
			survivor: []string{helperSeedItemB, helperSeedItemC},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			race := runCrossMachineRace(t, tc.roles)
			requireCrossMachineDistinctShortIDs(t, race.merged)
			requireCrossMachinePromotionIsWhole(t, race.merged, helperSeedItemA)
			requireCrossMachineChecklistRetains(t, race.merged, tc.survivor)
		})
	}
}

// requireCrossMachinePromotionIsWhole is the all-or-nothing claim over the
// merged document: the item's text reads either as one checklist entry or as one
// task, never as both and never twice. It counts rather than tests presence,
// because presence cannot tell one conversion from two.
func requireCrossMachinePromotionIsWhole(t *testing.T, project *models.Project, item string) {
	t.Helper()
	owner := crossMachineTask(t, project, helperRaceTitleA)
	items, children := 0, 0
	for _, entry := range owner.Checklist {
		if entry.Text == item {
			items++
		}
	}
	for _, task := range project.Tasks {
		if task.Title == item {
			children++
		}
	}
	if items+children != 1 {
		t.Fatalf("%q reads as %d checklist item(s) and %d task(s) after the race; a promotion is one or the other",
			item, items, children)
	}
	if children == 1 {
		child := crossMachineTask(t, project, item)
		if child.ParentID == nil || *child.ParentID != owner.ID {
			t.Fatalf("the promoted child of %q is not parented to it", helperRaceTitleA)
		}
	}
}

// requireCrossMachineChecklistRetains is the no-wrong-item-removal claim. The
// promotion names one item; every other one the race did not rewrite must still
// be on the list, because a conversion that took a neighbor would leave a
// perfectly valid document that quietly lost a line.
func requireCrossMachineChecklistRetains(t *testing.T, project *models.Project, items []string) {
	t.Helper()
	owner := crossMachineTask(t, project, helperRaceTitleA)
	stored := make([]string, 0, len(owner.Checklist))
	for _, entry := range owner.Checklist {
		stored = append(stored, entry.Text)
	}
	for _, item := range items {
		count := 0
		for _, entry := range stored {
			if entry == item {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("checklist item %q appears %d time(s) after the race: %v", item, count, stored)
		}
	}
}

// TestCrossMachineRefusalShapesAreDistinct is the one test that puts the two
// refusals side by side, in the only arrangement that can tell them apart.
//
// They are not degrees of the same thing. A strict read-set precondition is
// validated in one phase ahead of every merge, so a mutation guarded by one is
// refused before a conflict bundle, a ledger image, or a staged byte exists;
// there is nothing to show and nothing to retry from, and that is deliberate —
// the deadline the loser calculated was calculated against a document nobody has
// anymore. An ordinary merge-capable commit is the opposite: it merges what it
// can, and a whole it cannot validate is durably recorded so the losing machine
// has something to list and a defined next step.
//
// The same two mutations are raced in both directions, so the difference is in
// which guard was held and nothing else.
func TestCrossMachineRefusalShapesAreDistinct(t *testing.T) {
	const createdTitle = "Draft the rollout announcement"
	creation := crossMachineRole{
		name: "creation",
		role: "TestCrossMachineHelperAppliesTaskMutation",
		env: []string{
			helperMutationEnv + "=" + helperMutationCreate,
			helperTitleEnv + "=" + helperRaceTitleA,
			helperTextEnv + "=" + createdTitle,
		},
	}

	t.Run("precondition-stages-nothing", func(t *testing.T) {
		held := promotionRole("held")
		race := runCrossMachineInterruptedMutation(t, held, creation)
		if race.held != "expired" {
			t.Fatalf("the held promotion reported %q, want a staleness refusal", race.held)
		}
		requireCrossMachinePromotionIsWhole(t, race.merged, helperSeedItemA)
		requireCrossMachineChecklistRetains(t, race.merged,
			[]string{helperSeedItemA, helperSeedItemB, helperSeedItemC})
		requireCrossMachineStagedNothing(t, race, "Add Task")
	})

	t.Run("merge-conflict-is-recorded-and-retried", func(t *testing.T) {
		held := creation
		held.name = "held"
		race := runCrossMachineInterruptedMutation(t, held, promotionRole("writer"))
		if !strings.HasPrefix(race.held, "retried:") {
			t.Fatalf("the held creation reported %q, want a recorded conflict it then retried", race.held)
		}
		requireCrossMachineDistinctShortIDs(t, race.merged)
		requireCrossMachinePromotionIsWhole(t, race.merged, helperSeedItemA)
		requireCrossMachineCreated(t, race.merged, createdTitle)

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
	})
}
