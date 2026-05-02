package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
)

func (m *AskViewModel) renderResponse() string {
	if m.response == "" {
		return ""
	}

	var b strings.Builder

	rendered := m.response
	if m.mdRenderer != nil {
		if out, err := m.mdRenderer.Render(m.response); err == nil {
			rendered = strings.TrimSpace(out)
		}
	}
	b.WriteString(rendered)

	if m.state == askStateResult && len(m.commands) > 0 {
		b.WriteString("\n\n")
		cmdStyle := InfoStatusStyle
		b.WriteString(cmdStyle.Render(fmt.Sprintf("── %d command(s) extracted ──", len(m.commands))))
		b.WriteString("\n")
		for i, c := range m.commands {
			prefix := fmt.Sprintf("  %d. ", i+1)
			wrapWidth := max(m.viewport.Width()-2, 40)
			indent := strings.Repeat(" ", len(prefix))
			line := prefix + c
			if len(line) > wrapWidth {
				b.WriteString(prefix)
				remaining := c
				first := true
				for len(remaining) > 0 {
					avail := wrapWidth - len(prefix)
					if !first {
						avail = wrapWidth - len(indent)
					}
					if avail <= 0 {
						avail = 20
					}
					chunk := remaining
					if len(chunk) > avail {
						// Break at last space within avail
						bp := avail
						for j := avail; j > 0; j-- {
							if chunk[j-1] == ' ' {
								bp = j - 1
								break
							}
						}
						chunk = remaining[:bp]
						remaining = strings.TrimLeft(remaining[bp:], " ")
					} else {
						remaining = ""
					}
					if first {
						b.WriteString(chunk)
						first = false
					} else {
						b.WriteString(indent)
						b.WriteString(chunk)
					}
					b.WriteString("\n")
				}
			} else {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press 'a' to apply commands"))
	}

	return b.String()
}

// View implements tea.Model.
func (m *AskViewModel) View() tea.View {
	pad := " "
	var b strings.Builder

	subtitle := ""
	if !external.ClaudeAvailable() {
		subtitle = ErrorStatusStyle.Render("claude CLI not found")
	}
	b.WriteString(renderFullscreenTitle("AI Assistant", subtitle))
	b.WriteString("\n\n")

	var footerLeft, footerRight string

	switch m.state {
	case askStateInput:
		b.WriteString(pad)
		applyThemedTextarea(&m.textarea)
		b.WriteString(m.textarea.View())
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("Ctrl+S: submit")
		footerRight = HelpStyle.Render("Esc: back")

	case askStateLoading:
		b.WriteString(pad)
		b.WriteString(m.spinner.View())
		b.WriteString(" Waiting for response...")
		b.WriteString("\n")
		if m.response != "" {
			b.WriteString("\n")
			b.WriteString(pad)
			b.WriteString(m.viewport.View())
		}
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("Esc: cancel")

	case askStateResult:
		if m.ready {
			b.WriteString(pad)
			b.WriteString(m.viewport.View())
		}
		b.WriteString("\n")
		footer := "n: new query · Esc: close"
		if len(m.commands) > 0 {
			footer = "a: apply commands · " + footer
		}
		footerLeft = HelpStyle.Render(footer)
		if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
			footerRight = HelpStyle.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))
		}
	}

	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, m.width))

	return tea.NewView(b.String())
}
