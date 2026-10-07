package engine

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ReminderKind names what a reminder is about. Each kind is a moment the
// workspace already records; a reminder never invents one.
type ReminderKind string

const (
	// ReminderCheckIn is a scheduled check-in whose day arrives. Its fire time
	// is the check-in day at the device's reminder time of day, because the
	// check-in itself is a calendar date and never an instant.
	ReminderCheckIn ReminderKind = "check_in"
	// ReminderDeadline is a deadline carrying an explicit time of day. A
	// date-only deadline is a day, not a moment, and raises no reminder.
	ReminderDeadline ReminderKind = "deadline"
	// ReminderDeferralEnds is the instant a pending deferral lapses and the
	// task returns to view.
	ReminderDeferralEnds ReminderKind = "deferral_ends"
)

// ReminderOptions is everything a reminder plan depends on besides the
// workspace, passed explicitly so the plan never reads the process clock or
// zone behind the caller's back.
type ReminderOptions struct {
	// Now is the instant the plan is made at; nothing at or before it is
	// planned, so a reminder whose moment passed is never raised late.
	Now time.Time
	// Location is the device's zone. Check-in days and timed deadlines are
	// wall-clock values and fire at that wall-clock time in this zone, so a
	// device that changes zone replans them; a deferral is an instant and
	// fires at the same moment in every zone.
	Location *time.Location
	// CheckInHour and CheckInMinute are the local time of day a check-in day
	// is announced at.
	CheckInHour, CheckInMinute int
	// Horizon bounds how far ahead the plan looks; zero means 30 days.
	Horizon time.Duration
	// Limit bounds how many reminders the plan returns, soonest first; zero
	// means 60, under the 64 pending local notifications iOS keeps per app.
	Limit int
}

// Reminder is one planned notification.
type Reminder struct {
	Kind           ReminderKind
	ProjectID      string
	ProjectName    string
	TaskID         string
	ShortID        int
	Title          string
	FireAt         time.Time
	CanAcknowledge bool
	CanDefer       bool
}

// ReminderPlan is the reminders a device should have scheduled now.
type ReminderPlan struct {
	Reminders []Reminder
	// Truncated reports that the limit cut reminders inside the horizon.
	Truncated bool
}

const (
	defaultReminderHorizon = 30 * 24 * time.Hour
	defaultReminderLimit   = 60
)

// Validate refuses options that cannot name a moment.
func (o ReminderOptions) Validate() error {
	switch {
	case o.Location == nil:
		return fmt.Errorf("reminders need a time zone")
	case o.Now.IsZero():
		return fmt.Errorf("reminders need the current instant")
	case o.CheckInHour < 0 || o.CheckInHour > 23 || o.CheckInMinute < 0 || o.CheckInMinute > 59:
		return fmt.Errorf("check-in time %02d:%02d is not a time of day", o.CheckInHour, o.CheckInMinute)
	case o.Horizon < 0 || o.Limit < 0:
		return fmt.Errorf("horizon and limit must not be negative")
	}
	return nil
}

// Reminders plans the notifications this device should hold for the open
// workspace, reread from disk so an import or an external edit is seen.
func (r *Runtime) Reminders(opts ReminderOptions) (ReminderPlan, error) {
	if err := opts.Validate(); err != nil {
		return ReminderPlan{}, err
	}
	if err := r.Refresh(); err != nil {
		return ReminderPlan{}, fmt.Errorf("reload workspace domains: %w", err)
	}
	var loaded []*models.Project
	for _, project := range r.state.TaskWorkspaceProjects() {
		if project == nil {
			continue
		}
		if err := storage.LoadProject(project); err != nil {
			// A project that does not load raises nothing; the query surfaces
			// name it, and a reminder plan is not where a load failure is read.
			continue
		}
		loaded = append(loaded, project)
	}
	return PlanReminders(loaded, opts), nil
}

// PlanReminders is the planning rule over loaded projects, which are the task
// workspace population: archived and parked projects raise nothing.
//
// It applies the shared attention rules rather than restating them. A
// check-in is planned for the moment it would start asking for attention,
// which is exactly when models.Task.CheckInIsDueAt turns true at that
// instant: terminal and still-deferred work raises nothing, and a check-in
// already owed raises nothing new because its moment has passed. The review
// flag has no moment and is never a reminder. A deadline is planned only when
// it carries a time of day, on open work not deferred past it; a deferral is
// planned when it lapses (a day-long deferral that morning), on open work.
func PlanReminders(projects []*models.Project, opts ReminderOptions) ReminderPlan {
	horizon := opts.Horizon
	if horizon == 0 {
		horizon = defaultReminderHorizon
	}
	limit := opts.Limit
	if limit == 0 {
		limit = defaultReminderLimit
	}
	end := opts.Now.Add(horizon)
	upcoming := func(at time.Time) bool { return at.After(opts.Now) && !at.After(end) }

	var plan []Reminder
	for _, project := range projects {
		for _, task := range project.Tasks {
			if task == nil || task.IsTerminalState() {
				continue
			}
			base := Reminder{
				ProjectID: project.ID, ProjectName: project.Name,
				TaskID: task.ID, ShortID: task.ShortID, Title: task.Title,
			}
			if at, ok := checkInMoment(task, opts); ok && upcoming(at) && task.CheckInIsDueAt(at) {
				reminder := base
				reminder.Kind, reminder.FireAt = ReminderCheckIn, at
				reminder.CanAcknowledge = models.TaskAllowsExplicitReview(task, at)
				reminder.CanDefer = true
				plan = append(plan, reminder)
			}
			if task.DueHasTime() {
				at := wallClock(*task.DueDate, opts.Location)
				if upcoming(at) && !task.IsDeferredAt(at) {
					reminder := base
					reminder.Kind, reminder.FireAt, reminder.CanDefer = ReminderDeadline, at, true
					reminder.CanAcknowledge = models.TaskAllowsExplicitReview(task, at)
					plan = append(plan, reminder)
				}
			}
			if at, ok := deferralMoment(task, opts); ok && upcoming(at) {
				reminder := base
				reminder.Kind, reminder.FireAt, reminder.CanDefer = ReminderDeferralEnds, at, true
				reminder.CanAcknowledge = models.TaskAllowsExplicitReview(task, at)
				plan = append(plan, reminder)
			}
		}
	}
	slices.SortStableFunc(plan, func(a, b Reminder) int {
		if c := a.FireAt.Compare(b.FireAt); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ProjectID, b.ProjectID); c != 0 {
			return c
		}
		if c := cmp.Compare(a.TaskID, b.TaskID); c != 0 {
			return c
		}
		return cmp.Compare(a.Kind, b.Kind)
	})
	result := ReminderPlan{Reminders: plan}
	if len(plan) > limit {
		result.Reminders, result.Truncated = plan[:limit], true
	}
	return result
}

// checkInMoment is the check-in day at the reminder time of day in the
// device's zone. The stored date is read by its calendar components, never
// as an instant, so the same date names the same local day in every zone.
func checkInMoment(task *models.Task, opts ReminderOptions) (time.Time, bool) {
	if task.NextCheckIn == nil {
		return time.Time{}, false
	}
	year, month, day := task.NextCheckIn.Date()
	return time.Date(year, month, day, opts.CheckInHour, opts.CheckInMinute, 0, 0, opts.Location), true
}

// deferralMoment is when a pending deferral is announced. A deferral is an
// instant and lapses at the same moment in every zone. A deferral to a day
// (the date forms land on local midnight) is announced that morning at the
// reminder time of day rather than at midnight; the task is back in view from
// midnight either way.
func deferralMoment(task *models.Task, opts ReminderOptions) (time.Time, bool) {
	if task.DeferredUntil == nil || !task.DeferredUntil.After(opts.Now) {
		return time.Time{}, false
	}
	local := task.DeferredUntil.In(opts.Location)
	if hour, minute, second := local.Clock(); hour == 0 && minute == 0 && second == 0 && local.Nanosecond() == 0 {
		year, month, day := local.Date()
		return time.Date(year, month, day, opts.CheckInHour, opts.CheckInMinute, 0, 0, opts.Location), true
	}
	return task.DeferredUntil.UTC(), true
}

// wallClock re-reads a stored wall-clock value in the device's zone: a
// deadline at 14:30 is due at 14:30 wherever the device is.
func wallClock(value time.Time, location *time.Location) time.Time {
	year, month, day := value.Date()
	hour, minute, second := value.Clock()
	return time.Date(year, month, day, hour, minute, second, value.Nanosecond(), location)
}
