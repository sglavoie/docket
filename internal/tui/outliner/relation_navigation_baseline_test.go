package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
)

// TestDismissingARestoredLensKeepsCurrentTaskData is the deepest of these: a
// lens the hop suspended carries a baseline captured before the reload, and the
// dismissal that comes later — an ordinary Esc, long after the return — is where
// that baseline's projects would go back on screen. A baseline may name the
// projects a view held; it may not speak for what is in them.
func TestDismissingARestoredLensKeepsCurrentTaskData(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	outlinerView.PublishLens(state.NamedLens("lens-1", "Waiting"), state, nil,
		outlinerView.CaptureOutlinerViewSnapshot())

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	landReloadedFixture(t, outlinerView, outlinerView.Population.LoadGen)
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if !outlinerView.Lens.hasActive() {
		t.Fatal("the return did not put the suspended lens back")
	}

	outlinerView.RestoreLensBaseline()
	if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract (edited)" {
		t.Fatalf("dismissing the restored lens resurrected pre-reload data: remote = %q", got)
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("dismissing the restored lens resurrected pre-reload data: sibling = %q", got)
	}
}

// TestABaselineKeepsTheProjectsALensExcluded is the other half of that rule. The
// membership is the baseline's and only the objects are re-read, so a lens that
// narrowed the population still hands back the projects it excluded — there is
// no newer reading of those here to prefer.
func TestABaselineKeepsTheProjectsALensExcluded(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })
	baseline := outlinerView.CaptureOutlinerViewSnapshot()

	// The lens reads a narrowed population: beta alone.
	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	_, narrowedBeta := reloadedRelationFixture(t)
	outlinerView.PublishLens(state.NamedLens("lens-1", "Beta only"), state,
		[]*models.Project{narrowedBeta}, baseline)
	if len(outlinerView.Population.Projects) != 1 {
		t.Fatalf("the lens read %d projects, want the narrowed 1", len(outlinerView.Population.Projects))
	}

	outlinerView.RestoreLensBaseline()
	if len(outlinerView.Population.Projects) != 2 {
		t.Fatalf("the baseline restored %d projects, want the captured 2",
			len(outlinerView.Population.Projects))
	}
	// Alpha was excluded and comes back as captured; beta was in the narrowed read
	// and comes back as the newest reading of it.
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema" {
		t.Fatalf("the baseline dropped a project the lens excluded: sibling = %q", got)
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract (edited)" {
		t.Fatalf("the baseline preferred its own reading of a live project: remote = %q", got)
	}
}

// TestABaselineDropsAMemberTheWorkspaceWithdrew is the case the rule above must
// not swallow. Excluded by a lens and withdrawn by the workspace look identical
// from the population alone — both are simply absent from the live set — and
// only one of them entitles a baseline to hand its captured object back. The
// project is archived, parked, or deleted while a relationship visit is away, so
// the dismissal that comes later is the last place its tasks could reappear in a
// view that now has no route to them at all.
func TestABaselineDropsAMemberTheWorkspaceWithdrew(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		withdraw func(beta *models.Project) []*models.Project
	}{
		{
			name: "archived",
			withdraw: func(beta *models.Project) []*models.Project {
				beta.Archived = true
				return []*models.Project{beta}
			},
		},
		{
			name: "parked",
			withdraw: func(beta *models.Project) []*models.Project {
				beta.Tags = []string{models.ParkedProjectTag}
				return []*models.Project{beta}
			},
		},
		{
			name:     "deleted",
			withdraw: func(*models.Project) []*models.Project { return nil },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			alpha, beta := relationFixture(t)
			outlinerView := scopeTestOutliner(alpha, beta)
			outlinerView.SetPersistActiveLens(func(string) error { return nil })

			state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
			outlinerView.PublishLens(state.NamedLens("lens-1", "Trail"), state, nil,
				outlinerView.CaptureOutlinerViewSnapshot())

			hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

			// The workspace withdraws beta while the visit is standing in it, and the
			// read that follows is the base workspace it now names: alpha alone.
			reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
			setTestProjects(outlinerView.Runtime,
				append([]*models.Project{reloadedAlpha}, testCase.withdraw(reloadedBeta)...))
			landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedAlpha)

			if !outlinerView.PopRelationReturn() {
				t.Fatal("the return refused")
			}
			if loadedProject(outlinerView, "proj-b") != nil {
				t.Fatal("the return itself put the withdrawn project back")
			}

			outlinerView.RestoreLensBaseline()
			if loadedProject(outlinerView, "proj-b") != nil {
				t.Fatalf("dismissing the restored lens resurrected the withdrawn project: %v",
					loadedProjectIDs(outlinerView))
			}
			// The baseline's other member is untouched by the rule, and is still the
			// newest reading of itself rather than the captured one.
			if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
				t.Fatalf("the baseline lost a member the workspace still holds: sibling = %q", got)
			}
		})
	}
}

// TestABaselineDropsAMemberTheDestinationStillLoads is the same withdrawal
// reached the other way round. The workspace withdrew the project from the frame
// being restored, but the view the user is standing in was reading under axes of
// its own — a lens that included archived or parked projects — so the withdrawn
// project is still in the live population when the dismissal comes. Membership
// is the restored frame's question, so being loaded by some other view's axes
// buys nothing: the baseline excludes it either way, and a rule that asked only
// about absent members would catch this one only when the destination happened
// to stop loading it too.
func TestABaselineDropsAMemberTheDestinationStillLoads(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		lens     string
		withdraw func(beta *models.Project)
	}{
		{
			name:     "archived",
			lens:     "+archived",
			withdraw: func(beta *models.Project) { beta.Archived = true },
		},
		{
			name:     "parked",
			lens:     "+parked",
			withdraw: func(beta *models.Project) { beta.Tags = []string{models.ParkedProjectTag} },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			alpha, beta := relationFixture(t)
			outlinerView := scopeTestOutliner(alpha, beta)
			outlinerView.SetPersistActiveLens(func(string) error { return nil })

			// The baseline is the ordinary active-projects view; the lens standing
			// over it widens the population axis, which is what lets the destination
			// legitimately hold the project the baseline will not.
			state := LensState{
				Mode:   chrome.ModeAll,
				Facets: checkpointQuery(t, testCase.lens),
				Sort:   query.NewSortState(),
			}
			outlinerView.PublishLens(state.NamedLens("lens-1", "Trail"), state, nil,
				outlinerView.CaptureOutlinerViewSnapshot())

			hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

			// Beta is withdrawn while the visit is standing in it, and the read that
			// lands still carries it because the lens's axes admit it.
			reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
			testCase.withdraw(reloadedBeta)
			setTestProjects(outlinerView.Runtime, []*models.Project{reloadedAlpha, reloadedBeta})
			landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedAlpha, reloadedBeta)
			if loadedProject(outlinerView, "proj-b") == nil {
				t.Fatal("the lens population dropped the project this test is about")
			}

			if !outlinerView.PopRelationReturn() {
				t.Fatal("the return refused")
			}

			outlinerView.RestoreLensBaseline()
			if loadedProject(outlinerView, "proj-b") != nil {
				t.Fatalf("dismissing the restored lens kept a project the baseline withdraws: %v",
					loadedProjectIDs(outlinerView))
			}
			for _, row := range outlinerView.Rows {
				if row.IsTaskRow() && row.ProjectID == "proj-b" {
					t.Fatal("the withdrawn project's tasks are still on screen")
				}
			}
			if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
				t.Fatalf("the baseline lost a member the workspace still holds: sibling = %q", got)
			}
		})
	}
}

// TestAnIsolatingAxisStillHandsBackTheMembershipItCaptured is the limit of that
// rule. An axis narrows this read in one direction only: a project that stopped
// being archived is not a project the workspace withdrew, and a frame whose own
// axes and population disagree — which is every frame captured while an axis
// change is still loading — keeps the membership it holds rather than being
// audited against a population nothing read.
func TestAnIsolatingAxisStillHandsBackTheMembershipItCaptured(t *testing.T) {
	alpha, beta := relationFixture(t)
	alpha.Archived = true
	beta.Archived = true
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })
	outlinerView.SetFilterQuery(checkpointQuery(t, "+archived:only"))
	baseline := outlinerView.CaptureOutlinerViewSnapshot()

	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	outlinerView.PublishLens(state.NamedLens("lens-1", "Alpha only"), state,
		[]*models.Project{alpha}, baseline)

	// Beta is restored to the live workspace while the lens stands.
	reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
	reloadedAlpha.Archived = true
	setTestProjects(outlinerView.Runtime, []*models.Project{reloadedAlpha, reloadedBeta})

	outlinerView.RestoreLensBaseline()
	if got := loadedProjectIDs(outlinerView); !slices.Equal(got, []string{"proj-a", "proj-b"}) {
		t.Fatalf("restored population = %v, want the captured membership whole", got)
	}
}

// TestARetainedBaselineMemberWearsTheCatalogsCurrentMetadata is the seam between
// those two rules. A member the live population does not hold comes back with
// the tasks the frame captured, because nothing here read it again — but the
// project itself is described by the catalog, which is current, so the name a
// group header draws is the one the workspace holds now and not the one that
// stood when the lens was applied.
func TestARetainedBaselineMemberWearsTheCatalogsCurrentMetadata(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })
	baseline := outlinerView.CaptureOutlinerViewSnapshot()

	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	_, narrowedBeta := reloadedRelationFixture(t)
	outlinerView.PublishLens(state.NamedLens("lens-1", "Beta only"), state,
		[]*models.Project{narrowedBeta}, baseline)

	renamedAlpha, _ := reloadedRelationFixture(t)
	renamedAlpha.Name = "Alpha (renamed)"
	setTestProjects(outlinerView.Runtime, []*models.Project{renamedAlpha, narrowedBeta})

	outlinerView.RestoreLensBaseline()
	restored := loadedProject(outlinerView, "proj-a")
	if restored == nil {
		t.Fatalf("the baseline dropped a project the lens excluded: %v", loadedProjectIDs(outlinerView))
	}
	if restored.Name != "Alpha (renamed)" {
		t.Fatalf("the retained member kept a superseded description: name = %q", restored.Name)
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema" {
		t.Fatalf("the retained member claimed a reading nothing took: sibling = %q", got)
	}
}
