package tui

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

// buildProjectMixFixture holds the five project shapes the Select Project
// open-task filter distinguishes: active with an open task, active with none,
// active whose tasks are all finished, parked with an open task, and archived
// with none. It is the only fixture carrying an archived project and the only
// one where "holds open tasks" and "reaches the base task workspace" disagree in
// both directions; the finished project is what the two narrowings separate,
// since it holds tasks and no open work, so `with open tasks` hides it and
// `without open tasks` lists it beside the two known-empty projects.
func buildProjectMixFixture(w *uitestWorkspace) error {
	active, err := w.addProject(projectMixActiveName, func(p *models.Project) {
		p.Description = projectMixActiveDescription
		p.Tags = []string{"commerce"}
		// The one pinned project is also the alphabetically first one, so
		// pinned-first ordering leaves the active listing in exactly the order it
		// had before the pin — and Ctrl+P still reaches a non-empty pinned Hub.
		p.Pinned = true

		rollout := newFixtureTask(p, "Mix rollout plan")
		rollout.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(projectMixEmptyName, func(p *models.Project) {
		p.Description = projectMixEmptyDescription
		p.Tags = []string{"charter"}
	}); err != nil {
		return err
	}
	if _, err = w.addProject(projectMixFinishedName, func(p *models.Project) {
		p.Description = projectMixFinishedDescription
		p.Tags = []string{"housekeeping"}

		markDone(newFixtureTask(p, "Mix cleanup sweep"))
		markDone(newFixtureTask(p, "Mix cleanup follow-up"))
	}); err != nil {
		return err
	}
	if _, err = w.addProject(projectMixParkedName, func(p *models.Project) {
		p.Description = projectMixParkedDescription
		p.Tags = []string{models.ParkedProjectTag, "vendor"}

		migration := newFixtureTask(p, "Mix migration audit")
		migration.ApplyWorkflowState(models.StateBacklog)
	}); err != nil {
		return err
	}
	// Archiving happens inside the mutator because catalog.UpsertProject runs
	// after it, so the flag is what the catalog records.
	if _, err = w.addProject(projectMixArchivedName, func(p *models.Project) {
		p.Description = projectMixArchivedDescription
		p.Tags = []string{"kiosk"}
		p.Archive()
	}); err != nil {
		return err
	}
	w.setCurrentProject(active.ID)
	return nil
}

// buildSignalsFixture carries the time signals no other fixture holds: a deferral
// that has already lapsed beside a whole-day and an hour-valued one still
// pending, overdue work concentrated in one project, and a durable Today
// membership. Its project name is
// deliberately long so a project header's pressure suffix has to degrade at
// narrow widths.
func buildSignalsFixture(w *uitestWorkspace) error {
	var triage *models.Task
	project, err := w.addProject(signalsFixtureProjectName, func(p *models.Project) {
		audit := newFixtureTask(p, "Alpha overdue audit")
		audit.ApplyWorkflowState(models.StateBacklog)
		setDue(audit, -3*24*time.Hour)

		rollout := newFixtureTask(p, "Alpha overdue rollout")
		rollout.ApplyWorkflowState(models.StateBacklog)
		setDue(rollout, -24*time.Hour)

		woke := newFixtureTask(p, "Alpha back from a break")
		woke.ApplyWorkflowState(models.StateBacklog)
		lapsed := time.Now().AddDate(0, 0, -1)
		woke.DeferredUntil = &lapsed
		woke.DeferCount = 3
		woke.Context = "difficult-review"
		woke.Checklist = []models.ChecklistItem{{Text: "Review the concern"}}
		woke.Comments = []*models.Comment{{Text: "Deferred before", CreatedAt: time.Now()}}
		woke.Attachments = []models.Attachment{{Filename: "brief.md", AddedAt: time.Now()}}

		pending := newFixtureTask(p, "Alpha still hidden")
		pending.ApplyWorkflowState(models.StateBacklog)
		deferred := util.StartOfDay(time.Now()).AddDate(0, 0, 5)
		pending.DeferredUntil = &deferred

		// A midnight deferral renders bare, so the clock ladder needs one row
		// whose wake time is a fixed hour rather than an offset from now.
		morning := newFixtureTask(p, "Alpha waking at nine")
		morning.ApplyWorkflowState(models.StateBacklog)
		tomorrow := time.Now().AddDate(0, 0, 1)
		wake := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, time.Local)
		morning.DeferredUntil = &wake

		triage = newFixtureTask(p, "Alpha overdue intake")
		triage.ApplyWorkflowState(models.StateInbox)
		setDue(triage, -2*24*time.Hour)

		// The two extra hours keep the day bucket at 12 after the clock
		// round-trips through the schema-8 writer's second-precision timestamp.
		blocked := newFixtureTask(p, "Alpha waiting on vendor")
		blocked.ApplyWorkflowState(models.StateWaiting)
		blocked.StateChangedAt = time.Now().Add(-12*24*time.Hour - 2*time.Hour)
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(signalsFixtureOtherName, func(p *models.Project) {
		steady := newFixtureTask(p, "Beta steady work")
		steady.ApplyWorkflowState(models.StateBacklog)
	}); err != nil {
		return err
	}
	w.Runtime.Planning().AddToToday(triage.ID, project.ID)
	w.setCurrentProject(project.ID)
	return nil
}

// buildStaleFixture isolates old touch times so lifecycle-lane placement and the
// fs facet can be exercised without shifting cursor-sensitive scenarios on
// other fixtures.
// Both stale tasks carry one shared tag, which is what lets a vocabulary rename
// reach exactly the untouched population: the rename must leave every timestamp
// alone, so the lane it is run from has to be the lane it is measured against.
func buildStaleFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Dormant Delivery", func(p *models.Project) {
		old := time.Now().AddDate(0, 0, -(models.StaleTaskThresholdDays + 10))
		for _, title := range []string{
			"Revisit dormant launch checklist",
			"Renew untouched vendor briefing",
		} {
			task := newFixtureTask(p, title)
			task.ApplyWorkflowState(models.StateBacklog)
			task.Tags = []string{"dormant"}
			task.CreatedAt = old
			task.UpdatedAt = old
			task.StateChangedAt = old
		}

		fresh := newFixtureTask(p, "Draft current launch note")
		fresh.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildChecklistFixture is the one fixture whose tasks carry checklist items, so
// the inline checklist rows and their fold glyph can be exercised without
// shifting any other fixture's projection. It holds three shapes the fold ladder
// distinguishes: a checklist with no children, a checklist beneath a task that
// also has a child, and a task with neither.
func buildChecklistFixture(w *uitestWorkspace) error {
	project, err := w.addProject(checklistFixtureProjectName, func(p *models.Project) {
		runbook := newFixtureTask(p, checklistFixtureFoldTitle)
		runbook.ApplyWorkflowState(models.StateBacklog)
		runbook.Checklist = []models.ChecklistItem{
			{Text: checklistFixtureFirstItem, Done: true},
			{Text: checklistFixtureSecondItem},
			{Text: checklistFixtureThirdItem},
		}

		rollout := newFixtureTask(p, checklistFixtureParentTitle)
		rollout.ApplyWorkflowState(models.StateBacklog)
		rollout.Checklist = []models.ChecklistItem{
			{Text: "Book the maintenance window"},
			{Text: "Notify the support rota", Done: true},
		}
		soak := newFixtureTask(p, "Confirm the staging soak")
		soak.ApplyWorkflowState(models.StateBacklog)
		soak.ParentID = &rollout.ID

		plain := newFixtureTask(p, checklistFixturePlainTitle)
		plain.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// newFixtureTask creates a task with the next short ID, appends it to the
// project, and returns it for further customization before the tree is built.
func newFixtureTask(p *models.Project, title string) *models.Task {
	task, err := models.NewGenericTask(uuid.New().String(), p.NextShortID, title)
	if err != nil {
		// Fixture titles are authored in this file and validated by tests, so a
		// failure here is a programming error, not runtime input.
		panic(fmt.Sprintf("newFixtureTask(%q): %v", title, err))
	}
	p.Tasks = append(p.Tasks, task)
	p.NextShortID++
	return task
}

// markDone completes a task via the canonical done state and stamps CompletedAt
// like the lifecycle transition would.
func markDone(t *models.Task) {
	now := time.Now()
	t.ApplyWorkflowState(models.StateDone)
	t.CompletedAt = &now
}

// setDue sets a due date relative to now, on the whole minute the grammar
// would have produced. util.ParseDueDate builds a timed due with zero seconds
// and treats a midnight clock as the date-only signal, so a raw time.Now()
// offset is a value no user can enter: its seconds render away and the
// schema-13 codec refuses the "00:00" that is left, which broke every fixture
// build during the one minute a day the offset landed on midnight.
func setDue(t *models.Task, offset time.Duration) {
	due := fixtureDueInstant(time.Now(), offset)
	t.DueDate = &due
}

// fixtureDueInstant is setDue's clock arithmetic, split out so a test can drive
// it from a base instant the wall clock only visits for one minute a day.
func fixtureDueInstant(base time.Time, offset time.Duration) time.Time {
	return base.Add(offset).Truncate(time.Minute)
}
