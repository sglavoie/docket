package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// updateValidParents filters tasks that can be parents of the current type.
func (m *TaskEditorModel) updateValidParents() {
	m.validParents = []*models.Task{}

	for _, task := range m.project.Tasks {
		// Can't be parent of itself or its descendants
		if task.ID == m.task.ID {
			continue
		}
		if m.isDescendant(task, m.task) {
			continue
		}
		if task.CanAddChild(m.taskType) {
			m.validParents = append(m.validParents, task)
		}
	}

	// Find current parent cursor position
	m.parentCursor = 0
	if m.parentID != nil {
		for i, p := range m.validParents {
			if p.ID == *m.parentID {
				m.parentCursor = i + 1 // +1 because 0 is "None"
				break
			}
		}
		// If parent no longer valid, clear it
		if m.parentCursor == 0 {
			m.parentID = nil
		}
	}
}

// isDescendant checks if candidate is a descendant of task.
func (m *TaskEditorModel) isDescendant(candidate, task *models.Task) bool {
	for _, child := range task.Children {
		if child.ID == candidate.ID {
			return true
		}
		if m.isDescendant(candidate, child) {
			return true
		}
	}
	return false
}

// checkChanges detects if any field has been modified.
func (m *TaskEditorModel) checkChanges() bool {
	// Normalize title by replacing newlines with spaces (same as saveTask)
	currentTitle := strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	if currentTitle != m.originalTitle {
		return true
	}
	if strings.Join(m.tags, ", ") != m.originalTags {
		return true
	}
	if m.taskType != m.originalType {
		return true
	}
	if m.priority != m.originalPriority {
		return true
	}
	// Compare parent IDs
	if m.parentID == nil && m.originalParentID != nil {
		return true
	}
	if m.parentID != nil && m.originalParentID == nil {
		return true
	}
	if m.parentID != nil && m.originalParentID != nil && *m.parentID != *m.originalParentID {
		return true
	}
	// Check section
	if m.section != m.originalSection {
		return true
	}
	// Check due date
	if m.dueDate == nil && m.originalDueDate != nil {
		return true
	}
	if m.dueDate != nil && m.originalDueDate == nil {
		return true
	}
	if m.dueDate != nil && m.originalDueDate != nil && !m.dueDate.Equal(*m.originalDueDate) {
		return true
	}
	// Check flags
	if m.pinned != m.originalPinned {
		return true
	}
	if m.active != m.originalActive {
		return true
	}
	// Check recurrence
	if !recurrenceEqual(m.recurrence, m.originalRecurrence) {
		return true
	}
	// Check URL and Source
	if m.urlInput != m.originalURL {
		return true
	}
	if m.sourceInput != m.originalSource {
		return true
	}
	// Check type-specific attributes
	if m.attrsChanged() {
		return true
	}
	return false
}

// attrsChanged returns true if any type-specific attribute has been modified.
func (m *TaskEditorModel) attrsChanged() bool {
	switch m.taskType {
	case models.TaskTypeBug:
		current := &models.BugAttrs{}
		if m.bugReproducibleCursor > 0 {
			current.Reproducible = models.ValidBugReproducible[m.bugReproducibleCursor-1]
		}
		if m.bugSeverityCursor > 0 {
			current.Severity = models.ValidBugSeverity[m.bugSeverityCursor-1]
		}
		current.Workaround = m.bugWorkaround
		return !bugAttrsEqual(current, m.originalBugAttrs)
	case models.TaskTypeIdea:
		current := &models.IdeaAttrs{}
		if m.ideaCategoryCursor > 0 {
			current.Category = models.ValidIdeaCategory[m.ideaCategoryCursor-1]
		}
		if m.ideaEffortCursor > 0 {
			current.Effort = models.ValidEffortEstimate[m.ideaEffortCursor-1]
		}
		if m.ideaValueCursor > 0 {
			current.Value = models.ValidIdeaValue[m.ideaValueCursor-1]
		}
		current.Validated = m.ideaValidated
		return !ideaAttrsEqual(current, m.originalIdeaAttrs)
	case models.TaskTypeTask:
		current := &models.TaskAttrs{}
		if m.taskEffortCursor > 0 {
			current.Effort = models.ValidEffortEstimate[m.taskEffortCursor-1]
		}
		if m.taskWorkTypeCursor > 0 {
			current.WorkType = models.ValidWorkType[m.taskWorkTypeCursor-1]
		}
		return !taskAttrsEqual(current, m.originalTaskAttrs)
	case models.TaskTypeStory:
		current := &models.StoryAttrs{}
		if m.storyPointsCursor > 0 {
			current.StoryPoints = storyPointsValues[m.storyPointsCursor]
		}
		current.Persona = m.storyPersona
		return !storyAttrsEqual(current, m.originalStoryAttrs)
	case models.TaskTypeEpic:
		current := &models.EpicAttrs{}
		if m.epicScopeCursor > 0 {
			current.Scope = models.ValidEpicScope[m.epicScopeCursor-1]
		}
		current.Milestone = m.epicMilestone
		current.SuccessMetrics = m.epicSuccessMetrics
		return !epicAttrsEqual(current, m.originalEpicAttrs)
	}
	return false
}

func bugAttrsEqual(a, b *models.BugAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		// Check if non-nil one is empty
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Reproducible == b.Reproducible && a.Severity == b.Severity && a.Workaround == b.Workaround
}

func ideaAttrsEqual(a, b *models.IdeaAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Category == b.Category && a.Effort == b.Effort && a.Value == b.Value && a.Validated == b.Validated
}

func taskAttrsEqual(a, b *models.TaskAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Effort == b.Effort && a.WorkType == b.WorkType
}

func storyAttrsEqual(a, b *models.StoryAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.StoryPoints == b.StoryPoints && a.Persona == b.Persona
}

func epicAttrsEqual(a, b *models.EpicAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Scope == b.Scope && a.Milestone == b.Milestone && a.SuccessMetrics == b.SuccessMetrics
}

// recurrenceEqual compares two recurrence patterns for equality.
func recurrenceEqual(a, b *models.RecurrencePattern) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Interval != b.Interval || a.Unit != b.Unit {
		return false
	}
	// Compare weekdays
	if len(a.Weekdays) != len(b.Weekdays) {
		return false
	}
	for i, wd := range a.Weekdays {
		if wd != b.Weekdays[i] {
			return false
		}
	}
	return true
}

// updateRecurrenceFromCursor updates the recurrence pattern based on cursor position.
func (m *TaskEditorModel) updateRecurrenceFromCursor() {
	switch m.recurrenceCursor {
	case 0: // None
		m.recurrence = nil
	case 1: // Daily
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	case 2: // Weekly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}
	case 3: // Biweekly
		m.recurrence = &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}
	case 4: // Monthly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly}
	case 5: // Yearly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceYearly}
	}
}

// updateFlagsFromCursor updates pinned/active based on cursor position.
func (m *TaskEditorModel) updateFlagsFromCursor() {
	switch m.flagCursor {
	case 0: // None
		m.pinned = false
		m.active = false
	case 1: // Pin
		m.pinned = true
		m.active = false
	case 2: // Active
		m.pinned = false
		m.active = true
	}
}

// flagCursorFromFlags returns the cursor position based on pinned/active flags.
func flagCursorFromFlags(pinned, active bool) int {
	if pinned {
		return 1 // Pin
	}
	if active {
		return 2 // Active
	}
	return 0 // None
}

// parseDueDate parses the due date input string and updates m.dueDate.
func (m *TaskEditorModel) parseDueDate() error {
	value := strings.TrimSpace(m.dueDateInput)

	// Handle empty or "none" to clear due date
	if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
		m.dueDate = nil
		return nil
	}

	// Parse the date using the existing ParseDate function
	date, err := ParseDate(value)
	if err != nil {
		return err
	}

	m.dueDate = &date
	return nil
}

// blurCurrentField removes focus from the currently focused field.
func (m *TaskEditorModel) blurCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Blur()
	}
}

// focusCurrentField sets focus to the currently focused field.
func (m *TaskEditorModel) focusCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Focus()
	}
}

// loadExistingAttrs loads type-specific attributes from the task into form state.
func (m *TaskEditorModel) loadExistingAttrs(task *models.Task) {
	// Store original attrs for change detection
	if task.BugAttrs != nil {
		copy := *task.BugAttrs
		m.originalBugAttrs = &copy
	}
	if task.IdeaAttrs != nil {
		copy := *task.IdeaAttrs
		m.originalIdeaAttrs = &copy
	}
	if task.TaskAttrs != nil {
		copy := *task.TaskAttrs
		m.originalTaskAttrs = &copy
	}
	if task.StoryAttrs != nil {
		copy := *task.StoryAttrs
		m.originalStoryAttrs = &copy
	}
	if task.EpicAttrs != nil {
		copy := *task.EpicAttrs
		m.originalEpicAttrs = &copy
	}

	// Load bug attrs
	if task.BugAttrs != nil {
		m.bugReproducibleCursor = findIndexInSlice(models.ValidBugReproducible, task.BugAttrs.Reproducible) + 1
		m.bugSeverityCursor = findIndexInSlice(models.ValidBugSeverity, task.BugAttrs.Severity) + 1
		m.bugWorkaround = task.BugAttrs.Workaround
	}

	// Load idea attrs
	if task.IdeaAttrs != nil {
		m.ideaCategoryCursor = findIndexInSlice(models.ValidIdeaCategory, task.IdeaAttrs.Category) + 1
		m.ideaEffortCursor = findIndexInSlice(models.ValidEffortEstimate, task.IdeaAttrs.Effort) + 1
		m.ideaValueCursor = findIndexInSlice(models.ValidIdeaValue, task.IdeaAttrs.Value) + 1
		m.ideaValidated = task.IdeaAttrs.Validated
	}

	// Load task attrs
	if task.TaskAttrs != nil {
		m.taskEffortCursor = findIndexInSlice(models.ValidEffortEstimate, task.TaskAttrs.Effort) + 1
		m.taskWorkTypeCursor = findIndexInSlice(models.ValidWorkType, task.TaskAttrs.WorkType) + 1
	}

	// Load story attrs
	if task.StoryAttrs != nil {
		m.storyPointsCursor = findStoryPointsCursor(task.StoryAttrs.StoryPoints)
		m.storyPersona = task.StoryAttrs.Persona
	}

	// Load epic attrs
	if task.EpicAttrs != nil {
		m.epicScopeCursor = findIndexInSlice(models.ValidEpicScope, task.EpicAttrs.Scope) + 1
		m.epicMilestone = task.EpicAttrs.Milestone
		m.epicSuccessMetrics = task.EpicAttrs.SuccessMetrics
	}
}

// findIndexInSlice returns the index of value in slice, or -1 if not found.
func findIndexInSlice[T comparable](slice []T, value T) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return -1
}

// findStoryPointsCursor returns the cursor index for a story points value.
func findStoryPointsCursor(points int) int {
	for i, v := range storyPointsValues {
		if v == points {
			return i
		}
	}
	return 0 // (none)
}

// getAttrFieldCount returns the number of focusable attribute fields for the current task type.
func (m *TaskEditorModel) getAttrFieldCount() int {
	switch m.taskType {
	case models.TaskTypeBug:
		return 3 // reproducible, severity, workaround
	case models.TaskTypeIdea:
		return 4 // category, effort, value, validated
	case models.TaskTypeTask:
		return 2 // effort, workType
	case models.TaskTypeStory:
		return 2 // storyPoints, persona
	case models.TaskTypeEpic:
		return 3 // scope, milestone, successMetrics
	}
	return 0
}

// attrsStartIndex returns the focus index where attribute fields begin.
func (m *TaskEditorModel) attrsStartIndex() int {
	return 10 // After source (index 9)
}

// totalFieldCount returns total number of focusable fields.
func (m *TaskEditorModel) totalFieldCount() int {
	return m.attrsStartIndex() + m.getAttrFieldCount()
}

// hasNonEmptyAttrs returns true if the current task type has any non-empty attribute values.
func (m *TaskEditorModel) hasNonEmptyAttrs() bool {
	switch m.taskType {
	case models.TaskTypeBug:
		return m.bugReproducibleCursor > 0 || m.bugSeverityCursor > 0 || m.bugWorkaround != ""
	case models.TaskTypeIdea:
		return m.ideaCategoryCursor > 0 || m.ideaEffortCursor > 0 || m.ideaValueCursor > 0 || m.ideaValidated
	case models.TaskTypeTask:
		return m.taskEffortCursor > 0 || m.taskWorkTypeCursor > 0
	case models.TaskTypeStory:
		return m.storyPointsCursor > 0 || m.storyPersona != ""
	case models.TaskTypeEpic:
		return m.epicScopeCursor > 0 || m.epicMilestone != "" || m.epicSuccessMetrics != ""
	}
	return false
}

// clearCurrentAttrs resets all attribute values for the current task type.
func (m *TaskEditorModel) clearCurrentAttrs() {
	switch m.taskType {
	case models.TaskTypeBug:
		m.bugReproducibleCursor = 0
		m.bugSeverityCursor = 0
		m.bugWorkaround = ""
	case models.TaskTypeIdea:
		m.ideaCategoryCursor = 0
		m.ideaEffortCursor = 0
		m.ideaValueCursor = 0
		m.ideaValidated = false
	case models.TaskTypeTask:
		m.taskEffortCursor = 0
		m.taskWorkTypeCursor = 0
	case models.TaskTypeStory:
		m.storyPointsCursor = 0
		m.storyPersona = ""
	case models.TaskTypeEpic:
		m.epicScopeCursor = 0
		m.epicMilestone = ""
		m.epicSuccessMetrics = ""
	}
}

// handleAttrLeft handles left arrow key for attribute fields.
func (m *TaskEditorModel) handleAttrLeft(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0:
			if m.bugReproducibleCursor > 0 {
				m.bugReproducibleCursor--
			}
		case 1:
			if m.bugSeverityCursor > 0 {
				m.bugSeverityCursor--
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0:
			if m.ideaCategoryCursor > 0 {
				m.ideaCategoryCursor--
			}
		case 1:
			if m.ideaEffortCursor > 0 {
				m.ideaEffortCursor--
			}
		case 2:
			if m.ideaValueCursor > 0 {
				m.ideaValueCursor--
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0:
			if m.taskEffortCursor > 0 {
				m.taskEffortCursor--
			}
		case 1:
			if m.taskWorkTypeCursor > 0 {
				m.taskWorkTypeCursor--
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 0 && m.storyPointsCursor > 0 {
			m.storyPointsCursor--
		}
	case models.TaskTypeEpic:
		if fieldIdx == 0 && m.epicScopeCursor > 0 {
			m.epicScopeCursor--
		}
	}
}

// handleAttrRight handles right arrow key for attribute fields.
func (m *TaskEditorModel) handleAttrRight(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0:
			if m.bugReproducibleCursor < len(bugReproducibleOptions)-1 {
				m.bugReproducibleCursor++
			}
		case 1:
			if m.bugSeverityCursor < len(bugSeverityOptions)-1 {
				m.bugSeverityCursor++
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0:
			if m.ideaCategoryCursor < len(ideaCategoryOptions)-1 {
				m.ideaCategoryCursor++
			}
		case 1:
			if m.ideaEffortCursor < len(effortOptions)-1 {
				m.ideaEffortCursor++
			}
		case 2:
			if m.ideaValueCursor < len(ideaValueOptions)-1 {
				m.ideaValueCursor++
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0:
			if m.taskEffortCursor < len(effortOptions)-1 {
				m.taskEffortCursor++
			}
		case 1:
			if m.taskWorkTypeCursor < len(workTypeOptions)-1 {
				m.taskWorkTypeCursor++
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 0 && m.storyPointsCursor < len(storyPointsOptions)-1 {
			m.storyPointsCursor++
		}
	case models.TaskTypeEpic:
		if fieldIdx == 0 && m.epicScopeCursor < len(epicScopeOptions)-1 {
			m.epicScopeCursor++
		}
	}
}

// handleAttrEnter handles enter key for text attribute fields.
func (m *TaskEditorModel) handleAttrEnter(fieldIdx int) func() interface{} {
	switch m.taskType {
	case models.TaskTypeBug:
		if fieldIdx == 2 {
			return func() interface{} {
				return ShowTextEditorMsg{
					FieldName:    "bugWorkaround",
					CurrentValue: m.bugWorkaround,
				}
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 1 {
			return func() interface{} {
				return ShowTextEditorMsg{
					FieldName:    "storyPersona",
					CurrentValue: m.storyPersona,
				}
			}
		}
	case models.TaskTypeEpic:
		if fieldIdx == 1 {
			return func() interface{} {
				return ShowTextEditorMsg{
					FieldName:    "epicMilestone",
					CurrentValue: m.epicMilestone,
				}
			}
		}
		if fieldIdx == 2 {
			return func() interface{} {
				return ShowTextEditorMsg{
					FieldName:    "epicSuccessMetrics",
					CurrentValue: m.epicSuccessMetrics,
				}
			}
		}
	}
	return nil
}

// buildTaskAttrs creates the appropriate attrs struct from form state.
func (m *TaskEditorModel) buildTaskAttrs() (bugAttrs *models.BugAttrs, ideaAttrs *models.IdeaAttrs, taskAttrs *models.TaskAttrs, storyAttrs *models.StoryAttrs, epicAttrs *models.EpicAttrs) {
	switch m.taskType {
	case models.TaskTypeBug:
		attrs := &models.BugAttrs{}
		if m.bugReproducibleCursor > 0 {
			attrs.Reproducible = models.ValidBugReproducible[m.bugReproducibleCursor-1]
		}
		if m.bugSeverityCursor > 0 {
			attrs.Severity = models.ValidBugSeverity[m.bugSeverityCursor-1]
		}
		attrs.Workaround = m.bugWorkaround
		if !attrs.IsEmpty() {
			bugAttrs = attrs
		}
	case models.TaskTypeIdea:
		attrs := &models.IdeaAttrs{}
		if m.ideaCategoryCursor > 0 {
			attrs.Category = models.ValidIdeaCategory[m.ideaCategoryCursor-1]
		}
		if m.ideaEffortCursor > 0 {
			attrs.Effort = models.ValidEffortEstimate[m.ideaEffortCursor-1]
		}
		if m.ideaValueCursor > 0 {
			attrs.Value = models.ValidIdeaValue[m.ideaValueCursor-1]
		}
		attrs.Validated = m.ideaValidated
		if !attrs.IsEmpty() {
			ideaAttrs = attrs
		}
	case models.TaskTypeTask:
		attrs := &models.TaskAttrs{}
		if m.taskEffortCursor > 0 {
			attrs.Effort = models.ValidEffortEstimate[m.taskEffortCursor-1]
		}
		if m.taskWorkTypeCursor > 0 {
			attrs.WorkType = models.ValidWorkType[m.taskWorkTypeCursor-1]
		}
		if !attrs.IsEmpty() {
			taskAttrs = attrs
		}
	case models.TaskTypeStory:
		attrs := &models.StoryAttrs{}
		if m.storyPointsCursor > 0 {
			attrs.StoryPoints = storyPointsValues[m.storyPointsCursor]
		}
		attrs.Persona = m.storyPersona
		if !attrs.IsEmpty() {
			storyAttrs = attrs
		}
	case models.TaskTypeEpic:
		attrs := &models.EpicAttrs{}
		if m.epicScopeCursor > 0 {
			attrs.Scope = models.ValidEpicScope[m.epicScopeCursor-1]
		}
		attrs.Milestone = m.epicMilestone
		attrs.SuccessMetrics = m.epicSuccessMetrics
		if !attrs.IsEmpty() {
			epicAttrs = attrs
		}
	}
	return
}

// saveTask saves the current task changes to the project.
func (m *TaskEditorModel) saveTask() error {
	// Replace newlines with spaces since titles should be single logical lines
	title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("task title cannot be empty")
	}

	// Parse due date before saving
	if err := m.parseDueDate(); err != nil {
		return err
	}

	// Build type-specific attributes
	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildTaskAttrs()

	// Update the task in the project via MutateTask (auto-sets UpdatedAt)
	m.project.MutateTask(m.task.ID, func(t *models.Task) {
		t.Title = title
		t.Type = m.taskType
		t.Priority = m.priority
		t.ParentID = m.parentID
		if models.IsGTDSection(m.section) || m.section == "" {
			t.Section = m.section
		} else {
			t.CustomSection = m.section
		}
		t.DueDate = m.dueDate
		t.Tags = append([]string{}, m.tags...)
		t.Active = m.active
		t.Recurrence = m.recurrence
		t.URL = strings.TrimSpace(m.urlInput)
		t.Source = strings.TrimSpace(m.sourceInput)

		// Update type-specific attributes (clear attrs for other types)
		t.BugAttrs = bugAttrs
		t.IdeaAttrs = ideaAttrs
		t.TaskAttrs = taskAttrs
		t.StoryAttrs = storyAttrs
		t.EpicAttrs = epicAttrs

		// Handle pinned status change
		if m.pinned != t.Pinned {
			t.Pinned = m.pinned
			if m.pinned {
				now := time.Now()
				t.PinnedAt = &now
			} else {
				t.PinnedAt = nil
			}
		}
	})

	// Rebuild task tree
	m.project.BuildTaskTree()

	// Save project with backup for undo support
	if err := saveProjectSync(m.project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	return nil
}
