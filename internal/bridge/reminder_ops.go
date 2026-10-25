package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sglavoie/docket/internal/engine"
)

type remindersPayload struct {
	// TimeZone is the device's zone now. It is required rather than inherited
	// from workspace.open, because a device that travels replans its
	// wall-clock reminders in the zone it is in, and the plan says which.
	TimeZone string `json:"time_zone"`
	// CheckInTime is the local time of day a check-in day is announced, as
	// HH:MM.
	CheckInTime string `json:"check_in_time"`
	HorizonDays int    `json:"horizon_days,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type reminderView struct {
	// ID names the reminder's subject, not its moment: one task's check-in,
	// deadline, or deferral. A rescheduled moment keeps the id and changes the
	// fingerprint, so a client replaces the notification rather than keeping
	// both.
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Task        locator `json:"task"`
	ProjectName string  `json:"project_name"`
	ShortID     int     `json:"short_id"`
	Title       string  `json:"title"`
	FireAt      string  `json:"fire_at"`
	// LocalTime is FireAt as wall-clock time in the plan's zone.
	LocalTime      string `json:"local_time"`
	CanAcknowledge bool   `json:"can_acknowledge"`
	CanDefer       bool   `json:"can_defer"`
	Fingerprint    string `json:"fingerprint"`
}

type remindersResult struct {
	TimeZone    string         `json:"time_zone"`
	CheckInTime string         `json:"check_in_time"`
	GeneratedAt string         `json:"generated_at"`
	Truncated   bool           `json:"truncated"`
	Reminders   []reminderView `json:"reminders"`
}

// queryReminders plans the local notifications this device should hold: the
// engine applies the shared attention rules and the client only schedules,
// replaces, and cancels what the plan names.
func (b *Bridge) queryReminders(request Request) (any, error) {
	var payload remindersPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.TimeZone == "" {
		return nil, newError(CodeInvalidRequest, "time_zone is required")
	}
	if err := adoptTimeZone(payload.TimeZone); err != nil {
		return nil, err
	}
	clock, err := time.Parse("15:04", payload.CheckInTime)
	if err != nil {
		return nil, newError(CodeValidation, "check_in_time %q is not HH:MM", payload.CheckInTime).with("field", "check_in_time")
	}
	if payload.HorizonDays < 0 || payload.HorizonDays > 366 || payload.Limit < 0 || payload.Limit > 64 {
		return nil, newError(CodeInvalidRequest, "horizon_days must be 0-366 and limit 0-64")
	}
	now := b.now()
	plan, err := b.workspace.runtime.Reminders(engine.ReminderOptions{
		Now:           now,
		Location:      time.Local,
		CheckInHour:   clock.Hour(),
		CheckInMinute: clock.Minute(),
		Horizon:       time.Duration(payload.HorizonDays) * 24 * time.Hour,
		Limit:         payload.Limit,
	})
	if err != nil {
		return nil, newError(CodeValidation, "%v", err)
	}
	result := remindersResult{
		TimeZone:    time.Local.String(),
		CheckInTime: clock.Format("15:04"),
		GeneratedAt: now.UTC().Format(time.RFC3339Nano),
		Truncated:   plan.Truncated,
		Reminders:   make([]reminderView, 0, len(plan.Reminders)),
	}
	for _, reminder := range plan.Reminders {
		view := reminderView{
			ID:             fmt.Sprintf("%s/%s/%s", reminder.Kind, reminder.ProjectID, reminder.TaskID),
			Kind:           string(reminder.Kind),
			Task:           locator{ProjectID: reminder.ProjectID, TaskID: reminder.TaskID},
			ProjectName:    reminder.ProjectName,
			ShortID:        reminder.ShortID,
			Title:          reminder.Title,
			FireAt:         reminder.FireAt.UTC().Format(time.RFC3339),
			LocalTime:      reminder.FireAt.In(time.Local).Format("2006-01-02T15:04:05"),
			CanAcknowledge: reminder.CanAcknowledge,
			CanDefer:       reminder.CanDefer,
		}
		sum := sha256.Sum256(fmt.Appendf(nil, "%s\x00%s\x00%s\x00%s\x00%t\x00%t",
			view.ID, view.FireAt, view.Title, view.ProjectName, view.CanAcknowledge, view.CanDefer))
		view.Fingerprint = hex.EncodeToString(sum[:8])
		result.Reminders = append(result.Reminders, view)
	}
	return result, nil
}
