package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// buildEmptyFixture leaves the workspace with no user projects, exercising the
// zero-task dashboard capture hint.
func buildEmptyFixture(_ *uitestWorkspace) error {
	return nil
}

// buildFreshFixture removes even the harness-seeded sandbox so NewAppModel must
// bootstrap a genuine blank install, then starts on the configured default view.
func buildFreshFixture(w *uitestWorkspace) {
	w.Runtime.SetCatalogInMemory(config.NewWorkspaceCatalog())
	w.startAtDefault = true
}

// buildRichFixture creates a project with a full task hierarchy spanning every
// retained workflow states, priorities, hierarchy, and relationships.
func buildRichFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Rich Project", func(p *models.Project) {
		epic := newFixtureTask(p, "Launch platform")
		epic.Description = "Ship the first public release."
		epic.Priority = 1

		story := newFixtureTask(p, "User authentication")
		story.ParentID = &epic.ID
		story.Priority = 2

		login := newFixtureTask(p, "Build login form")
		login.ParentID = &story.ID
		login.ApplyWorkflowState(models.StateDoing)

		validate := newFixtureTask(p, "Add password validation")
		validate.ParentID = &story.ID
		validate.Tags = []string{"backend", "security"}
		validate.Context = "platform"

		mockup := newFixtureTask(p, "Design login mockup")
		mockup.ParentID = &story.ID
		markDone(mockup)

		bug := newFixtureTask(p, "Fix redirect loop after logout")
		bug.Priority = 1
		bug.Tags = []string{"urgent"}
		bug.ApplyWorkflowState(models.StateWaiting)

		idea := newFixtureTask(p, "Explore social login")
		idea.Description = "Google / GitHub OAuth."
		idea.ApplyWorkflowState(models.StateSomeday)

		inbox := newFixtureTask(p, "Triage support inbox")
		inbox.ApplyWorkflowState(models.StateInbox)
	})
	if err != nil {
		return err
	}
	globalNote := &models.Note{
		Title:      "Workspace launch brief",
		CreatedAt:  time.Date(2025, time.January, 2, 9, 0, 0, 0, time.UTC),
		ModifiedAt: time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC),
		Filename:   "2025-01-02-090000-workspace-launch-brief.md",
		Body:       "Shared launch context for every active project.\n",
	}
	projectNote := &models.Note{
		Title:      "Rich Project delivery plan",
		CreatedAt:  time.Date(2025, time.January, 3, 9, 0, 0, 0, time.UTC),
		ModifiedAt: time.Date(2025, time.January, 3, 10, 0, 0, 0, time.UTC),
		Filename:   "2025-01-03-090000-rich-project-delivery-plan.md",
		Body:       richProjectNoteBody(),
	}
	w.noteSeeds = append(w.noteSeeds,
		fixtureNoteSeed{dir: storage.GlobalNotesDir(w.DataDir), note: globalNote},
		fixtureNoteSeed{dir: storage.NotesDir(project), note: projectNote},
	)
	w.setCurrentProject(project.ID)
	return nil
}

// richProjectNoteBody is the one note body long enough to page and to anchor
// against. Its first line is unchanged from the single-line body it grew out of,
// so every committed Notes search scenario still matches and counts exactly what
// it did before. Below it sit Markdown headings (which the pane renders rather
// than shows), forty numbered rows (so a page turn is observable), and one
// deliberately rare word — kestrel — far enough down that a body match can only
// be on screen because the preview anchored on it.
func richProjectNoteBody() string {
	lines := []string{
		"Track the authentication milestone and release readiness.",
		"",
		"## Delivery checkpoints",
		"",
	}
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("- Checkpoint %02d confirms the staged rollout gate.", i))
	}
	lines = append(lines,
		"",
		"## Rollback rehearsal",
		"",
		"The rehearsal runs on the Friday before launch with the kestrel runbook as its fallback.",
		"",
	)
	return strings.Join(lines, "\n")
}

// buildBusyFixture creates a dense mix of urgent, scheduled, waiting,
// and inbox work for dashboard stress tests.
func buildBusyFixture(w *uitestWorkspace) error {
	return buildBusyFixtureAt(w, time.Now())
}

func buildBusyFixtureAt(w *uitestWorkspace, now time.Time) error {
	project, err := w.addProject("Busy Project", func(p *models.Project) {
		overdue := newFixtureTask(p, "Overdue: submit quarterly report")
		overdue.Priority = 1
		overdue.ApplyWorkflowState(models.StateBacklog)
		setDue(overdue, -3*24*time.Hour)

		dueSoon := newFixtureTask(p, "Due soon: renew certificate")
		dueSoon.Priority = 2
		dueSoon.ApplyWorkflowState(models.StateBacklog)
		// Deadline stepping starts tomorrow regardless of the current hour.
		due := fixtureDueInstant(now.AddDate(0, 0, 1), 0)
		dueSoon.DueDate = &due

		urgentWaiting := newFixtureTask(p, "Waiting: production outage")
		urgentWaiting.Priority = 1
		urgentWaiting.ApplyWorkflowState(models.StateWaiting)

		waiting := newFixtureTask(p, "Waiting on vendor response")
		waiting.ApplyWorkflowState(models.StateWaiting)

		completed := newFixtureTask(p, "Completed: archive quarterly paperwork")
		markDone(completed)
		completedAt := time.Now().AddDate(0, 0, -45)
		completed.CompletedAt = &completedAt
		completed.UpdatedAt = completedAt

		canceled := newFixtureTask(p, "Canceled: retire obsolete certificate")
		canceled.ApplyWorkflowState(models.StateCanceled)
		canceledAt := time.Now().AddDate(0, 0, -120)
		canceled.CompletedAt = &canceledAt
		canceled.UpdatedAt = canceledAt
	})
	if err != nil {
		return err
	}
	_, err = w.addProject("Inbox Backlog", func(p *models.Project) {
		stale := time.Now().Add(-21 * 24 * time.Hour)
		for i := 0; i < 5; i++ {
			task := newFixtureTask(p, fmt.Sprintf("Untriaged inbox item %d", i+1))
			task.ApplyWorkflowState(models.StateInbox)
			task.CreatedAt = stale
			task.UpdatedAt = stale
		}
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildWorkflowFixture creates a spread of tasks across workflow states so the
// state and planning operations have something to render.
func buildWorkflowFixture(w *uitestWorkspace) error {
	titles := []string{
		"Sketch intake flow",
		"Design settings screen",
		"Implement export pipeline",
		"Review release notes",
	}
	project, err := w.addProject("Workflow Project", func(p *models.Project) {
		for i, title := range titles {
			t := newFixtureTask(p, title)
			t.Priority = (i % 3) + 1
			t.ApplyWorkflowState(models.StateBacklog)
			if i == len(titles)-1 {
				markDone(t)
			}
		}
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildNarrowFixture creates tasks with long titles and tags to stress layout
// when the driver is run at a narrow terminal width.
func buildNarrowFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Narrow Project", func(p *models.Project) {
		long := newFixtureTask(p,
			"Investigate intermittent timeout when syncing very large project archives over slow connections")

		long.Priority = 1
		long.Tags = []string{"infrastructure", "reliability", "sync"}
		long.Context = "distributed-systems"
		long.ApplyWorkflowState(models.StateBacklog)

		another := newFixtureTask(p,
			"Truncated column headers overlap when the terminal is resized below the minimum comfortable width")

		another.ApplyWorkflowState(models.StateDoing)
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildReleaseFixture creates a large canonical Outliner catalog with Unicode
// content. The release-hardening fixture exercises wide and minimum layouts,
// viewport bounds, and repeated resizes
// against substantially more rows than the everyday review fixtures.
func buildReleaseFixture(w *uitestWorkspace) error {
	states := []models.WorkflowState{
		models.StateInbox,
		models.StateBacklog,
		models.StateBacklog,
		models.StateDoing,
		models.StateWaiting,
		models.StatePaused,
		models.StateSomeday,
	}
	project, err := w.addProject("Release Ω Project", func(p *models.Project) {
		unicodeTask := newFixtureTask(p, "国際化 🚀 café résumé release check")
		unicodeTask.Description = "Unicode display-width and visible-search acceptance row."
		unicodeTask.Tags = []string{"i18n", "release"}
		unicodeTask.ApplyWorkflowState(models.StateDoing)

		for i := 0; i < 240; i++ {
			task := newFixtureTask(
				p,

				fmt.Sprintf("Release acceptance item %03d with bounded viewport content", i+1))

			task.Priority = (i % 3) + 1
			task.Tags = []string{"release", fmt.Sprintf("batch-%d", i%8)}
			task.ApplyWorkflowState(states[i%len(states)])
		}
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildMultiProjectFixture spreads one backlog task across four distinctly named
// projects so a scenario that searches or moves between projects has more than
// one plausible destination to choose from.
func buildMultiProjectFixture(w *uitestWorkspace) error {
	projects := []struct {
		name string
		task string
	}{
		{"Client Alpha", "Alpha migration kickoff"},
		{"Client Beta", "Beta rollout checklist"},
		{"Home Errands", "Renew passport"},
		{"Reading List", "Finish the distributed systems book"},
	}
	var first *models.Project
	for _, spec := range projects {
		project, err := w.addProject(spec.name, func(p *models.Project) {
			task := newFixtureTask(p, spec.task)
			task.ApplyWorkflowState(models.StateBacklog)
		})
		if err != nil {
			return err
		}
		if first == nil {
			first = project
		}
	}
	w.setCurrentProject(first.ID)
	return nil
}

// buildInboxTriageFixture is the only fixture whose Inbox spans more than one
// project group, which is the single row shape where the Inbox cursor advance
// is observable: the last task of a non-final group has to hand the cursor to
// the first task of the next one. The `busy` fixture keeps its whole Inbox in
// one project and every other multi-project fixture carries backlog work only,
// so neither can express the crossing. Its second group also holds the nested
// Inbox parent whose surviving child is promoted into its place when the parent
// is triaged away.
//
// It is deliberately its own fixture rather than an extension of
// `multi-project`, whose four projects are read by twenty-one committed
// scenarios that would all see a changed row population.
func buildInboxTriageFixture(w *uitestWorkspace) error {
	// Alphabetical project names fix the group order, since project groups sort
	// pinned-first and then by name and nothing here is pinned.
	alpha, err := w.addProject(inboxTriageAlphaName, func(p *models.Project) {
		// The first group's two tasks are the only rows Peek can tell apart by
		// content alone, so each carries a description no other row repeats. That
		// is how the undo scenario proves which row the cursor is on.
		lead := newFixtureTask(p, inboxTriageAlphaLeadTitle)
		lead.ApplyWorkflowState(models.StateInbox)
		lead.Description = inboxTriageAlphaLeadDetail

		final := newFixtureTask(p, inboxTriageAlphaFinalTitle)
		final.ApplyWorkflowState(models.StateInbox)
		final.Description = inboxTriageAlphaFinalDetail
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(inboxTriageBetaName, func(p *models.Project) {
		parent := newFixtureTask(p, inboxTriageBetaParentTitle)
		parent.ApplyWorkflowState(models.StateInbox)

		child := newFixtureTask(p, inboxTriageBetaChildTitle)
		child.ApplyWorkflowState(models.StateInbox)
		child.ParentID = &parent.ID

		trailing := newFixtureTask(p, inboxTriageBetaTrailingTitle)
		trailing.ApplyWorkflowState(models.StateInbox)
	}); err != nil {
		return err
	}
	w.setCurrentProject(alpha.ID)
	return nil
}

// buildParkedFixture tags one project parked and leaves the other in the base
// task workspace, which is the only fixture shape where the base task workspace
// is a strict subset of the workspace's projects. Every other fixture leaves all
// of its projects untagged, so parked behavior would be indistinguishable from
// an empty catalog there.
func buildParkedFixture(w *uitestWorkspace) error {
	active, err := w.addProject(parkedFixtureWorkspaceName, func(p *models.Project) {
		checklist := newFixtureTask(p, "Ship the delivery checklist")
		checklist.Priority = 1
		checklist.ApplyWorkflowState(models.StateBacklog)

		window := newFixtureTask(p, "Confirm the release window")
		window.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(parkedFixtureParkedName, func(p *models.Project) {
		p.Tags = []string{models.ParkedProjectTag}

		migration := newFixtureTask(p, "Revisit the paused migration")
		migration.ApplyWorkflowState(models.StateBacklog)

		vendor := newFixtureTask(p, "Reopen the vendor conversation")
		vendor.ApplyWorkflowState(models.StateInbox)
	}); err != nil {
		return err
	}
	w.setCurrentProject(active.ID)
	return nil
}
