package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// formBasicsLabelWidth is the fixed label column width for all Basics-tab rows.
const formBasicsLabelWidth = 11 // "Priority: " + 1 pad = 11

// renderBasicsTab renders the Basics tab: Title, Type, Priority, Parent, Section, Flags, Tags.
func (m *TaskFormModel) renderBasicsTab() string {
	var b strings.Builder
	const lw = formBasicsLabelWidth

	// 0: Title (inline textarea)
	b.WriteString(renderFormLabelWithWidth("Title:", lw, m.focusIndex == 0))
	b.WriteString("\n")
	applyThemedTextarea(&m.titleInput)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// 1: Type
	b.WriteString(renderFormLabelWithWidth("Type:", lw, m.focusIndex == 1))
	{
		icon := GetTypeBadge(string(m.draft.taskType), m.draft.taskType.Icon())
		val := fmt.Sprintf("%s %s", icon, m.draft.taskType.HeadingPrefix())
		if m.focusIndex == 1 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// 2: Priority
	b.WriteString(renderFormLabelWithWidth("Priority:", lw, m.focusIndex == 2))
	{
		icon := GetPriorityStyle(m.draft.priority).Render(PriorityIcon(m.draft.priority))
		lbl := formPriorityLabel(m.draft.priority)
		val := fmt.Sprintf("%s %s", icon, lbl)
		if m.focusIndex == 2 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// 3: Parent
	b.WriteString(renderFormLabelWithWidth("Parent:", lw, m.focusIndex == 3))
	{
		var val string
		if m.draft.parentID != nil {
			if p := formFindTask(m.project.Tasks, *m.draft.parentID); p != nil {
				val = fmt.Sprintf("#%d %s", p.ShortID, truncateToVisualWidth(p.Title, 40))
			} else {
				val = MutedStyle.Render("(not found)")
			}
		} else {
			val = MutedStyle.Render("(none)")
		}
		if m.focusIndex == 3 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// 4: Section
	b.WriteString(renderFormLabelWithWidth("Section:", lw, m.focusIndex == 4))
	{
		val := m.draft.section
		if val == "" {
			val = MutedStyle.Render("(none)")
		}
		if m.focusIndex == 4 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// 5: Flags
	b.WriteString(renderFormLabelWithWidth("Flags:", lw, m.focusIndex == 5))
	b.WriteString(renderFormFlagsValue(m.draft.pinned, m.draft.active, m.draft.dimmed, m.focusIndex == 5))
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// 6: Tags
	b.WriteString(renderFormLabelWithWidth("Tags:", lw, m.focusIndex == 6))
	{
		var val string
		if len(m.draft.tags) == 0 {
			val = MutedStyle.Render("(none)")
		} else {
			val = strings.Join(m.draft.tags, ", ")
		}
		if m.focusIndex == 6 {
			b.WriteString(SelectorOptionSelectedStyle.Render(val))
		} else {
			b.WriteString(val)
		}
	}
	b.WriteString(MutedStyle.Render("  Enter"))
	b.WriteString("\n")

	// ── Type-specific attribute rows ───────────────────────────────────────────
	if formTypeHasAttrs(m.draft.taskType) {
		b.WriteString("\n")
		b.WriteString(renderFormTypeAttrsHeader(m.draft.taskType))
		b.WriteString(m.renderTypeAttrsRows())
	}

	return b.String()
}

// renderFormFlagsValue renders the flag badges for a row value.
func renderFormFlagsValue(pinned, active, dimmed bool, focused bool) string {
	var parts []string
	if pinned {
		parts = append(parts, "Pinned")
	}
	if active {
		parts = append(parts, "Active")
	}
	if dimmed {
		parts = append(parts, "Dimmed")
	}
	if len(parts) == 0 {
		return MutedStyle.Render("(none)")
	}
	val := strings.Join(parts, "  ")
	if focused {
		return SelectorOptionSelectedStyle.Render(val)
	}
	return val
}

// renderFormTypeAttrsHeader renders the collapsed type-attrs section header placeholder.
// Full attr editing is wired in Session 4.
func renderFormTypeAttrsHeader(taskType models.TaskType) string {
	sep := MutedStyle.Render(strings.Repeat("─", 3))
	label := BoldStyle.Render(fmt.Sprintf(" %s ", strings.Title(string(taskType)))) //nolint:staticcheck
	return sep + label + MutedStyle.Render(strings.Repeat("─", 20)) + "\n"
}

// formTypeHasAttrs reports whether a type has type-specific attributes.
func formTypeHasAttrs(t models.TaskType) bool {
	switch t {
	case models.TaskTypeBug, models.TaskTypeIdea, models.TaskTypeTask, models.TaskTypeStory, models.TaskTypeEpic:
		return true
	}
	return false
}

// formPriorityLabel returns the human-readable label for a priority value.
func formPriorityLabel(p int) string {
	for _, e := range formPriorityEntries {
		if e.priority == p {
			return e.label
		}
	}
	return fmt.Sprintf("%d", p)
}

// renderTypeAttrsRows renders the type-specific attribute rows for the active type.
// Each row is at focusIndex formBasicsFieldCount+localIdx.
func (m *TaskFormModel) renderTypeAttrsRows() string {
	var b strings.Builder
	const lw = formAttrsLabelWidth

	switch m.draft.taskType {
	case models.TaskTypeBug:
		// 0: Reproducible
		b.WriteString(renderFormLabelWithWidth("Reproducible:", lw, m.attrRowFocused(0)))
		b.WriteString(renderAttrEnumValue(string(m.draft.bugAttrs.Reproducible), m.attrRowFocused(0)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 1: Severity
		b.WriteString(renderFormLabelWithWidth("Severity:", lw, m.attrRowFocused(1)))
		b.WriteString(renderAttrEnumValue(string(m.draft.bugAttrs.Severity), m.attrRowFocused(1)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 2: Workaround (free text)
		b.WriteString(renderFormLabelWithWidth("Workaround:", lw, m.attrRowFocused(2)))
		b.WriteString(renderAttrTextValue(m.draft.bugAttrs.Workaround, m.attrRowFocused(2)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")

	case models.TaskTypeIdea:
		// 0: Category
		b.WriteString(renderFormLabelWithWidth("Category:", lw, m.attrRowFocused(0)))
		b.WriteString(renderAttrEnumValue(string(m.draft.ideaAttrs.Category), m.attrRowFocused(0)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 1: Effort
		b.WriteString(renderFormLabelWithWidth("Effort:", lw, m.attrRowFocused(1)))
		b.WriteString(renderAttrEnumValue(string(m.draft.ideaAttrs.Effort), m.attrRowFocused(1)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 2: Value
		b.WriteString(renderFormLabelWithWidth("Value:", lw, m.attrRowFocused(2)))
		b.WriteString(renderAttrEnumValue(string(m.draft.ideaAttrs.Value), m.attrRowFocused(2)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 3: Validated
		b.WriteString(renderFormLabelWithWidth("Validated:", lw, m.attrRowFocused(3)))
		validatedVal := ""
		if m.draft.ideaAttrs.Validated {
			validatedVal = "validated"
		}
		b.WriteString(renderAttrEnumValue(validatedVal, m.attrRowFocused(3)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")

	case models.TaskTypeTask:
		// 0: Effort
		b.WriteString(renderFormLabelWithWidth("Effort:", lw, m.attrRowFocused(0)))
		b.WriteString(renderAttrEnumValue(string(m.draft.taskAttrs.Effort), m.attrRowFocused(0)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 1: Work Type
		b.WriteString(renderFormLabelWithWidth("Work Type:", lw, m.attrRowFocused(1)))
		b.WriteString(renderAttrEnumValue(string(m.draft.taskAttrs.WorkType), m.attrRowFocused(1)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")

	case models.TaskTypeStory:
		// 0: Story Points
		b.WriteString(renderFormLabelWithWidth("Points:", lw, m.attrRowFocused(0)))
		pointsVal := ""
		if m.draft.storyAttrs.StoryPoints > 0 {
			pointsVal = fmt.Sprintf("%d", m.draft.storyAttrs.StoryPoints)
		}
		b.WriteString(renderAttrEnumValue(pointsVal, m.attrRowFocused(0)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 1: Persona
		b.WriteString(renderFormLabelWithWidth("Persona:", lw, m.attrRowFocused(1)))
		b.WriteString(renderAttrTextValue(m.draft.storyAttrs.Persona, m.attrRowFocused(1)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")

	case models.TaskTypeEpic:
		// 0: Scope
		b.WriteString(renderFormLabelWithWidth("Scope:", lw, m.attrRowFocused(0)))
		b.WriteString(renderAttrEnumValue(string(m.draft.epicAttrs.Scope), m.attrRowFocused(0)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 1: Milestone
		b.WriteString(renderFormLabelWithWidth("Milestone:", lw, m.attrRowFocused(1)))
		b.WriteString(renderAttrTextValue(m.draft.epicAttrs.Milestone, m.attrRowFocused(1)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
		// 2: Success Metrics
		b.WriteString(renderFormLabelWithWidth("Metrics:", lw, m.attrRowFocused(2)))
		b.WriteString(renderAttrTextValue(m.draft.epicAttrs.SuccessMetrics, m.attrRowFocused(2)))
		b.WriteString(MutedStyle.Render("  Enter"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderAttrEnumValue renders an enum attr value row content (highlighted when focused).
func renderAttrEnumValue(v string, focused bool) string {
	if v == "" {
		return MutedStyle.Render("(none)")
	}
	if focused {
		return SelectorOptionSelectedStyle.Render(v)
	}
	return v
}

// renderAttrTextValue renders a free-text attr value row content (preview, highlighted when focused).
func renderAttrTextValue(v string, focused bool) string {
	if v == "" {
		return MutedStyle.Render("(none)")
	}
	preview := truncateToVisualWidth(v, 40)
	if focused {
		return SelectorOptionSelectedStyle.Render(preview)
	}
	return preview
}
