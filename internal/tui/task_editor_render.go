package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// renderFormLabel renders a form field label with fixed width alignment.
func (m *TaskEditorModel) renderFormLabel(label string, focusIdx int) string {
	return renderFormLabelShared(label, focusIdx, m.focusIndex)
}

// renderAttributesSection renders the type-specific attributes section.
func (m *TaskEditorModel) renderAttributesSection() string {
	var b strings.Builder

	attrsStart := m.attrsStartIndex()

	switch m.taskType {
	case models.TaskTypeBug:
		// Reproducible
		b.WriteString(renderFormLabelShared("Repro", attrsStart, m.focusIndex))
		b.WriteString(renderBugReproducibleSelector(m.bugReproducibleCursor, m.focusIndex == attrsStart))
		b.WriteString("\n")

		// Severity
		b.WriteString(renderFormLabelShared("Severity", attrsStart+1, m.focusIndex))
		b.WriteString(renderBugSeveritySelector(m.bugSeverityCursor, m.focusIndex == attrsStart+1))
		b.WriteString("\n")

		// Workaround
		b.WriteString(renderFormLabelShared("Workaround", attrsStart+2, m.focusIndex))
		b.WriteString(renderTextAttrPreview(m.bugWorkaround, m.focusIndex == attrsStart+2))
		b.WriteString("\n")

	case models.TaskTypeIdea:
		// Category
		b.WriteString(renderFormLabelShared("Category", attrsStart, m.focusIndex))
		b.WriteString(renderIdeaCategorySelector(m.ideaCategoryCursor, m.focusIndex == attrsStart))
		b.WriteString("\n")

		// Effort
		b.WriteString(renderFormLabelShared("Effort", attrsStart+1, m.focusIndex))
		b.WriteString(renderEffortSelector(m.ideaEffortCursor, m.focusIndex == attrsStart+1))
		b.WriteString("\n")

		// Value
		b.WriteString(renderFormLabelShared("Value", attrsStart+2, m.focusIndex))
		b.WriteString(renderIdeaValueSelector(m.ideaValueCursor, m.focusIndex == attrsStart+2))
		b.WriteString("\n")

		// Validated
		b.WriteString(renderFormLabelShared("Validated", attrsStart+3, m.focusIndex))
		b.WriteString(renderValidatedToggle(m.ideaValidated, m.focusIndex == attrsStart+3))
		b.WriteString("\n")

	case models.TaskTypeTask:
		// Effort
		b.WriteString(renderFormLabelShared("Effort", attrsStart, m.focusIndex))
		b.WriteString(renderEffortSelector(m.taskEffortCursor, m.focusIndex == attrsStart))
		b.WriteString("\n")

		// WorkType
		b.WriteString(renderFormLabelShared("Work Type", attrsStart+1, m.focusIndex))
		b.WriteString(renderWorkTypeSelector(m.taskWorkTypeCursor, m.focusIndex == attrsStart+1))
		b.WriteString("\n")

	case models.TaskTypeStory:
		// Story Points
		b.WriteString(renderFormLabelShared("Points", attrsStart, m.focusIndex))
		b.WriteString(renderStoryPointsSelector(m.storyPointsCursor, m.focusIndex == attrsStart))
		b.WriteString("\n")

		// Persona
		b.WriteString(renderFormLabelShared("Persona", attrsStart+1, m.focusIndex))
		b.WriteString(renderTextAttrPreview(m.storyPersona, m.focusIndex == attrsStart+1))
		b.WriteString("\n")

	case models.TaskTypeEpic:
		// Scope
		b.WriteString(renderFormLabelShared("Scope", attrsStart, m.focusIndex))
		b.WriteString(renderEpicScopeSelector(m.epicScopeCursor, m.focusIndex == attrsStart))
		b.WriteString("\n")

		// Milestone
		b.WriteString(renderFormLabelShared("Milestone", attrsStart+1, m.focusIndex))
		b.WriteString(renderTextAttrPreview(m.epicMilestone, m.focusIndex == attrsStart+1))
		b.WriteString("\n")

		// Success Metrics
		b.WriteString(renderFormLabelShared("Metrics", attrsStart+2, m.focusIndex))
		b.WriteString(renderTextAttrPreview(m.epicSuccessMetrics, m.focusIndex == attrsStart+2))
		b.WriteString("\n")
	}

	return b.String()
}

// renderTypeSelector renders the task type selector options.
func (m *TaskEditorModel) renderTypeSelector() string {
	return renderTypeSelectorShared(m.types, m.typeCursor, m.focusIndex == 1)
}

// renderParentSelector renders the parent task selector options.
func (m *TaskEditorModel) renderParentSelector() string {
	return renderParentSelectorShared(m.validParents, m.parentCursor, m.focusIndex == 2)
}

// renderPrioritySelector renders the priority selector options.
func (m *TaskEditorModel) renderPrioritySelector() string {
	return renderPrioritySelectorShared(m.priority, m.focusIndex == 3)
}

// renderSectionSelector renders the section selector options.
func (m *TaskEditorModel) renderSectionSelector() string {
	return renderSectionSelectorShared(m.sections, m.sectionCursor, m.focusIndex == 4)
}

// renderDueDateField renders the due date input field.
func (m *TaskEditorModel) renderDueDateField() string {
	return renderDueDateFieldShared(m.dueDateInput, m.focusIndex == 5)
}

// renderFlagsField renders the mutually exclusive flag options.
func (m *TaskEditorModel) renderFlagsField() string {
	return renderFlagsSelectorShared(m.flagCursor, m.focusIndex == 6)
}

// renderRecurrenceSelector renders the recurrence pattern selector options.
func (m *TaskEditorModel) renderRecurrenceSelector() string {
	return renderRecurrenceSelectorShared(m.recurrenceOpts, m.recurrenceCursor, m.focusIndex == 7, m.recurrence)
}

// renderURLField renders the URL text input field.
func (m *TaskEditorModel) renderURLField() string {
	isFocused := m.focusIndex == 8
	value := m.urlInput
	if value == "" {
		if isFocused {
			return FieldLabelFocusedStyle.Render("_")
		}
		return MutedStyle.Render("(none)")
	}
	if isFocused {
		return FieldLabelFocusedStyle.Render(value + "_")
	}
	return value
}

// renderSourceField renders the Source text input field.
func (m *TaskEditorModel) renderSourceField() string {
	isFocused := m.focusIndex == 9
	value := m.sourceInput
	if value == "" {
		if isFocused {
			return FieldLabelFocusedStyle.Render("_")
		}
		return MutedStyle.Render("(none)")
	}
	if isFocused {
		return FieldLabelFocusedStyle.Render(value + "_")
	}
	return value
}

// renderScoreSummary renders the composite score and per-dimension breakdown
// when a scoring profile is active for the project. Returns empty string if no profile.
func (m *TaskEditorModel) renderScoreSummary() string {
	if m.config == nil {
		return ""
	}
	profile := m.config.GetProjectScoringProfile(m.project.ID)
	if profile == nil {
		return ""
	}

	var b strings.Builder
	totalScore := scoring.ComputeScore(m.task, profile)
	results := scoring.ScoreBreakdown(m.task, profile)

	pct := scoring.ScorePercent(totalScore, profile)
	b.WriteString(renderFormLabelShared("Score", -1, m.focusIndex))
	fmt.Fprintf(&b, "%d/%d", int(pct), scoring.MaxNormalizedScore(profile))
	b.WriteString("  ")

	// Compact dimension summary
	var dimParts []string
	for _, r := range results {
		dimParts = append(dimParts, fmt.Sprintf("%s=%d", r.DimensionName, r.Value))
	}
	b.WriteString(MutedStyle.Render("(" + strings.Join(dimParts, ", ") + ")"))
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("S to edit"))
	b.WriteString("\n")

	return b.String()
}
