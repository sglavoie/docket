package tui

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
)

func (m *AskViewModel) submitQuery(query string) tea.Cmd {
	m.state = askStateLoading
	m.response = ""
	m.commands = nil
	m.textarea.Blur()

	var systemPrompt string
	if m.project != nil {
		systemPrompt = external.BuildTaskContext(m.project)
	}
	if m.cliReference != "" {
		systemPrompt += "\n\n" + external.BuildCLIContext(m.cliReference, query)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel

	streamCmd := func() tea.Msg {
		reader, cmd, err := external.ClaudeAskStreaming(ctx, query, systemPrompt)
		if err != nil {
			return AskStreamDoneMsg{Err: err}
		}

		buf := make([]byte, 1024)
		n, readErr := reader.Read(buf)
		if n > 0 {
			return AskStreamStartMsg{Reader: reader, Cmd: cmd, FirstText: string(buf[:n])}
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
		if errors.Is(err, io.EOF) || err == nil {
			return AskStreamDoneMsg{}
		}
		return AskStreamDoneMsg{Err: err}
	}
}
