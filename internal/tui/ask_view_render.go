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

	return b.String()
}

// View implements tea.Model.
func (m *AskViewModel) View() tea.View {
	pad := " "
	var b strings.Builder

	subtitle := ""
	if !external.ClaudeAvailable() {
		subtitle = ErrorStatusStyle.Render("claude CLI not found")
	} else {
		subtitle = InfoStatusStyle.Render("context: " + string(m.contextMode))
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
		footerLeft = HelpStyle.Render("Ctrl+S: submit · Ctrl+T: context")
		footerRight = HelpStyle.Render("Esc: back")

	case askStateConsent:
		b.WriteString(pad)
		b.WriteString("tt can send the current project name and task content to the configured Claude CLI provider.\n\n")
		b.WriteString(pad)
		b.WriteString("Remember consent and include task context? [y/N]")
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("y: remember consent · n: continue without context")
		footerRight = HelpStyle.Render("Esc: cancel")

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
		if len(m.plan.Actions) > 0 {
			footer = "a: review/apply plan · " + footer
		}
		footerLeft = HelpStyle.Render(footer)
		if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
			footerRight = HelpStyle.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))
		}

	case askStateConfirmSafe:
		b.WriteString(pad)
		_, _ = fmt.Fprintf(&b, "Apply %d safe action(s) as one transaction? [y/N]", m.preview.SafeCount)
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("y: accept safe batch · n: decline")

	case askStateConfirmDestructive:
		b.WriteString(pad)
		b.WriteString(ErrorStatusStyle.Render(fmt.Sprintf("Destructive escalation: include %d destructive action(s)? [y/N]", m.preview.DestructiveCount)))
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("y: explicitly include · n: exclude")

	case askStateApplying:
		b.WriteString(pad)
		b.WriteString(m.spinner.View())
		b.WriteString(" Applying accepted actions as one transaction...")
		b.WriteString("\n")
	}

	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, m.width))

	return tea.NewView(b.String())
}
