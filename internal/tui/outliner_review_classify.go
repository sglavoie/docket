package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/util"
)

// DailyReviewSection represents the different sections in the Daily Review
// computed Outliner mode. The standalone Daily Review model was retired; these
// section identities and the comparators/thresholds below are the package-level
// classification primitives reused by the Outliner review row builders.
type DailyReviewSection int

const (
	SectionOverdue DailyReviewSection = iota
	SectionDueToday
	SectionDueSoon
	SectionDueLater
	SectionFuture
	SectionInProgress
	SectionBlocked
	SectionInReview
	SectionOnStandby
	SectionOld
	SectionCompleted
)

// isToday returns true if t falls on the current calendar day in the local timezone.
func isToday(t time.Time) bool {
	now := time.Now()
	tl := t.In(now.Location())
	return tl.Year() == now.Year() && tl.YearDay() == now.YearDay()
}

type dailyReviewThresholds struct {
	today              time.Time
	dueSoonDays        int
	futureDays         int
	oldDays            int
	completedHours     int
	escalationDays     int
	dueSoonEnd         time.Time
	escalationEnd      time.Time
	futureStart        time.Time
	oldThreshold       time.Time
	completedThreshold time.Time
}

func newDailyReviewThresholds(settings *config.Settings, now time.Time) dailyReviewThresholds {
	today := util.StartOfDay(now)
	dueSoonDays := settings.GetReviewDueSoonDays()
	futureDays := settings.GetReviewFutureDays()
	oldDays := settings.GetReviewOldDays()
	completedHours := settings.GetReviewCompletedHours()
	escalationDays := settings.GetPriorityEscalationDays()

	return dailyReviewThresholds{
		today:              today,
		dueSoonDays:        dueSoonDays,
		futureDays:         futureDays,
		oldDays:            oldDays,
		completedHours:     completedHours,
		escalationDays:     escalationDays,
		dueSoonEnd:         today.AddDate(0, 0, dueSoonDays+1),
		escalationEnd:      today.AddDate(0, 0, escalationDays+1),
		futureStart:        today.AddDate(0, 0, futureDays),
		oldThreshold:       now.AddDate(0, 0, -oldDays),
		completedThreshold: now.Add(-time.Duration(completedHours) * time.Hour),
	}
}

type crossTaskLess func(a, b CrossProjectTask) bool

func byProjectDisplayOrderThen(less crossTaskLess) crossTaskLess {
	return func(a, b CrossProjectTask) bool {
		if c := compareProjectsByDisplayOrder(a.Project, b.Project); c != 0 {
			return c < 0
		}
		return less(a, b)
	}
}

func sortDailyReviewSection(tasks []CrossProjectTask, less crossTaskLess) {
	less = byProjectDisplayOrderThen(less)
	sort.Slice(tasks, func(i, j int) bool { return less(tasks[i], tasks[j]) })
}

// crossTaskLess comparators for recurring sort patterns on CrossProjectTask slices.
func byOverdueDueDateThenID(a, b CrossProjectTask) bool {
	if !a.Task.DueDate.Equal(*b.Task.DueDate) {
		return a.Task.DueDate.Before(*b.Task.DueDate)
	}
	return a.Task.ID < b.Task.ID
}

func byDueDateThenPriority(a, b CrossProjectTask) bool {
	if !a.Task.DueDate.Equal(*b.Task.DueDate) {
		return a.Task.DueDate.Before(*b.Task.DueDate)
	}
	if a.Task.Priority != b.Task.Priority {
		return a.Task.Priority < b.Task.Priority
	}
	return a.Task.ID < b.Task.ID
}

func byPriorityThenID(a, b CrossProjectTask) bool {
	if a.Task.Priority != b.Task.Priority {
		return a.Task.Priority < b.Task.Priority
	}
	return a.Task.ID < b.Task.ID
}

func byPriorityThenUpdatedAt(a, b CrossProjectTask) bool {
	if a.Task.Priority != b.Task.Priority {
		return a.Task.Priority < b.Task.Priority
	}
	if !a.Task.UpdatedAt.Equal(b.Task.UpdatedAt) {
		return a.Task.UpdatedAt.After(b.Task.UpdatedAt)
	}
	return a.Task.ID < b.Task.ID
}

func byEffectiveUpdatedAtThenID(a, b CrossProjectTask) bool {
	ti := a.Task.UpdatedAt
	if ti.IsZero() {
		ti = a.Task.CreatedAt
	}
	tj := b.Task.UpdatedAt
	if tj.IsZero() {
		tj = b.Task.CreatedAt
	}
	if !ti.Equal(tj) {
		return ti.Before(tj)
	}
	return a.Task.ID < b.Task.ID
}

func byCompletedAtThenID(a, b CrossProjectTask) bool {
	if a.Task.CompletedAt == nil {
		return false
	}
	if b.Task.CompletedAt == nil {
		return true
	}
	if !a.Task.CompletedAt.Equal(*b.Task.CompletedAt) {
		return a.Task.CompletedAt.After(*b.Task.CompletedAt)
	}
	return a.Task.ID < b.Task.ID
}

// getSectionName returns a human-readable name for a Daily Review section.
func getSectionName(section DailyReviewSection) string {
	switch section {
	case SectionOverdue:
		return "Overdue"
	case SectionDueToday:
		return "Today"
	case SectionDueSoon:
		return "Due Soon"
	case SectionDueLater:
		return "Due Later"
	case SectionFuture:
		return "Future"
	case SectionInProgress:
		return "In Progress"
	case SectionBlocked:
		return "Blocked"
	case SectionInReview:
		return "In Review"
	case SectionOnStandby:
		return "On Standby"
	case SectionOld:
		return "Old"
	case SectionCompleted:
		return "Completed"
	}
	return ""
}
