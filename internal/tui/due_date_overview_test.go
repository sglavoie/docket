package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestDueDateOverviewCategorizeTask(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	m := &DueDateOverviewModel{}

	tests := []struct {
		name    string
		dueDate time.Time
		want    DueDateGroup
	}{
		{"overdue by 3 days", today.AddDate(0, 0, -3), GroupOverdue},
		{"overdue by 1 day", today.AddDate(0, 0, -1), GroupOverdue},
		{"today", today, GroupToday},
		{"tomorrow", today.AddDate(0, 0, 1), GroupThisWeek},
		{"in 3 days", today.AddDate(0, 0, 3), GroupThisWeek},
		{"in 7 days", today.AddDate(0, 0, 7), GroupThisWeek},
		{"in 8 days", today.AddDate(0, 0, 8), GroupLater},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			due := tc.dueDate
			task := &models.Task{DueDate: &due}
			got := m.categorizeTask(task)
			if got != tc.want {
				t.Errorf("categorizeTask() = %v, want %v", got, tc.want)
			}
		})
	}
}
