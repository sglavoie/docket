package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/alerts"
)

type alertCenterSection struct {
	label       string
	alerts      []alerts.Alert
	maxSeverity int
	isGlobal    bool
}

const globalAlertSectionKey = "global"

// buildItems flattens alerts into project/global sections, ordered by severity.
func (m *NotificationCenterModel) buildItems() {
	m.items = nil
	m.sectionStarts = nil
	sections := m.projectSections()
	if len(sections) == 0 {
		return
	}

	for _, section := range sections {
		m.items = append(m.items, alertCenterItem{
			isHeader: true,
			label:    sectionHeaderLabel(section.label, len(section.alerts)),
		})
		m.sectionStarts = append(m.sectionStarts, len(m.items))
		for _, a := range section.alerts {
			label, details := notificationCenterText(a)
			m.items = append(m.items, alertCenterItem{
				isHeader:  false,
				label:     label,
				details:   details,
				dismissed: m.showDismissed && m.dismissedSet[a.DismissalKey()],
				alert:     a,
			})
		}
	}
}

// rebuildForShowDismissed switches to showing the raw snapshot and rebuilds items.
func (m *NotificationCenterModel) rebuildForShowDismissed() {
	m.buildItems()
	m.clampCursor()
}

func (m *NotificationCenterModel) projectSections() []alertCenterSection {
	snap := m.activeSnapshot()
	if snap == nil || len(snap.Groups) == 0 {
		return nil
	}

	sectionsByKey := make(map[string]*alertCenterSection)
	order := make([]string, 0, len(snap.Groups))
	for _, group := range snap.Groups {
		for _, a := range group.Alerts {
			key := alertSectionKey(a)
			section, ok := sectionsByKey[key]
			if !ok {
				section = &alertCenterSection{
					label:    a.ProjectDisplayName(),
					isGlobal: a.IsGlobal(),
				}
				sectionsByKey[key] = section
				order = append(order, key)
			}
			section.alerts = append(section.alerts, a)
			if a.Severity > section.maxSeverity {
				section.maxSeverity = a.Severity
			}
		}
	}

	sections := make([]alertCenterSection, 0, len(order))
	for _, key := range order {
		section := sectionsByKey[key]
		sortAlertSection(section.alerts)
		sections = append(sections, *section)
	}

	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].maxSeverity != sections[j].maxSeverity {
			return sections[i].maxSeverity > sections[j].maxSeverity
		}
		if sections[i].isGlobal != sections[j].isGlobal {
			return sections[i].isGlobal
		}
		return strings.ToLower(sections[i].label) < strings.ToLower(sections[j].label)
	})

	return sections
}

func sortAlertSection(items []alerts.Alert) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Severity != items[j].Severity {
			return items[i].Severity > items[j].Severity
		}
		if alertCategoryRank(items[i].Category) != alertCategoryRank(items[j].Category) {
			return alertCategoryRank(items[i].Category) < alertCategoryRank(items[j].Category)
		}
		if !strings.EqualFold(items[i].Title, items[j].Title) {
			return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
		}
		return strings.ToLower(items[i].Details) < strings.ToLower(items[j].Details)
	})
}

func alertCategoryRank(category alerts.AlertCategory) int {
	switch category {
	case alerts.CategoryDailyReview:
		return 0
	case alerts.CategoryWeeklyReview:
		return 1
	case alerts.CategoryOverdue:
		return 2
	case alerts.CategoryDueSoon:
		return 3
	case alerts.CategoryStale:
		return 4
	case alerts.CategoryInbox:
		return 5
	case alerts.CategoryBlocked:
		return 6
	case alerts.CategoryProjectHealth:
		return 7
	default:
		return 100
	}
}

func alertSectionKey(a alerts.Alert) string {
	if a.IsGlobal() {
		return globalAlertSectionKey
	}
	if a.ProjectID != "" {
		return "project:" + a.ProjectID
	}
	return "project-name:" + a.ProjectDisplayName()
}

func sectionHeaderLabel(label string, count int) string {
	if count <= 1 {
		return label
	}
	return fmt.Sprintf("%s (%d)", label, count)
}

func notificationCenterText(a alerts.Alert) (string, string) {
	label := a.Title
	if label == "" {
		label = alerts.CategoryDisplayName(a.Category)
	}
	if a.IsGlobal() {
		return label, a.Details
	}
	if a.Details == "" {
		return label, alerts.CategoryDisplayName(a.Category)
	}
	return label, alerts.CategoryDisplayName(a.Category) + " · " + a.Details
}
