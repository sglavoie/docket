package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/actionplan"
	"github.com/sglavoie/tt/internal/aiharness"
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

	subtitle := m.askSubtitle()
	title := renderFullscreenTitle("Ask AI", subtitle)
	b.WriteString(fitStyledChromeToWidth("ask title", title, m.width))
	b.WriteString("\n\n")

	var footerLeft, footerRight string

	switch m.state {
	case askStateInput:
		b.WriteString(m.renderAskInput())
		footerLeft = HelpStyle.Render("Tab: fields · ←/→: harness · Ctrl+S: submit · Ctrl+T: context")
		footerRight = HelpStyle.Render("Esc: back")

	case askStateSavingPreferences:
		b.WriteString(pad)
		b.WriteString("Saving AI harness preferences...")
		b.WriteString("\n")

	case askStateConsent:
		b.WriteString(pad)
		_, _ = fmt.Fprintf(&b, "tt can send names and task content from all active projects to the configured %s CLI provider.\n", m.harness.Label())
		b.WriteString(pad)
		b.WriteString("For a consented full-context request, Codex can inspect the active workspace directory read-only. It cannot modify tasks directly; every answer and action still uses the strict JSON action-plan review and confirmation flow.\n\n")
		b.WriteString(pad)
		b.WriteString("Remember consent and include task context? [y/N]")
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("y: remember consent · n: continue without context")
		footerRight = HelpStyle.Render("Esc: cancel")

	case askStateLoading:
		b.WriteString(pad)
		b.WriteString(m.spinner.View())
		_, _ = fmt.Fprintf(&b, " Waiting for %s...", m.harness.Label())
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
		if m.reuseAvailable {
			footer = "r: reuse request · " + footer
		}
		if m.prepared != nil && len(m.prepared.Preview().Actions) > 0 {
			footer = "a: review/apply plan · " + footer
		}
		footerLeft = HelpStyle.Render(footer)
		if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
			footerRight = HelpStyle.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))
		}

	case askStateConfirmSafe:
		preview := m.prepared.Preview()
		b.WriteString(pad)
		_, _ = fmt.Fprintf(&b, "Apply %d safe action(s) as one transaction? [y/N]", preview.SafeCount)
		b.WriteString("\n")
		footerLeft = HelpStyle.Render("y: accept safe batch · n: decline")

	case askStateConfirmDestructive:
		preview := m.prepared.Preview()
		b.WriteString(pad)
		b.WriteString(ErrorStatusStyle.Render(fmt.Sprintf("Destructive escalation: include %d destructive action(s)? [y/N]", preview.DestructiveCount)))
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

func (m *AskViewModel) askSubtitle() string {
	label := m.harness.Label()
	if label == "" {
		label = "Unknown harness"
	}
	if m.width > 0 && m.width < 18 {
		return ""
	}
	if !m.harnessIsAvailable() {
		return ErrorStatusStyle.Render(label + " unavailable")
	}
	if m.width > 0 && m.width < 36 {
		return InfoStatusStyle.Render(label)
	}
	return InfoStatusStyle.Render(label + " · context: " + string(m.contextMode))
}

func (m *AskViewModel) harnessIsAvailable() bool {
	if m.available == nil {
		return false
	}
	return m.available(m.harness)
}

func (m *AskViewModel) renderAskInput() string {
	const labelWidth = 12
	contentWidth := max(m.width-2, 1)
	fieldWidth := max(contentWidth-labelWidth-2, 1)
	var b strings.Builder

	applyThemedTextarea(&m.textarea)
	applyThemedTextInput(&m.configInput)

	queryLabel := " " + renderFormLabelWithWidth("Query", labelWidth, m.focusIndex == askFocusQuery)
	b.WriteString(fitStyledLineToWidth(queryLabel, max(m.width, 1)))
	b.WriteString("\n ")
	b.WriteString(m.textarea.View())
	b.WriteString("\n")

	options := make([]string, 0, len(aiharness.Names()))
	selected := 0
	for i, name := range aiharness.Names() {
		options = append(options, name.Label())
		if name == m.harness {
			selected = i
		}
	}
	harnessLine := " " + renderFormLabelWithWidth("Harness", labelWidth, m.focusIndex == askFocusHarness) +
		renderEnumSelector(options, selected, m.focusIndex == askFocusHarness, fieldWidth)
	b.WriteString(fitStyledLineToWidth(harnessLine, max(m.width, 1)))
	b.WriteString("\n")

	availability := fmt.Sprintf("%s CLI (%s): available", m.harness.Label(), m.harness.Executable())
	if !m.harnessIsAvailable() {
		availability = fmt.Sprintf("%s CLI (%s): not found", m.harness.Label(), m.harness.Executable())
	}
	b.WriteString(fitStyledLineToWidth("   "+InfoStatusStyle.Render(availability), max(m.width, 1)))
	b.WriteString("\n")

	configLine := " " + renderFormLabelWithWidth("Config dir", labelWidth, m.focusIndex == askFocusConfigDir) + m.configInput.View()
	b.WriteString(fitStyledLineToWidth(configLine, max(m.width, 1)))
	b.WriteString("\n")

	effective := m.askEffectiveConfigDirLine()
	b.WriteString(fitStyledLineToWidth("   "+MutedStyle.Render(effective), max(m.width, 1)))
	b.WriteString("\n")
	if m.inputError != "" {
		for _, line := range wrapStyled(ValidationErrorStyle.Render(m.inputError), contentWidth) {
			b.WriteString(" ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *AskViewModel) askEffectiveConfigDirLine() string {
	draft := m.configInput.Value()
	resolved, err := aiharness.ResolveConfigDir(m.harness, draft)
	if err != nil {
		return "Effective: invalid override — " + askErrorText(err)
	}
	source := "native default"
	switch resolved.Source {
	case aiharness.ConfigDirNativeDefault:
	case aiharness.ConfigDirMachineOverride:
		source = "machine override"
		if m.runtime == nil || m.runtime.Machine().GetAIConfigDir(m.harness) != draft {
			source = "unsaved override"
		}
	case aiharness.ConfigDirEnvironment:
		source = "environment " + m.harness.ConfigEnvVar()
	}
	path := actionplan.TerminalSafeDisplayText(resolved.Path, actionplan.DisplayTextSingleLine)
	return fmt.Sprintf("Effective: %s (%s)", path, source)
}
