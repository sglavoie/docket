package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

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
		switch m.activeTab {
		case formTabBasics:
			b.WriteString(m.renderBasicsTab())
		case formTabSchedule:
			b.WriteString(m.renderScheduleTab())
		case formTabDetails:
			b.WriteString(m.renderDetailsTab())
		case formTabChildren:
			b.WriteString(m.renderChildrenTab())
		}
	}

	// ── Error banner ──────────────────────────────────────────────────────────
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	helpText := "[]: tabs · Tab: next · Enter: open · Ctrl+S: save · Esc: cancel"
	if m.activeTab == formTabChildren {
		helpText = "[]: tabs · c: checklist · S: subtasks · Ctrl+A: add item · Ctrl+T: add subtask · Ctrl+D: delete · Ctrl+J/K: reorder · Ctrl+S: save · Esc: cancel"
	}
	var out strings.Builder
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(MutedStyle.Render(strings.Repeat("─", m.width)))
	out.WriteString("\n")
	out.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.width)))

	return tea.NewView(out.String())
}
