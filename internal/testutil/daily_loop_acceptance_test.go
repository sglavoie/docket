package testutil

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestDailyLoopWorkflowStateAcceptanceMatrix(t *testing.T) {
	matrix := DailyLoopWorkflowStateAcceptance()
	wantStates := models.AllWorkflowStates()
	if len(matrix) != len(wantStates) {
		t.Fatalf("acceptance matrix has %d states, want %d", len(matrix), len(wantStates))
	}

	for i, row := range matrix {
		if row.State != wantStates[i] {
			t.Errorf("state %d = %q, want %q", i, row.State, wantStates[i])
		}
		if row.Terminal != row.State.IsTerminal() {
			t.Errorf("%q terminal=%v, want %v", row.State, row.Terminal, row.State.IsTerminal())
		}
	}
}

func TestDailyLoopLegacyLifecycleFixturesCoverEveryInputFamily(t *testing.T) {
	fixtures := DailyLoopLegacyLifecycleFixtures()
	statuses := make(map[string]bool)
	sections := make(map[string]bool)
	ids := make(map[string]bool)
	var hasActive, hasEmpty, hasStage bool

	for _, fixture := range fixtures {
		if ids[fixture.ID] {
			t.Errorf("duplicate legacy fixture ID %q", fixture.ID)
		}
		ids[fixture.ID] = true
		statuses[fixture.Status] = true
		sections[fixture.Section] = true
		hasActive = hasActive || fixture.Active
		hasEmpty = hasEmpty || (fixture.Section == "" && fixture.Stage == "")
		hasStage = hasStage || fixture.Stage != ""

	}

	for _, status := range []string{"todo", "in_progress", "blocked", "standby", "review", "maybe", "done", "canceled"} {
		if !statuses[status] {
			t.Errorf("legacy status %q has no reusable fixture", status)
		}
	}
	for _, section := range []string{
		"Inbox",
		"Next",
		"Waiting",
		"Standby",
		"Review",
		"Someday",
		"Completed",
	} {
		if !sections[section] {
			t.Errorf("legacy section %q has no reusable fixture", section)
		}
	}
	if !hasActive || !hasEmpty || !hasStage {
		t.Fatalf("fixture families active=%v empty=%v stage=%v, all must be covered", hasActive, hasEmpty, hasStage)
	}
}

func TestDailyLoopPlanningAcceptanceMatrixIsInternallyConsistent(t *testing.T) {
	for _, fixture := range DailyLoopPlanningTransitionFixtures() {
		t.Run(fixture.Name, func(t *testing.T) {
			if !fixture.InitialState.IsValid() || !fixture.WantState.IsValid() {
				t.Fatalf("invalid state transition %q -> %q", fixture.InitialState, fixture.WantState)
			}
			if fixture.InitialNext != (fixture.InitialState == models.StateNext) {
				t.Errorf("initial Next membership disagrees with state %q", fixture.InitialState)
			}
			if fixture.WantNext != (fixture.WantState == models.StateNext) {
				t.Errorf("result Next membership disagrees with state %q", fixture.WantState)
			}
			if fixture.WantToday && (fixture.WantNext || fixture.WantState == models.StateInbox || fixture.WantState.IsTerminal()) {
				t.Errorf("result violates Today eligibility or Today/Next exclusivity")
			}
		})
	}
}

func TestDailyLoopTargetContractInventory(t *testing.T) {
	contract := DailyLoopTargetContract()
	wantModes := []string{"All", "Inbox", "Next", "Backlog", "Due"}
	if !slices.Equal(contract.Modes, wantModes) {
		t.Fatalf("modes = %v, want %v", contract.Modes, wantModes)
	}
	if !slices.Equal(contract.TabCycle, wantModes) {
		t.Errorf("Tab cycle = %v, want %v", contract.TabCycle, wantModes)
	}
	if contract.InitialMode != "All" || contract.InitialScope != "All Projects" {
		t.Errorf("initial route = %s / %s, want All / All Projects", contract.InitialMode, contract.InitialScope)
	}
	if contract.InteractionMax != 2 {
		t.Errorf("overlay depth = %d, want operation sheet plus one value picker", contract.InteractionMax)
	}

	seenKeys := make(map[string]bool)
	for _, binding := range contract.Keys {
		if binding.Key == "" || binding.Action == "" || binding.Area == "" {
			t.Errorf("incomplete key binding: %+v", binding)
		}
		if seenKeys[binding.Key] {
			t.Errorf("duplicate stable key %q", binding.Key)
		}
		seenKeys[binding.Key] = true
	}

	public := contract.Public
	if public.CreationFlag != "--state" || public.ListArchiveFilter != "state" || public.SortField != "state" {
		t.Errorf("public CLI vocabulary is not state-only: %+v", public)
	}
	if !slices.Equal(public.TaskJSONLifecycleFields, []string{"state"}) {
		t.Errorf("task JSON lifecycle fields = %v, want only state", public.TaskJSONLifecycleFields)
	}
	if public.TemplateField != "state" {
		t.Errorf("template persistence field = %q, want state", public.TemplateField)
	}
	if public.ActionPlanVersion != 2 || !public.CreateTaskStateOptional {
		t.Errorf("action-plan contract = v%d optional=%v", public.ActionPlanVersion, public.CreateTaskStateOptional)
	}
	for _, rule := range []string{
		"normal space opens contextual action sheet",
		"multi-select space toggles selection",
		"multi-select enter opens bulk actions",
		"delete exists only in the action sheet and requires confirmation",
	} {
		if !slices.Contains(contract.Interaction, rule) {
			t.Errorf("interaction contract is missing %q", rule)
		}
	}
	if len(contract.ScopeRules) != 3 || len(contract.Visibility) != 4 {
		t.Errorf("scope/visibility contract is incomplete: %v / %v", contract.ScopeRules, contract.Visibility)
	}
	for _, rule := range []string{
		"N always opens All Notes across Global and every active project",
		"Notes v scopes s searches title and body and m moves one note; archived projects and Ctrl+N are excluded",
	} {
		if !slices.Contains(contract.ScopeRules, rule) {
			t.Errorf("scope contract is missing %q", rule)
		}
	}
	if slices.ContainsFunc(contract.Keys, func(binding KeyAcceptance) bool {
		return binding.Key == "R" || binding.Action == "weekly review"
	}) {
		t.Error("daily-loop contract retained the removed weekly review tool")
	}
}

func TestDailyLoopCreationAcceptanceDefaultsToInbox(t *testing.T) {
	fixtures := DailyLoopCreationAcceptance()
	if len(fixtures) < 2 {
		t.Fatal("creation matrix must cover base and project scope")
	}
	for _, fixture := range fixtures {
		if fixture.Surface == "" || fixture.ProjectTarget == "" {
			t.Errorf("incomplete creation fixture: %+v", fixture)
		}
		if fixture.DefaultState != models.StateInbox {
			t.Errorf("%s defaults to %q, want inbox", fixture.Surface, fixture.DefaultState)
		}
	}
}
