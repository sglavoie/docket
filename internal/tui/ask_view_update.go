package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
)

// Update implements tea.Model.
func (m *AskViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case AskStreamStartMsg:
		m.streamReader = msg.Reader
		m.streamCmd = msg.Cmd
		m.response += msg.FirstText
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, m.readNextChunk()

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
		m.viewport.GotoTop()
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

	case AskCommandBatchMsg:
		for _, result := range msg.Results {
			status := "ok"
			if result.Err != nil {
				status = fmt.Sprintf("error: %v", result.Err)
			}
			m.response += fmt.Sprintf("\n\n> %s\n%s", result.Command, status)
			if result.Output != "" {
				m.response += "\n" + result.Output
			}
		}
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	if m.state == askStateLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *AskViewModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case askStateInput:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textarea.Value()) != "" {
				return m, nil
			}
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
			// Close the reader pipe and reap the child process to prevent
			// zombie processes. The context cancellation kills the subprocess,
			// but Wait() is still needed to release OS resources.
			if m.streamReader != nil {
				_ = m.streamReader.Close()
			}
			if m.streamCmd != nil {
				_ = m.streamCmd.Wait()
			}
			m.state = askStateInput
			m.streamReader = nil
			m.streamCmd = nil
			m.textarea.Focus()
			return m, textarea.Blink
		}

	case askStateResult:
		switch msg.String() {
		case "esc":
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
