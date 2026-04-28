package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// View renders the context picker.
func (m *ContextPickerModel) View() tea.View {
	return tea.NewView(m.renderContextPicker(m.renderCurrentFilterHeader()))
}

func (m *ContextPickerModel) renderCurrentFilterHeader() string {
	switch {
	case m.currentCustomFilter != "":
		return MutedStyle.Render(fmt.Sprintf("Current: + %s", m.currentCustomFilter))
	case m.currentFilter != "":
		return MutedStyle.Render(fmt.Sprintf("Current: %s", m.currentFilter))
	default:
		return MutedStyle.Render("Current: All Contexts")
	}
}

// renderContextPicker renders the context picker with custom active highlighting.
func (m *ContextPickerModel) renderContextPicker(headerContent string) string {
	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Filter by Context", m.renderSubtitle()))
	b.WriteString("\n\n")
	b.WriteString(m.RenderSearchBar())
	b.WriteString(headerContent)
	b.WriteString("\n\n")
	b.WriteString(m.renderList())
	b.WriteString("\n")
	b.WriteString(m.renderStatusArea())
	b.WriteString(m.renderFooter())
	return b.String()
}

func (m *ContextPickerModel) renderSubtitle() string {
	customCount := 0
	projectContextCount := 0
	for _, ctx := range m.Items {
		if ctx.Name == "" {
			continue
		}
		if ctx.IsCustom {
			customCount++
		} else {
			projectContextCount++
		}
	}

	switch {
	case customCount > 0 && projectContextCount > 0:
		return fmt.Sprintf("%d custom, %d project contexts", customCount, projectContextCount)
	case customCount > 0:
		return fmt.Sprintf("%d custom contexts", customCount)
	case projectContextCount > 0:
		return fmt.Sprintf("%d contexts", projectContextCount)
	default:
		return "No contexts defined"
	}
}

func (m *ContextPickerModel) renderList() string {
	if len(m.Filtered) == 0 {
		return MutedStyle.Render(m.Config.NoMatchMessage) + "\n"
	}

	var b strings.Builder
	start, end, maxVisible := m.visibleWindow()
	for index := start; index < end; index++ {
		b.WriteString(m.renderContextRow(index))
		b.WriteString("\n")
	}
	if len(m.Filtered) > maxVisible {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", len(m.Filtered)-maxVisible)))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *ContextPickerModel) visibleWindow() (int, int, int) {
	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 10
	}
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.Cursor >= maxVisible {
		start = m.Cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.Filtered))
	return start, end, maxVisible
}

func (m *ContextPickerModel) renderContextRow(index int) string {
	ctx := m.Filtered[index]
	line := fmt.Sprintf("  %s ", m.displayName(ctx))
	count := TagManagerCountStyle.Render(fmt.Sprintf("(%d)", ctx.ProjectCount))

	switch {
	case index == m.Cursor:
		return TagManagerItemSelectedStyle.Render(line) + count
	case m.isActiveContext(ctx):
		return InfoStatusStyle.Render(line) + count
	default:
		return TagManagerItemStyle.Render(line) + count
	}
}

func (m *ContextPickerModel) displayName(ctx ContextInfo) string {
	if ctx.IsCustom {
		return "+ " + ctx.DisplayText()
	}
	return ctx.DisplayText()
}

func (m *ContextPickerModel) isActiveContext(ctx ContextInfo) bool {
	if ctx.IsCustom {
		return ctx.Name == m.currentCustomFilter
	}
	return ctx.Name == m.currentFilter && m.currentCustomFilter == ""
}

func (m *ContextPickerModel) renderStatusArea() string {
	if m.renameMode {
		return "Rename to: " + m.renameInput.View() + "\n" + MutedStyle.Render("enter save · esc cancel")
	}
	if m.statusMessage != "" {
		return MutedStyle.Render(m.statusMessage)
	}
	return ""
}

func (m *ContextPickerModel) renderFooter() string {
	footerLeft := ""
	if !m.renameMode && !m.confirm.IsActive() {
		footerLeft = HelpStyle.Render("n:new · e:edit · d:del · r:rename · enter · esc")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.Filtered)))
	return renderStatusBarFooter(footerLeft, right, m.Width())
}