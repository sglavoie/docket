package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderDetailView renders a modal showing the full content of the selected diff line.
// For modifications (remove followed by add), shows both lines together.
func (m *UndoHistoryModel) renderDetailView() string {
	var b strings.Builder
	wrapWidth := max(m.Width()-4, 20)

	pairedLine, isModification := m.detailModificationPair()
	if isModification && pairedLine != nil {
		b.WriteString(TitleStyle.Render("Modified Line (Full Content)"))
		b.WriteString("\n\n")

		oldContent, newContent := m.modificationContents(*pairedLine)
		b.WriteString(DiffRemoveStyle.Render("[-] OLD:"))
		b.WriteString("\n")
		for _, line := range wrapText(oldContent, wrapWidth) {
			b.WriteString("  ")
			b.WriteString(DiffRemoveStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		b.WriteString(DiffAddStyle.Render("[+] NEW:"))
		b.WriteString("\n")
		for _, line := range wrapText(newContent, wrapWidth) {
			b.WriteString("  ")
			b.WriteString(DiffAddStyle.Render(line))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(TitleStyle.Render(m.detailTitle()))
		b.WriteString("\n\n")

		kindStyle, kindLabel := m.detailKindStyle()
		b.WriteString(kindStyle.Render(kindLabel))
		b.WriteString("\n\n")

		for _, line := range wrapText(m.detailLine.Content, wrapWidth) {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press Esc/Enter to close"))
	return b.String()
}

func (m *UndoHistoryModel) detailModificationPair() (*DiffLine, bool) {
	if m.detailLine.Kind == DiffRemove && m.diffCursor+1 < len(m.diffLines) {
		next := m.diffLines[m.diffCursor+1]
		if next.Kind == DiffAdd {
			return &next, true
		}
	}

	if m.detailLine.Kind == DiffAdd && m.diffCursor > 0 {
		prev := m.diffLines[m.diffCursor-1]
		if prev.Kind == DiffRemove {
			return &prev, true
		}
	}

	return nil, false
}

func (m *UndoHistoryModel) modificationContents(pairedLine DiffLine) (string, string) {
	if m.detailLine.Kind == DiffRemove {
		return m.detailLine.Content, pairedLine.Content
	}

	return pairedLine.Content, m.detailLine.Content
}

func (m *UndoHistoryModel) detailTitle() string {
	switch m.detailLine.Kind {
	case DiffAdd:
		return "Added Line (Full Content)"
	case DiffRemove:
		return "Removed Line (Full Content)"
	case DiffContext:
		return "Context Line (Full Content)"
	default:
		return "Line Detail"
	}
}

func (m *UndoHistoryModel) detailKindStyle() (lipgloss.Style, string) {
	switch m.detailLine.Kind {
	case DiffAdd:
		return DiffAddStyle, "[+] ADDED"
	case DiffRemove:
		return DiffRemoveStyle, "[-] REMOVED"
	default:
		return DiffContextStyle, "[ ] CONTEXT"
	}
}
