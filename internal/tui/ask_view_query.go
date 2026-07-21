package tui

import (
	"context"
	"errors"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/external"
)

func (m *AskViewModel) submitQuery(query string) tea.Cmd {
	if strings.TrimSpace(query) == "" {
		m.state = askStateInput
		m.pendingQuery = ""
		return m.focusAskField(askFocusQuery)
	}

	// Direct callers normally arrive from the input state, but invalidating any
	// prior lifecycle here also makes a superseding request safe by construction.
	m.cancelActiveRequest()
	m.requestID = nextAskOperationID()
	requestID := m.requestID

	m.state = askStateLoading
	m.pendingQuery = ""
	m.inputError = ""
	m.response = ""
	m.raw = ""
	m.prepared = nil
	m.acceptedSafe = false
	m.requestQuery = query
	m.requestContextMode = m.contextMode
	m.failureQuery = ""
	m.reuseAvailable = false
	m.textarea.Blur()
	m.configInput.Blur()

	// Snapshot active projects on the UI event path before the asynchronous Tea
	// command starts. The loader then performs only task-file I/O against these
	// private, rehydrated models. Focused constructors retain their supplied
	// single project.
	if m.workspaceLoader != nil && m.requestContextMode != external.TaskContextNone && m.contextConsentGranted() {
		projects := m.runtime.ActiveProjects()
		loader := m.workspaceLoader
		loadCmd := func() tea.Msg {
			loadedProjects, err := loader(projects)
			return AskWorkspaceLoadedMsg{RequestID: requestID, Projects: loadedProjects, Err: err}
		}
		return tea.Batch(loadCmd, m.spinner.Tick())
	}

	var taskContext string
	if m.project != nil && m.requestContextMode != external.TaskContextNone && m.contextConsentGranted() {
		taskContext = external.BuildTaskContextWithMode(m.project, m.requestContextMode)
	}
	return m.startQuery(requestID, query, taskContext)
}

// startQuery crosses the provider boundary only after any required workspace
// snapshot has been loaded successfully.
func (m *AskViewModel) startQuery(requestID uint64, query, taskContext string) tea.Cmd {
	var cliContext string
	if m.cliReference != "" {
		cliContext = external.BuildCLIContext(m.cliReference, query)
	}
	workspaceDir := m.requestWorkspaceDir()
	systemPrompt := external.BuildActionPlanPrompt(taskContext, cliContext)
	if m.harness == aiharness.Codex && workspaceDir != "" {
		systemPrompt = external.BuildCodexWorkspaceActionPlanPrompt(taskContext, cliContext)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel
	runner := m.runner
	request := external.Request{
		Harness:      m.harness,
		ConfigDir:    m.configDir,
		WorkspaceDir: workspaceDir,
		Query:        query,
		SystemPrompt: systemPrompt,
	}

	streamCmd := func() tea.Msg {
		if runner == nil {
			return AskStreamDoneMsg{RequestID: requestID, Err: errors.New("AI runner is unavailable")}
		}
		stream, err := runner.Start(ctx, request)
		if err != nil {
			return AskStreamDoneMsg{RequestID: requestID, Err: err}
		}
		return AskStreamStartMsg{RequestID: requestID, Stream: stream}
	}
	return tea.Batch(streamCmd, m.spinner.Tick())
}

// requestWorkspaceDir exposes filesystem inspection capability only to a
// full-context request that has current provider-specific consent. Redacted,
// no-context, and declined requests keep this field empty.
func (m *AskViewModel) requestWorkspaceDir() string {
	if m.requestContextMode != external.TaskContextFull || !m.contextConsentGranted() || m.runtime == nil {
		return ""
	}
	return m.runtime.DataDir()
}

// cancelActiveRequest invalidates the current generation before canceling its
// context and stream. The replacement generation comes from the process-wide
// sequence so a stale message can never match another AskViewModel instance.
func (m *AskViewModel) cancelActiveRequest() {
	hadActiveRequest := m.cancelFn != nil || m.stream != nil || m.state == askStateLoading
	if hadActiveRequest {
		m.requestID = nextAskOperationID()
	}
	if m.cancelFn != nil {
		m.cancelFn()
	}
	if m.stream != nil {
		m.stream.Cancel()
	}
	m.cancelFn = nil
	m.stream = nil
}

func (m *AskViewModel) contextConsentGranted() bool {
	return m.runtime != nil && m.runtime.Settings().AIConsent.GrantedFor(m.harness)
}

func (m *AskViewModel) readNextChunk(requestID uint64, stream external.Stream) tea.Cmd {
	if stream == nil {
		return func() tea.Msg {
			return AskStreamDoneMsg{RequestID: requestID, Err: errors.New("AI runner returned an empty stream")}
		}
	}
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, readErr := stream.Read(buf)
		if n > 0 || readErr == nil {
			return AskStreamChunkMsg{
				RequestID: requestID,
				Stream:    stream,
				Text:      string(buf[:n]),
				ReadErr:   readErr,
			}
		}
		return waitForAskStream(requestID, stream, readErr)
	}
}

func waitForAskStreamCmd(requestID uint64, stream external.Stream, readErr error) tea.Cmd {
	return func() tea.Msg {
		return waitForAskStream(requestID, stream, readErr)
	}
}

func waitForAskStream(requestID uint64, stream external.Stream, readErr error) AskStreamDoneMsg {
	waitErr := stream.Wait()
	var result error
	switch {
	case errors.Is(readErr, external.ErrHarnessOutputLimit):
		result = external.ErrHarnessOutputLimit
	case waitErr != nil:
		result = waitErr
	case readErr != nil && !errors.Is(readErr, io.EOF):
		result = readErr
	}
	return AskStreamDoneMsg{RequestID: requestID, Err: result}
}
