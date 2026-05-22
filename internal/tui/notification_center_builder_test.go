package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/alerts"
)

func TestNotificationCenterBuildItems_GroupsByProjectAndGlobal(t *testing.T) {
	m := NewNotificationCenterModel(testNotificationCenterSnapshot(), testNotificationCenterSnapshot(), map[string]bool{})

	var headers []string
	for _, item := range m.items {
		if item.isHeader {
			headers = append(headers, item.label)
		}
	}

	if !reflect.DeepEqual(headers, []string{"Alpha (2)", "Global", "Beta"}) {
		t.Fatalf("unexpected section headers: %v", headers)
	}
	if !reflect.DeepEqual(m.sectionStarts, []int{1, 4, 6}) {
		t.Fatalf("unexpected section starts: %v", m.sectionStarts)
	}
	if m.cursor != 1 {
		t.Fatalf("expected cursor on first alert, got %d", m.cursor)
	}
	if got := m.items[1].details; got != "Overdue · Due Jan 2" {
		t.Fatalf("expected project-scoped details, got %q", got)
	}
	if got := m.items[2].details; got != "Stale · No updates for 14 days" {
		t.Fatalf("expected second Alpha alert to keep category context, got %q", got)
	}
	if got := m.items[4].details; got != "Last completed yesterday" {
		t.Fatalf("expected global alert details to remain unchanged, got %q", got)
	}
}

func TestNotificationCenterSectionNavigationUsesProjectSections(t *testing.T) {
	m := NewNotificationCenterModel(testNotificationCenterSnapshot(), testNotificationCenterSnapshot(), map[string]bool{})

	m.SectionGoToNext(m.sectionStarts)
	if m.cursor != 4 {
		t.Fatalf("expected next section cursor at 4, got %d", m.cursor)
	}

	m.SectionGoToNext(m.sectionStarts)
	if m.cursor != 6 {
		t.Fatalf("expected next section cursor at 6, got %d", m.cursor)
	}

	m.SectionGoToPrev(m.sectionStarts)
	if m.cursor != 4 {
		t.Fatalf("expected previous section cursor at 4, got %d", m.cursor)
	}

	m.cursor = 1
	m.SectionGoToPrev(m.sectionStarts)
	if m.cursor != 6 {
		t.Fatalf("expected wrapped previous section cursor at 6, got %d", m.cursor)
	}
}

func testNotificationCenterSnapshot() *alerts.AlertSnapshot {
	return &alerts.AlertSnapshot{
		Groups: []alerts.AlertGroup{
			{
				Category: alerts.CategoryOverdue,
				Alerts: []alerts.Alert{
					{
						Category:    alerts.CategoryOverdue,
						Title:       "Critical deadline",
						Details:     "Due Jan 2",
						ProjectID:   "alpha-id",
						ProjectName: "Alpha",
						Severity:    alerts.SeverityUrgent,
					},
					{
						Category:    alerts.CategoryOverdue,
						Title:       "Routine follow-up",
						Details:     "Due Jan 5",
						ProjectID:   "beta-id",
						ProjectName: "Beta",
						Severity:    alerts.SeverityInfo,
					},
				},
			},
			{
				Category: alerts.CategoryDailyReview,
				Alerts: []alerts.Alert{{
					Category: alerts.CategoryDailyReview,
					Title:    "Daily review pending",
					Details:  "Last completed yesterday",
					Severity: alerts.SeverityWarning,
				}},
			},
			{
				Category: alerts.CategoryStale,
				Alerts: []alerts.Alert{{
					Category:    alerts.CategoryStale,
					Title:       "Needs refresh",
					Details:     "No updates for 14 days",
					ProjectID:   "alpha-id",
					ProjectName: "Alpha",
					Severity:    alerts.SeverityInfo,
				}},
			},
		},
		TotalCount: 4,
	}
}
