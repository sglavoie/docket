package tui

import "strings"

func (m *SettingsModel) renderGeneralTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22
	base := tabFieldStart[tabGeneral]

	b.WriteString(renderFormLabelWithWidth("Editor: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.editorInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Command used to open tasks for editing", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Notes Editor: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.notesEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+1 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for opening notes with Ctrl+O (empty = code)", indent, m.focusIndex == base+1))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Default Context: ", labelWidth, m.focusIndex == base+2))
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	if len(m.contexts) > 0 {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(MutedStyle.Render("Available: "))
		for i, ctx := range m.contexts {
			if i == m.contextCursor {
				b.WriteString(BoldPrimaryStyle.Render("[" + ctx + "]"))
			} else {
				b.WriteString(MutedStyle.Render(ctx))
			}
			if i < len(m.contexts)-1 {
				b.WriteString(MutedStyle.Render(", "))
			}
		}
		if m.focusIndex == base+2 {
			b.WriteString(HelpStyle.Render("  (←/→ to select, or type custom)"))
		}
		b.WriteString("\n")
	}

	if m.focusIndex == base+2 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Default context for new projects", indent, m.focusIndex == base+2))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Undo History Depth: ", labelWidth, m.focusIndex == base+3))
	b.WriteString(m.undoDepthInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, max 1000)"))
	b.WriteString("\n")

	if m.focusIndex == base+3 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Number of undo steps to keep in history", indent, m.focusIndex == base+3))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Default View: ", labelWidth, m.focusIndex == base+4))
	b.WriteString(renderEnumSelector(defaultProjectViewOptions, m.defaultViewCursor, m.focusIndex == base+4, m.width-indent))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("View shown when opening a project", indent, m.focusIndex == base+4))

	return b.String()
}

func (m *SettingsModel) renderReviewTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22
	base := tabFieldStart[tabReview]

	b.WriteString(SubtitleStyle.Render("Review View Thresholds"))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = hide section)"))
	b.WriteString("\n\n")

	b.WriteString(renderFormLabelWithWidth("Due Soon (days): ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.dueSoonInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks due within this many days", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Future (days): ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.futureInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks due further out than this", indent, m.focusIndex == base+1))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Old (days): ", labelWidth, m.focusIndex == base+2))
	b.WriteString(m.oldDaysInput.View())
	b.WriteString("       ")
	b.WriteString(MutedStyle.Render("(based on last update)"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Tasks not updated in this many days", indent, m.focusIndex == base+2))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Completed (hours): ", labelWidth, m.focusIndex == base+3))
	b.WriteString(m.completedHoursInput.View())
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show recently completed within this window", indent, m.focusIndex == base+3))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Stale badge (days): ", labelWidth, m.focusIndex == base+4))
	b.WriteString(m.staleTaskDaysInput.View())
	b.WriteString("       ")
	b.WriteString(MutedStyle.Render("(0 = disabled)"))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show inline stale badge on tasks not updated in this many days", indent, m.focusIndex == base+4))

	if m.focusIndex >= base && m.focusIndex <= base+4 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *SettingsModel) renderAttachmentsTab() string {
	var b strings.Builder
	const labelWidth = 20
	const indent = 22
	base := tabFieldStart[tabAttachments]

	b.WriteString(SubtitleStyle.Render("Editor Configuration"))
	b.WriteString("\n\n")

	b.WriteString(renderFormLabelWithWidth("Attachment Editor: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(m.attachmentEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+0 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for general attachments (empty = OS default)", indent, m.focusIndex == base+0))

	b.WriteString("\n")

	b.WriteString(renderFormLabelWithWidth("Code Editor: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(m.codeEditorInput.View())
	b.WriteString("\n")
	if m.focusIndex == base+1 && m.validationErr != "" {
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}
	b.WriteString(renderFormFieldDesc("Editor for code references (empty = code)", indent, m.focusIndex == base+1))

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Code References"))
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("  Code references use the file:line format (e.g., src/main.go:42)."))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  They open in your code editor at the specified line."))
	b.WriteString("\n")

	return b.String()
}

func (m *SettingsModel) renderProjectTreeTab() string {
	var b strings.Builder
	const labelWidth = 18
	b.WriteString(SubtitleStyle.Render("Default Column Visibility"))
	b.WriteString("\n\n")
	m.renderColumnToggles(&b, m.treeColumnDefaults, tabFieldStart[tabProjectTree], labelWidth, m.treePriorityMode)
	return b.String()
}

func (m *SettingsModel) renderOutlinerTab() string {
	switch m.subTab {
	case 1:
		return m.renderOutlinerSectionStateSection()
	case 2:
		return m.renderOutlinerColumnVisibilitySection()
	default:
		return m.renderOutlinerSidebarSection()
	}
}

func (m *SettingsModel) renderOutlinerSidebarSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24
	base := subTabFieldStart(tabOutliner, 0)

	b.WriteString(renderFormLabelWithWidth("Cross-Project Sidebar: ", labelWidth, m.focusIndex == base+0))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.crossOutlinerSidebar, m.focusIndex == base+0))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Show sidebar by default in cross-project outliner", indent, m.focusIndex == base+0))

	b.WriteString(renderFormLabelWithWidth("Cycle Projects Only: ", labelWidth, m.focusIndex == base+1))
	b.WriteString(renderFormToggle("Enabled", "Disabled", m.crossOutlinerCycleOnly, m.focusIndex == base+1))
	b.WriteString("\n")
	b.WriteString(renderFormFieldDesc("Only show projects in the current cycle (context-aware)", indent, m.focusIndex == base+1))

	return b.String()
}

func (m *SettingsModel) renderOutlinerSectionStateSection() string {
	var b strings.Builder
	const labelWidth = 22
	const indent = 24
	base := subTabFieldStart(tabOutliner, 1)

	for i, label := range outlinerSectionLabels {
		focusIdx := base + i
		b.WriteString(renderFormLabelWithWidth(label+": ", labelWidth, m.focusIndex == focusIdx))
		b.WriteString(renderFormToggle("Collapsed", "Expanded", m.outlinerCollapsed[i], m.focusIndex == focusIdx))
		b.WriteString("\n")
		if i == 0 {
			b.WriteString(renderFormFieldDesc("Tasks with no explicit section (in-progress)", indent, m.focusIndex == focusIdx))
		}
	}

	return b.String()
}

func (m *SettingsModel) renderOutlinerColumnVisibilitySection() string {
	var b strings.Builder
	const labelWidth = 22

	m.renderColumnToggles(&b, m.outlinerColumnDefaults, subTabFieldStart(tabOutliner, 2), labelWidth, m.outlinerPriorityMode)

	return b.String()
}

func (m *SettingsModel) renderStagesTab() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Workflow Stages"))
	b.WriteString("\n\n")

	if len(m.stages) == 0 {
		b.WriteString(MutedStyle.Render("  No stages configured. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, stage := range m.stages {
			if i == m.stageCursor {
				b.WriteString(BoldPrimaryStyle.Render("  ▸ "))
				b.WriteString(StageBadgeStyle.Render(stage))
			} else {
				b.WriteString(MutedStyle.Render("    "))
				b.WriteString(StageBadgeStyle.Render(stage))
			}
			b.WriteString("\n")
		}
	}

	if m.stageEditMode {
		b.WriteString("\n")
		if m.stageAddMode {
			b.WriteString(BoldStyle.Render("  New stage: "))
		} else {
			b.WriteString(BoldStyle.Render("  Edit stage: "))
		}
		b.WriteString(m.stageInput.View())
		b.WriteString("\n")
		if m.stageValidationErr != "" {
			b.WriteString("  ")
			b.WriteString(ValidationErrorStyle.Render(m.stageValidationErr))
			b.WriteString("\n")
		}
	}

	if !m.stageEditMode && m.hasUnsavedStages() {
		b.WriteString("\n")
		b.WriteString(WarningMessageStyle.Render("  Unsaved changes — press Enter to save"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  ───────────────────────────────"))
	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("  a=add  e=edit  d=delete  J/K=reorder  Enter(save)/Esc(cancel)"))
	b.WriteString("\n")

	return b.String()
}
