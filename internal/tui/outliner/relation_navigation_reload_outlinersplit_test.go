package outliner

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// A relationship hop is presentation, but the workspace underneath it is not
// still. These are the crossings: a read already in flight when the key was
// pressed, a reload that landed while the user was away, a project that stopped
// being available, and a lens that was suspended before any of it happened. What
// they share is a restoration point — a return frame, a lens baseline, or a
// captured reload cursor — that was written against a reading of the workspace
// the view has since replaced. A restoration point may hand back a view; it may
// never hand back data.

// reloadedRelationFixture is the same two projects read again, as a disk reload
// produces them: fresh pointers, and two titles changed so a stale reading is
// visible in an assertion rather than merely possible.
func reloadedRelationFixture(t *testing.T) (*models.Project, *models.Project) {
	t.Helper()
	alpha, beta := relationFixture(t)
	alpha.GetTaskByID("sibling").Title = "Agree the schema (edited)"
	beta.GetTaskByID("remote").Title = "Publish the contract (edited)"
	return alpha, beta
}

// loadedTaskTitle reads one title out of the live population, which is the
// reading the view renders from.
func loadedTaskTitle(t *testing.T, outlinerView *OutlinerModel, projectID, taskID string) string {
	t.Helper()
	for _, project := range outlinerView.Population.Projects {
		if project == nil || project.ID != projectID {
			continue
		}
		task := project.GetTaskByID(taskID)
		if task == nil {
			t.Fatalf("project %q holds no task %q", projectID, taskID)
		}
		return task.Title
	}
	t.Fatalf("population holds no project %q", projectID)
	return ""
}

// loadedProject returns one member of the live population, or nil when the
// population no longer holds it. Membership is the question a baseline restore
// answers, so an assertion about it must be able to see an absence.
func loadedProject(outlinerView *OutlinerModel, projectID string) *models.Project {
	for _, project := range outlinerView.Population.Projects {
		if project != nil && project.ID == projectID {
			return project
		}
	}
	return nil
}

// loadedProjectIDs names the live population in order.
func loadedProjectIDs(outlinerView *OutlinerModel) []string {
	ids := make([]string, 0, len(outlinerView.Population.Projects))
	for _, project := range outlinerView.Population.Projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return ids
}

// landCrossProjects delivers a completed read at one generation.
func landCrossProjects(
	outlinerView *OutlinerModel, generation int, projects ...*models.Project,
) {
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:   projects,
		Generation: generation,
	})
}

// landReloadedFixture is the ordinary case: the whole workspace read again.
func landReloadedFixture(t *testing.T, outlinerView *OutlinerModel, generation int) {
	t.Helper()
	alpha, beta := reloadedRelationFixture(t)
	landCrossProjects(outlinerView, generation, alpha, beta)
}

// TestRelationReturnResolvesAgainstCurrentData is the whole of what a return
// frame is allowed to be: a description of a view. The tasks changed while the
// user was away, and the return shows the change rather than the frame.
func TestRelationReturnResolvesAgainstCurrentData(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	landReloadedFixture(t, outlinerView, outlinerView.Population.LoadGen)
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}

	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the return put a stale reading back: sibling = %q", got)
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract (edited)" {
		t.Fatalf("the return put a stale reading back: remote = %q", got)
	}
}

// TestRelationReturnSeatsANearestSurvivorWhenTheSourceIsDeleted is the fallback
// the checkpoint already owns, held to over a hop: the task the picker was
// opened over was deleted while the user was visiting, so the return lands on a
// row rather than on nothing.
func TestRelationReturnSeatsANearestSurvivorWhenTheSourceIsDeleted(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	survivingAlpha, reloadedBeta := reloadedRelationFixture(t)
	survivingAlpha.Tasks = []*models.Task{survivingAlpha.GetTaskByID("sibling")}
	survivingAlpha.BuildTaskTree()
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, survivingAlpha, reloadedBeta)

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	identity := outlinerView.CurrentTaskIdentity()
	if identity.TaskID == "source" {
		t.Fatal("the return seated a task the workspace no longer holds")
	}
	if identity.TaskID == "" {
		t.Fatalf("the return seated no row at all (cursor=%d rows=%d)",
			outlinerView.Cursor, len(outlinerView.Rows))
	}
}

// TestRelationReturnReconcilesAScopeWhoseProjectWentAway covers the origin
// leaving the population entirely — archived, parked, or deleted while the user
// was away. Reinstating the frame as written would stand the view inside a
// project the workspace no longer holds, so the return runs the same repair the
// reload path owns and says so.
func TestRelationReturnReconcilesAScopeWhoseProjectWentAway(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	_, reloadedBeta := reloadedRelationFixture(t)
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedBeta)

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.IsProjectScoped() {
		t.Fatalf("the return reinstated a scope on the missing %q", outlinerView.Scope.ProjectID)
	}
	if len(outlinerView.Rows) == 0 {
		t.Fatal("the return landed on an empty list")
	}
	if !strings.Contains(outlinerView.StatusMessage, "returned to the base scope") {
		t.Fatalf("the repair went unreported: %q", outlinerView.StatusMessage)
	}
	if !outlinerView.StatusIsTransient() {
		t.Fatal("the repair notice is sticky, and a return raises no Cmd to clear it")
	}
}

// TestAReadInFlightCannotOverwriteARelationDestination is the stale-cursor
// crossing. The read was started before the key was pressed and is carrying the
// origin's position; it is still wanted, because its projects are the newest
// reading there is, but landing it must not undo the hop.
func TestAReadInFlightCannotOverwriteARelationDestination(t *testing.T) {
	for _, testCase := range []struct {
		name string
		row  string
		want taskref.TaskIdentity
	}{
		{
			name: "across projects",
			row:  "out:proj-b:remote",
			want: taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"},
		},
		{
			name: "within one project",
			row:  "out:proj-a:sibling",
			want: taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "sibling"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			alpha, beta := relationFixture(t)
			outlinerView := scopeTestOutliner(alpha, beta)
			outlinerView.MoveCursorToProjectTask("source", "proj-a")
			// The reload captures the origin's cursor on its way out.
			_ = outlinerView.ReloadCrossProjects()
			generation := outlinerView.Population.LoadGen

			hopToRelation(t, outlinerView, "proj-a", "source", testCase.row)
			landReloadedFixture(t, outlinerView, generation)

			requireCursorTask(t, outlinerView, testCase.want)
			// The read was re-aimed, never restarted: its projects are on screen.
			if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract (edited)" {
				t.Fatalf("the re-aimed read published nothing: remote = %q", got)
			}
		})
	}
}

// TestAReadInFlightCannotOverwriteARestoredReturn is the same crossing on the
// way back: the read was started at the destination and would drag the cursor
// out of the origin the return frame just restored.
func TestAReadInFlightCannotOverwriteARestoredReturn(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")

	_ = outlinerView.ReloadCrossProjects()
	generation := outlinerView.Population.LoadGen
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	landReloadedFixture(t, outlinerView, generation)

	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}

// TestAReadInFlightCannotOverwriteAReset is the third act that replaces the
// whole projection rather than moving within it. `0` promised a launch view, and
// a read landing behind it must not put the discarded position back.
func TestAReadInFlightCannotOverwriteAReset(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")

	_ = outlinerView.ReloadCrossProjects()
	generation := outlinerView.Population.LoadGen
	outlinerView.ResetToLaunchView()
	reset := outlinerView.CurrentTaskIdentity()

	landReloadedFixture(t, outlinerView, generation)
	if got := outlinerView.CurrentTaskIdentity(); got != reset {
		t.Fatalf("a read landing after the reset moved the cursor to %+v, want %+v", got, reset)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a read landing after the reset resurrected the trail")
	}
}

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

// TestARestoredBaselineKeepsTheParkedOriginItNamed is that rule's exception, and
// the reason the eligibility asked of a live member is the frame's own rather
// than the base workspace's. A view standing in an explicitly focused parked
// project reads it whatever the parked axis says, so the frame naming it is
// entitled to hand it back — the axis withdrew nothing here, the user did the
// opposite and named it.
func TestARestoredBaselineKeepsTheParkedOriginItNamed(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	outlinerView.PublishLens(state.NamedLens("lens-1", "Trail"), state, nil,
		outlinerView.CaptureOutlinerViewSnapshot())

	outlinerView.RestoreLensBaseline()
	if loadedProject(outlinerView, "proj-a") == nil {
		t.Fatalf("the baseline dropped the parked project its own scope names: %v",
			loadedProjectIDs(outlinerView))
	}
}

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

// TestALensApplicationInFlightCannotPublishOverAHop closes the delayed-result
// crossing on the lens side. The hop suspends the frame, and the suspension has
// to invalidate the read the application armed as well, or a lens the user left
// behind installs itself over the destination.
func TestALensApplicationInFlightCannotPublishOverAHop(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	// A project facet the live population was not read under is what makes the
	// application asynchronous rather than immediate.
	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	lens := state.NamedLens("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d01", "Parked included")
	lens.Query = "+parked"
	if _, err := outlinerView.ApplyNamedLens(lens); err != nil {
		t.Fatalf("ApplyNamedLens: %v", err)
	}
	if !outlinerView.Lens.hasPending() {
		t.Fatal("the application published without reading a population")
	}
	generation := outlinerView.Lens.Pending.Generation

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
	if _, published := outlinerView.HandleLensProjectsLoaded(LensProjectsLoadedMsg{
		Projects:   []*models.Project{reloadedAlpha, reloadedBeta},
		Generation: generation,
	}); published {
		t.Fatal("a lens read landing after the hop installed itself over the destination")
	}
	if outlinerView.Lens.hasActive() {
		t.Fatal("a lens read landing after the hop went live")
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
}

// TestAnAbandonedParkedFocusLoadPublishesNothing is the parked exception's own
// crossing. The focus request was abandoned when the hop pushed its scope, so
// its read carries a project the live view has not asked for: publishing it
// would open a parked project nobody is looking at and take the cursor with it.
func TestAnAbandonedParkedFocusLoadPublishesNothing(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	parked := renderFixtureProject(t, "proj-p", "parked", "Parked", []*models.Task{
		renderFixtureTask(t, "parked-task", 1, "Parked work", models.StateBacklog, 1, models.StateBacklog),
	})
	parked.Tags = []string{"parked"}
	setTestProjects(outlinerView.Runtime, []*models.Project{alpha, beta, parked})

	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	_ = outlinerView.BeginParkedProjectFocus("proj-p")
	generation := outlinerView.Population.LoadGen

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	if outlinerView.Scope.PendingParkedID != "" {
		t.Fatalf("the hop left the focus request outstanding: %q", outlinerView.Scope.PendingParkedID)
	}

	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:       []*models.Project{alpha, beta, parked},
		Generation:     generation,
		FocusProjectID: "proj-p",
	})

	if outlinerView.Scope.ProjectID != "proj-b" {
		t.Fatalf("the abandoned focus pushed a scope: %q", outlinerView.Scope.ProjectID)
	}
	if outlinerView.Scope.ParkedProjectID != "" {
		t.Fatalf("the abandoned focus installed an exception: %q", outlinerView.Scope.ParkedProjectID)
	}
	if outlinerView.Population.Loading {
		t.Fatal("the abandoned read was never put down")
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
	if !outlinerView.HasRelationTrail() {
		t.Fatal("the abandoned focus discarded the return trail")
	}
}

// parkedOriginOutliner is the settled parked exception: one parked project the
// base workspace does not admit, focused by name the way P focuses it, with the
// load that admits it already landed and its scope already pushed.
func parkedOriginOutliner(t *testing.T, alpha, beta *models.Project) *OutlinerModel {
	t.Helper()
	alpha.Tags = []string{models.ParkedProjectTag}
	outlinerView := scopeTestOutliner(alpha, beta)
	_ = outlinerView.BeginParkedProjectFocus("proj-a")
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:       []*models.Project{alpha, beta},
		Generation:     outlinerView.Population.LoadGen,
		FocusProjectID: "proj-a",
	})
	if outlinerView.Scope.ProjectID != "proj-a" || outlinerView.Scope.ParkedProjectID != "proj-a" {
		t.Fatalf("the focus did not settle: scope = %q exception = %q",
			outlinerView.Scope.ProjectID, outlinerView.Scope.ParkedProjectID)
	}
	return outlinerView
}

// TestARelationReturnKeepsAnExplicitlyFocusedParkedOrigin is the parked
// exception's other crossing. The origin is in the snapshot only because the
// user named it, and a hop is a visit with a promised way back, so the exception
// is retained across it rather than shed the way leaving a focus sheds it. The
// return is owed the view it left, and a parked project the workspace still
// holds is not a project that went away.
func TestARelationReturnKeepsAnExplicitlyFocusedParkedOrigin(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("the return landed in scope %q, want the parked origin", outlinerView.Scope.ProjectID)
	}
	if outlinerView.ScopedProject() == nil {
		t.Fatal("the return restored the origin's scope without the project behind it")
	}
	if outlinerView.Scope.ParkedProjectID != "proj-a" {
		t.Fatalf("the exception admitting the origin is gone: %q", outlinerView.Scope.ParkedProjectID)
	}
	if strings.Contains(outlinerView.StatusMessage, "no longer exists") {
		t.Fatalf("the return reported a repair it did not need: %q", outlinerView.StatusMessage)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	// The visited project is off screen again: the restored scope is the origin's.
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.ProjectID == "proj-b" {
			t.Fatal("the return kept the visited project's tasks on screen")
		}
	}
}

// TestARetainedParkedOriginKeepsReadingCurrentData is what retaining the
// exception buys the return: the origin is still the project every later read
// re-requests, so a reload landing at the destination refreshes it and the
// return shows that reading rather than the one the hop departed from.
func TestARetainedParkedOriginKeepsReadingCurrentData(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	// A steady-state reload lands at the destination carrying the adopted focus,
	// which is the way an ordinary refresh reaches a parked project at all.
	reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
	reloadedAlpha.Tags = []string{models.ParkedProjectTag}
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:               []*models.Project{reloadedAlpha, reloadedBeta},
		Generation:             outlinerView.Population.LoadGen,
		retainedFocusProjectID: "proj-a",
	})
	if outlinerView.Scope.ParkedProjectID != "proj-a" {
		t.Fatalf("a reload at the destination retired the exception: %q",
			outlinerView.Scope.ParkedProjectID)
	}

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the return put a stale reading of the parked origin back: sibling = %q", got)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}

// TestARelationReturnFallsBackWhenTheParkedOriginWentAway is the other half of
// that rule. Retaining the exception is not a promise the project survived, so a
// parked origin that was deleted, archived, or made unreadable while the user was
// away still unwinds to the base scope and says so.
func TestARelationReturnFallsBackWhenTheParkedOriginWentAway(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	// The origin is gone: the read that lands carries the visited project alone.
	_, reloadedBeta := reloadedRelationFixture(t)
	setTestProjects(outlinerView.Runtime, []*models.Project{reloadedBeta})
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedBeta)

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.IsProjectScoped() {
		t.Fatalf("the return reinstated a scope on the missing %q", outlinerView.Scope.ProjectID)
	}
	if outlinerView.Scope.ParkedProjectID != "" {
		t.Fatalf("the repair left an exception re-requesting a project that is gone: %q",
			outlinerView.Scope.ParkedProjectID)
	}
	if !strings.Contains(outlinerView.StatusMessage, "returned to the base scope") {
		t.Fatalf("the repair went unreported: %q", outlinerView.StatusMessage)
	}
}

// originStateFacet is one row facet armed the way a filter chord arms it, so the
// axis under test is the one the checkpoint carries rather than a field poked
// into place.
func originStateFacet(outlinerView *OutlinerModel, state models.WorkflowState) {
	clauses := query.AppendFilterClause(nil, models.QueryFieldState,
		models.QueryOperatorIs, state.String())
	outlinerView.ReplaceFilterClauses(query.FilterQueryTaskFields(), clauses)
}

// stateFacetValues reads the armed state facet back as its values, so an
// assertion names what the user narrowed to.
func stateFacetValues(outlinerView *OutlinerModel) []string {
	values := make([]string, 0, 2)
	for _, clause := range query.ClausesOfFields(outlinerView.FilterQuery, models.QueryFieldState) {
		values = append(values, clause.Value)
	}
	return values
}

// TestAHopReplacesTheOriginRowFacetsAndTheReturnPutsThemBack is the changed-
// population-filter crossing. The hop owns the row facets while it is standing
// on its target — it installs the at most two predicates that target needs and
// nothing else — so the origin's narrowing has to come back from the frame, and
// come back over the rows as they now read rather than as they read when it was
// captured.
func TestAHopReplacesTheOriginRowFacetsAndTheReturnPutsThemBack(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	// Doing selects the source and drops its sibling, so the facet is observable
	// in the rows and the return still has the row it departed from to land on.
	originStateFacet(outlinerView, models.StateDoing)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	// Waiting is neither terminal nor deferred, so the target needs no predicate
	// at all and the origin's narrowing is simply gone.
	if got := stateFacetValues(outlinerView); len(got) != 0 {
		t.Fatalf("the hop carried the origin's state facet into the destination: %v", got)
	}

	landReloadedFixture(t, outlinerView, outlinerView.Population.LoadGen)
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}

	got := stateFacetValues(outlinerView)
	if len(got) != 1 || got[0] != models.StateDoing.String() {
		t.Fatalf("the return put back %v, want the origin's doing facet", got)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == "sibling" {
			t.Fatal("the restored facet is not selecting rows")
		}
	}
	// The facet is the frame's; the reading it selects over is not. The sibling was
	// retitled while the user was away, and the population the restored narrowing
	// runs against has to be that one.
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the restored facet is selecting over a stale reading: sibling = %q", got)
	}
}

// TestAHopRefusesATargetThatLeftThePopulationWhileThePickerWasOpen is the
// archived-target crossing. Archiving, parking, or deleting the target's project
// all reach the view the same way: the next read simply does not carry it. The
// picker was drawn while it was still there, so the row is on screen and the key
// press is legitimate; the resolution behind it is not, and a hop that cannot
// land must leave the view and the trail exactly as it found them.
func TestAHopRefusesATargetThatLeftThePopulationWhileThePickerWasOpen(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	relationPickerOver(t, outlinerView, "proj-a", "source")
	if !outlinerView.RelationPicker.Surface.SetCursorToID(relationPickerRowID("out:proj-b:remote")) {
		t.Fatal("the cross-project target is not listed")
	}

	// The target's project is archived: the read that lands carries the origin
	// alone, and the row the cursor is sitting on now names nothing.
	reloadedAlpha, _ := reloadedRelationFixture(t)
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedAlpha)

	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	if outlinerView.RelationPicker.Notice != relationStaleTargetRefusal {
		t.Fatalf("notice = %q, want %q", outlinerView.RelationPicker.Notice, relationStaleTargetRefusal)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a refused hop published a return frame")
	}
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("a refused hop moved the scope to %q", outlinerView.Scope.ProjectID)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}
