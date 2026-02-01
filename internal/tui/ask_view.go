package tui

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

type askState int

const (
	askStateInput askState = iota
	askStateLoading
	askStateResult
)

// AskViewModel implements the AI assistant view.
type AskViewModel struct {
	state    askState
	textarea textarea.Model
	viewport viewport.Model
	spinner  SpinnerModel
	project  *models.Project
	width    int
	height   int
	ready    bool
	response string
	commands []string
	cancelFn context.CancelFunc

	// Streaming state
	streamReader io.ReadCloser
	streamCmd    *exec.Cmd
}

// NewAskViewModel creates a new AI assistant view.
func NewAskViewModel(project *models.Project) *AskViewModel {
	ta := textarea.New()
	ta.Placeholder = "Ask about your tasks... (Ctrl+S to submit)"
	ta.CharLimit = 500
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(0, func(lineIdx int) string { return "" })
	ta.Focus()

	return &AskViewModel{
		state:    askStateInput,
		textarea: ta,
		spinner:  NewSpinner(),
		project:  project,
	}
}

// HasTextInput returns true when the textarea is active.
func (m *AskViewModel) HasTextInput() bool {
	return m.state == askStateInput
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *AskViewModel) ShouldCaptureEsc() bool {
	return m.state == askStateInput || m.state == askStateLoading
}

// Init implements tea.Model.
func (m *AskViewModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (m *AskViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case AskStreamChunkMsg:
		m.response += msg.Text
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, m.readNextChunk()

	case AskStreamDoneMsg:
		m.state = askStateResult
		m.streamReader = nil
		m.streamCmd = nil
		m.cancelFn = nil
		if msg.Err != nil {
			m.response += fmt.Sprintf("\n\nError: %v", msg.Err)
		}
		m.commands = external.ExtractTTCommands(m.response)
		m.viewport.SetContent(m.renderResponse())
		return m, nil

	case AskCommandResultMsg:
		status := "ok"
		if msg.Err != nil {
			status = fmt.Sprintf("error: %v", msg.Err)
		}
		m.response += fmt.Sprintf("\n\n> %s\n%s", msg.Command, status)
		if msg.Output != "" {
			m.response += "\n" + msg.Output
		}
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.state == askStateLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *AskViewModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case askStateInput:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }
		case "ctrl+s":
			query := strings.TrimSpace(m.textarea.Value())
			if query == "" {
				return m, nil
			}
			return m, m.submitQuery(query)
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

	case askStateLoading:
		if msg.String() == "esc" {
			if m.cancelFn != nil {
				m.cancelFn()
			}
			m.state = askStateInput
			m.streamReader = nil
			m.streamCmd = nil
			m.textarea.Focus()
			return m, textarea.Blink
		}

	case askStateResult:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return BackMsg{} }
		case "a":
			if len(m.commands) > 0 {
				return m, m.applyCommands()
			}
		case "n":
			m.state = askStateInput
			m.textarea.Reset()
			m.textarea.Focus()
			m.response = ""
			m.commands = nil
			return m, textarea.Blink
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *AskViewModel) submitQuery(query string) tea.Cmd {
	m.state = askStateLoading
	m.response = ""
	m.commands = nil
	m.textarea.Blur()

	var systemPrompt string
	if m.project != nil {
		systemPrompt = external.BuildTaskContext(m.project)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel

	streamCmd := func() tea.Msg {
		reader, cmd, err := external.ClaudeAskStreaming(ctx, query, systemPrompt)
		if err != nil {
			return AskStreamDoneMsg{Err: err}
		}
		m.streamReader = reader
		m.streamCmd = cmd

		buf := make([]byte, 1024)
		n, readErr := reader.Read(buf)
		if n > 0 {
			return AskStreamChunkMsg{Text: string(buf[:n])}
		}
		if readErr != nil {
			_ = cmd.Wait()
			return AskStreamDoneMsg{Err: readErr}
		}
		return AskStreamDoneMsg{}
	}
	return tea.Batch(streamCmd, m.spinner.Tick())
}

func (m *AskViewModel) readNextChunk() tea.Cmd {
	reader := m.streamReader
	cmd := m.streamCmd
	if reader == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if n > 0 {
			return AskStreamChunkMsg{Text: string(buf[:n])}
		}
		if cmd != nil {
			_ = cmd.Wait()
		}
		if err == io.EOF || err == nil {
			return AskStreamDoneMsg{}
		}
		return AskStreamDoneMsg{Err: err}
	}
}

func (m *AskViewModel) applyCommands() tea.Cmd {
	commands := make([]string, len(m.commands))
	copy(commands, m.commands)
	m.commands = nil

	return func() tea.Msg {
		for _, c := range commands {
			parts := strings.Fields(c)
			if len(parts) < 2 {
				continue
			}
			cmd := exec.Command(parts[0], parts[1:]...)
			out, err := cmd.CombinedOutput()
			return AskCommandResultMsg{Command: c, Output: string(out), Err: err}
		}
		return nil
	}
}

// wrapResponse word-wraps response text, preserving code blocks verbatim.
func (m *AskViewModel) wrapResponse() string {
	wrapWidth := m.viewport.Width - 2
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	var b strings.Builder
	lines := strings.Split(m.response, "\n")
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}
		if inCodeBlock || len(line) <= wrapWidth {
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}
		b.WriteString(wordWrap(line, wrapWidth))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// wordWrap wraps a single line at word boundaries.
func wordWrap(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	var lines []string
	for len(text) > width {
		bp := width
		for i := width; i > 0; i-- {
			if text[i-1] == ' ' {
				bp = i - 1
				break
			}
		}
		if bp == width {
			// No space found, force break
			bp = width
		}
		lines = append(lines, text[:bp])
		text = strings.TrimLeft(text[bp:], " ")
	}
	if text != "" {
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

func (m *AskViewModel) renderResponse() string {
	if m.response == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.wrapResponse())

	if m.state == askStateResult && len(m.commands) > 0 {
		b.WriteString("\n\n")
		cmdStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
		b.WriteString(cmdStyle.Render(fmt.Sprintf("── %d command(s) extracted ──", len(m.commands))))
		b.WriteString("\n")
		for i, c := range m.commands {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, c))
		}
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press 'a' to apply commands"))
	}

	return b.String()
}

// SetSize implements View.
func (m *AskViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	taWidth := width - 4
	if taWidth < 30 {
		taWidth = 30
	}
	m.textarea.SetWidth(taWidth)

	vpHeight := height - 10
	if vpHeight < 5 {
		vpHeight = 5
	}
	vpWidth := width - 2
	if vpWidth < 30 {
		vpWidth = 30
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
}

// View implements tea.Model.
func (m *AskViewModel) View() string {
	pad := " "
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	b.WriteString(pad)
	b.WriteString(titleStyle.Render("AI Assistant"))
	if m.project != nil {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s)", m.project.Name)))
	}
	if !external.ClaudeAvailable() {
		b.WriteString(" ")
		b.WriteString(ErrorStatusStyle.Render("claude CLI not found"))
	}
	b.WriteString("\n\n")

	switch m.state {
	case askStateInput:
		b.WriteString(pad)
		b.WriteString(m.textarea.View())
		b.WriteString("\n\n")
		b.WriteString(pad)
		b.WriteString(HelpStyle.Render("Ctrl+S: submit | Esc: back"))

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
		b.WriteString("\n\n")
		b.WriteString(pad)
		b.WriteString(HelpStyle.Render("Esc: cancel"))

	case askStateResult:
		if m.ready {
			b.WriteString(pad)
			b.WriteString(m.viewport.View())
		}
		b.WriteString("\n\n")
		footer := "n: new query | q: close"
		if len(m.commands) > 0 {
			footer = "a: apply commands | " + footer
		}
		scrollPercent := ""
		if m.ready && m.viewport.TotalLineCount() > m.viewport.Height {
			scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
		}
		b.WriteString(pad)
		b.WriteString(HelpStyle.Render(footer + scrollPercent))
	}

	return b.String()
}
