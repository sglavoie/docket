package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// analyzeCurrentValues examines all selected tasks to build summaries.
func (m *BulkEditModel) analyzeCurrentValues() { //nolint:gocyclo
	if len(m.tasks) == 0 {
		return
	}

	stateCounts := make(map[models.WorkflowState]int)
	for _, task := range m.tasks {
		stateCounts[task.State()]++
	}
	if len(stateCounts) == 1 {
		for state := range stateCounts {
			m.statusSummary = fmt.Sprintf("All: %s", state)
		}
	} else {
		parts := make([]string, 0, len(stateCounts))
		for state, count := range stateCounts {
			parts = append(parts, fmt.Sprintf("%d %s", count, state))
		}
		m.statusSummary = fmt.Sprintf("Mixed: %s", strings.Join(parts, ", "))
	}

	typeCounts := make(map[models.TaskType]int)
	for _, task := range m.tasks {
		typeCounts[task.Type]++
	}
	if len(typeCounts) == 1 {
		for taskType := range typeCounts {
			m.typeSummary = fmt.Sprintf("All: %s", taskType.HeadingPrefix())
		}
	} else {
		parts := make([]string, 0, len(typeCounts))
		for taskType, count := range typeCounts {
			parts = append(parts, fmt.Sprintf("%d %s", count, taskType.HeadingPrefix()))
		}
		m.typeSummary = fmt.Sprintf("Mixed: %s", strings.Join(parts, ", "))
	}

	priorities := make(map[int]bool)
	minPriority, maxPriority := 6, 0
	for _, task := range m.tasks {
		priorities[task.Priority] = true
		if task.Priority < minPriority {
			minPriority = task.Priority
		}
		if task.Priority > maxPriority {
			maxPriority = task.Priority
		}
	}
	if len(priorities) == 1 {
		m.prioritySummary = fmt.Sprintf("All: P%d", minPriority)
	} else {
		m.prioritySummary = fmt.Sprintf("Mixed: P%d-P%d", minPriority, maxPriority)
	}

	tagPresence := make(map[string]int)
	for _, task := range m.tasks {
		for _, tag := range task.Tags {
			tagPresence[tag]++
		}
	}

	m.commonTags = []string{}
	m.allTags = []string{}
	for tag, count := range tagPresence {
		m.allTags = append(m.allTags, tag)
		if count == len(m.tasks) {
			m.commonTags = append(m.commonTags, tag)
		}
	}
	sort.Strings(m.commonTags)
	sort.Strings(m.allTags)

	sections := make(map[string]int)
	for _, task := range m.tasks {
		sections[taskSectionBucket(task)]++
	}
	if len(sections) == 1 {
		for section := range sections {
			if section == "" {
				m.sectionSummary = "All: Active (no section)"
			} else {
				m.sectionSummary = fmt.Sprintf("All: %s", section)
			}
		}
	} else {
		parts := make([]string, 0, len(sections))
		for section, count := range sections {
			name := section
			if name == "" {
				name = "Active"
			}
			parts = append(parts, fmt.Sprintf("%d %s", count, name))
		}
		m.sectionSummary = fmt.Sprintf("Mixed: %s", strings.Join(parts, ", "))
	}

	withDate, withoutDate := 0, 0
	for _, task := range m.tasks {
		if task.DueDate != nil {
			withDate++
		} else {
			withoutDate++
		}
	}
	if withDate == 0 {
		m.dueDateSummary = "None have due dates"
	} else if withoutDate == 0 {
		m.dueDateSummary = "All have due dates"
	} else {
		m.dueDateSummary = fmt.Sprintf("%d with dates, %d without", withDate, withoutDate)
	}

	withRecurrence, withoutRecurrence := 0, 0
	recurrencePatterns := make(map[string]int)
	for _, task := range m.tasks {
		if task.Recurrence != nil {
			withRecurrence++
			recurrencePatterns[task.Recurrence.String()]++
		} else {
			withoutRecurrence++
		}
	}
	if withRecurrence == 0 {
		m.recurrenceSummary = "None recurring"
	} else if withoutRecurrence == 0 {
		if len(recurrencePatterns) == 1 {
			for pattern := range recurrencePatterns {
				m.recurrenceSummary = fmt.Sprintf("All: %s", pattern)
			}
		} else {
			m.recurrenceSummary = "All recurring (mixed)"
		}
	} else {
		m.recurrenceSummary = fmt.Sprintf("%d recurring, %d not", withRecurrence, withoutRecurrence)
	}

	parentCounts := make(map[string]int)
	rootCount := 0
	for _, task := range m.tasks {
		if task.ParentID == nil {
			rootCount++
		} else {
			parentCounts[*task.ParentID]++
		}
	}
	if rootCount == len(m.tasks) {
		m.parentSummary = "All at root"
	} else if rootCount == 0 && len(parentCounts) == 1 {
		for parentID := range parentCounts {
			for _, task := range m.project.Tasks {
				if task.ID == parentID {
					m.parentSummary = fmt.Sprintf("All under: %s", task.Title)
					break
				}
			}
		}
	} else {
		m.parentSummary = fmt.Sprintf("%d root, %d with parent", rootCount, len(m.tasks)-rootCount)
	}
}

// buildSectionOptions creates the list of available sections.
func (m *BulkEditModel) buildSectionOptions() {
	sectionSet := make(map[string]bool)

	for _, section := range m.project.GetSections() {
		sectionSet[section] = true
	}

	for _, task := range m.project.Tasks {
		if bucket := taskSectionBucket(task); bucket != "" {
			sectionSet[bucket] = true
		}
	}

	m.sectionOptions = []string{""}
	sections := []string{}
	for section := range sectionSet {
		if section != "" {
			sections = append(sections, section)
		}
	}
	sort.Strings(sections)
	m.sectionOptions = append(m.sectionOptions, sections...)
}

// buildParentCandidates creates the list of valid parent targets.
// Excludes tasks in the selected set and tasks that cannot have children.
func (m *BulkEditModel) buildParentCandidates() {
	selectedSet := make(map[string]bool, len(m.tasks))
	for _, task := range m.tasks {
		selectedSet[task.ID] = true
	}

	m.parentCandidates = nil
	for _, task := range m.project.Tasks {
		if selectedSet[task.ID] {
			continue
		}
		if !task.Type.CanHaveChildren() {
			continue
		}
		m.parentCandidates = append(m.parentCandidates, task)
	}
}
