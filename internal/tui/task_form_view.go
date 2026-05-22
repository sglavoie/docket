package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// helpPaneMinWidth is the minimum terminal width at which the help pane is shown.
// Below this threshold the form uses a single-column layout.
const helpPaneMinWidth = 60

func (m *TaskFormModel) View() tea.View {
	var b strings.Builder

	// ── Title ────────────────────────────────────────────────────────────────
	titleText := "Create Task"
	subtitle := ""
	if m.mode == FormModeEdit {
		titleText = "Edit Task"
		if m.task != nil {
			marker := ""
			if m.hasChanges {
				marker = "*"
			}
			subtitle = fmt.Sprintf("#%d%s", m.task.ShortID, marker)
		}
	}
	b.WriteString(renderFullscreenTitle(titleText, subtitle))
	b.WriteString("\n")

	// ── Tab bar ───────────────────────────────────────────────────────────────
	b.WriteString(RenderTabBar(formTabNames, m.activeTab, m.width-CreatorTabBarPadding, 1))
	b.WriteString("\n\n")

	// ── Per-tab content (or inline overlay) ───────────────────────────────────
	if m.activeOverlay != formOverlayNone {
		b.WriteString(m.renderFormOverlay())
		b.WriteString("\n")
	} else {
		var tabContent string
		switch m.activeTab {
		case formTabBasics:
			tabContent = m.renderBasicsTab()
		case formTabSchedule:
			tabContent = m.renderScheduleTab()
		case formTabDetails:
			tabContent = m.renderDetailsTab()
		case formTabChildren:
			tabContent = m.renderChildrenTab()
		}

		if m.width >= helpPaneMinWidth {
			// Two-column: form content left, help pane right.
			helpWidth := max(min(m.width*35/100, 30), 20)
			helpPane := m.renderFormHelpPane(helpWidth)
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabContent, "  ", helpPane))
		} else {
			b.WriteString(tabContent)
		}
		b.WriteString("\n")
	}

	// ── Error banner ──────────────────────────────────────────────────────────
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	helpText := "[]: tabs · Tab: next · Ctrl+S: save · Esc: cancel"
	if m.activeTab == formTabChildren {
		showSubtasks := m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType)
		helpText = "[]: tabs · Tab: next · c: checklist"
		if showSubtasks {
			helpText += " · S: subtasks"
		}
		helpText += " · Ctrl+A: add item"
		if showSubtasks {
			helpText += " · Ctrl+T: add subtask"
		}
		helpText += " · Ctrl+D: delete · Ctrl+J/K: reorder · Ctrl+S: save · Esc: cancel"
	}
	var out strings.Builder
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(MutedStyle.Render(strings.Repeat("─", m.width)))
	out.WriteString("\n")
	out.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.width)))

	return tea.NewView(out.String())
}
