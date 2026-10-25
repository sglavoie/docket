package outliner

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
)

// TestABaselineDropsAMemberTheLatestReadCouldNotOpen is the third reading of an
// absent member, and the one the catalog cannot answer. The project was neither
// excluded by a lens nor withdrawn by the workspace: it is still cataloged, still
// active, still the thing the next read will ask for — and the read that just
// landed asked for it and could not open it. Availability says "keep it", so the
// dismissal is the last place its captured tasks could go back on screen as
// though the workspace had confirmed them.
func TestABaselineDropsAMemberTheLatestReadCouldNotOpen(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	outlinerView.PublishLens(state.NamedLens("lens-1", "Trail"), state, nil,
		outlinerView.CaptureOutlinerViewSnapshot())

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	// Beta's tasks became unreadable while the visit was standing in it. The
	// catalog is untouched, so the read that lands carries alpha alone plus the
	// failure that explains the absence.
	reloadedAlpha, _ := reloadedRelationFixture(t)
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:   []*models.Project{reloadedAlpha},
		Generation: outlinerView.Population.LoadGen,
		FailedLoads: []ProjectLoadFailure{{
			ProjectID: "proj-b",
			Detail:    "failed to load project Beta: unreadable",
		}},
	})

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if loadedProject(outlinerView, "proj-b") != nil {
		t.Fatal("the return itself put the unreadable project back")
	}

	outlinerView.RestoreLensBaseline()
	if restored := loadedProject(outlinerView, "proj-b"); restored != nil {
		t.Fatalf("dismissing the restored lens resurrected %d captured tasks of an unreadable project: %v",
			len(restored.Tasks), loadedProjectIDs(outlinerView))
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the baseline lost a member the read did open: sibling = %q", got)
	}
}

// TestALensReadReportsItsOwnUnreadableProject is the same rule reached through
// the other publish. A lens application reads its own population, so the failures
// that come back with it describe the set it installs, and the baseline it
// destroyed the moment it published is exactly what the dismissal hands back.
func TestALensReadReportsItsOwnUnreadableProject(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	// A project facet the live population was not read under is what makes the
	// application asynchronous rather than immediate.
	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	lens := state.NamedLens("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d02", "Parked included")
	lens.Query = "+parked"
	if _, err := outlinerView.ApplyNamedLens(lens); err != nil {
		t.Fatalf("ApplyNamedLens: %v", err)
	}
	if !outlinerView.Lens.hasPending() {
		t.Fatal("the application published without reading a population")
	}

	reloadedAlpha, _ := reloadedRelationFixture(t)
	if _, published := outlinerView.HandleLensProjectsLoaded(LensProjectsLoadedMsg{
		Projects:   []*models.Project{reloadedAlpha},
		Generation: outlinerView.Lens.Pending.Generation,
		FailedLoads: []ProjectLoadFailure{{
			ProjectID: "proj-b",
			Detail:    "failed to load project Beta: unreadable",
		}},
	}); !published {
		t.Fatal("a partial lens read is publishable and did not publish")
	}

	outlinerView.RestoreLensBaseline()
	if restored := loadedProject(outlinerView, "proj-b"); restored != nil {
		t.Fatalf("dismissing the lens resurrected %d captured tasks the lens read could not open: %v",
			len(restored.Tasks), loadedProjectIDs(outlinerView))
	}
}

// TestAReadThatOpensAProjectAgainRestoresItsBaselineMembership is that rule's
// expiry. A read failure is one read's answer about one project, so the next read
// that does open it is the newer answer and a frame is entitled to its member
// again — with the tasks the frame captured, because the lens standing over it
// narrowed the population rather than reading that project a second time.
func TestAReadThatOpensAProjectAgainRestoresItsBaselineMembership(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })
	baseline := outlinerView.CaptureOutlinerViewSnapshot()

	// The failing read first: alpha alone, beta reported unreadable.
	reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:   []*models.Project{reloadedAlpha},
		Generation: outlinerView.Population.LoadGen,
		FailedLoads: []ProjectLoadFailure{{
			ProjectID: "proj-b",
			Detail:    "failed to load project Beta: unreadable",
		}},
	})
	// Then the read that opens it again.
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedAlpha, reloadedBeta)

	// A lens narrows the population back to alpha, so the baseline is restored
	// over a live set that lacks beta and has to speak for it from its own capture.
	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	outlinerView.PublishLens(state.NamedLens("lens-1", "Alpha only"), state,
		[]*models.Project{reloadedAlpha}, baseline)

	outlinerView.RestoreLensBaseline()
	restored := loadedProject(outlinerView, "proj-b")
	if restored == nil {
		t.Fatalf("a read that opened the project again left it withdrawn: %v",
			loadedProjectIDs(outlinerView))
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract" {
		t.Fatalf("the retained member claimed a reading nothing took: remote = %q", got)
	}
}

// TestAnUnrelatedReadCannotExpireAnotherProjectsReadFailure is the other half of
// that expiry, and the half a whole-record replacement got wrong. Only a read
// that opened the project is a newer answer about it; a read that never asked
// for it is no answer at all. Here the workspace fails Beta, and the read that
// lands next selects Alpha alone — once as a project-tag lens, once as an
// ordinary reload under the same facet — so nothing between the failure and the
// dismissal ever tried Beta again. The baseline holds Beta's captured tasks the
// whole time, and the dismissal is where they would go back on screen.
func TestAnUnrelatedReadCannotExpireAnotherProjectsReadFailure(t *testing.T) {
	for _, route := range []string{"lens", "cross"} {
		t.Run(route, func(t *testing.T) {
			alpha, beta := relationFixture(t)
			// One project tag is what lets the narrower read select Alpha by itself
			// while Beta stays cataloged, active, and simply unasked for.
			alpha.Tags = []string{"keep"}
			outlinerView := scopeTestOutliner(alpha, beta)
			outlinerView.SetPersistActiveLens(func(string) error { return nil })

			state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
			outlinerView.PublishLens(state.NamedLens("lens-1", "Trail"), state, nil,
				outlinerView.CaptureOutlinerViewSnapshot())

			hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

			reloadedAlpha, _ := reloadedRelationFixture(t)
			reloadedAlpha.Tags = []string{"keep"}
			outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
				Projects:   []*models.Project{reloadedAlpha},
				Generation: outlinerView.Population.LoadGen,
				FailedLoads: []ProjectLoadFailure{{
					ProjectID: "proj-b",
					Detail:    "failed to load project Beta: unreadable",
				}},
			})
			if !outlinerView.PopRelationReturn() {
				t.Fatal("the return refused")
			}
			if loadedProject(outlinerView, "proj-b") != nil {
				t.Fatal("the unreadable project was already back before the narrower read")
			}

			narrowed := LensState{
				Mode:   chrome.ModeAll,
				Sort:   query.NewSortState(),
				Facets: checkpointQuery(t, "+#keep"),
			}
			filter := narrowed.ProjectFilterState()
			// The premise the whole test rests on: this population is Alpha's alone,
			// so no route below asks the workspace about Beta a second time.
			selected := SelectCrossScopeProjects(outlinerView.Runtime,
				CrossScopeOpts{ActiveOnly: true, ProjectFilter: filter})
			if len(selected) != 1 || selected[0].ID != "proj-a" {
				t.Fatalf("the narrowed read asks for %d projects, want proj-a alone", len(selected))
			}

			if route == "lens" {
				lens := narrowed.NamedLens("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d02", "Alpha only")
				if _, err := outlinerView.ApplyNamedLens(lens); err != nil {
					t.Fatalf("ApplyNamedLens: %v", err)
				}
				if !outlinerView.Lens.hasPending() {
					t.Fatal("the narrowed lens published without reading a population")
				}
				if _, published := outlinerView.HandleLensProjectsLoaded(LensProjectsLoadedMsg{
					Projects:   []*models.Project{reloadedAlpha},
					Generation: outlinerView.Lens.Pending.Generation,
				}); !published {
					t.Fatal("the Alpha-only lens read did not publish")
				}
			} else {
				outlinerView.SetFilterQuery(narrowed.Facets)
				outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
					Projects:            []*models.Project{reloadedAlpha},
					Generation:          outlinerView.Population.LoadGen,
					LoadedProjectFilter: query.ProjectFilterFingerprint(&filter),
				})
			}

			outlinerView.RestoreLensBaseline()
			if restored := loadedProject(outlinerView, "proj-b"); restored != nil {
				t.Fatalf("an unrelated %s read expired the failure and handed back %d captured tasks: %v",
					route, len(restored.Tasks), loadedProjectIDs(outlinerView))
			}
			for _, row := range outlinerView.Rows {
				if row.IsTaskRow() && row.ProjectID == "proj-b" {
					t.Fatalf("a task of the still-unreadable project rendered after the %s read", route)
				}
			}
			if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
				t.Fatalf("the baseline lost the member the reads did open: sibling = %q", got)
			}
		})
	}
}
