package tui

import (
	"io"
	"os/exec"
)

// ShowAskMsg opens the AI assistant view.
type ShowAskMsg struct{}

// AskStreamStartMsg carries the streaming reader and command back to the
// Update loop so they are assigned on the main goroutine, avoiding a data
// race between the tea.Cmd goroutine and the model.
type AskStreamStartMsg struct {
	Reader    io.ReadCloser
	Cmd       *exec.Cmd
	FirstText string
}

// AskStreamChunkMsg delivers a chunk of streaming response text.
type AskStreamChunkMsg struct {
	Text string
}

// AskStreamDoneMsg signals the streaming response is complete.
type AskStreamDoneMsg struct {
	FullResponse string
	Err          error
}

// AskApplyCommandsMsg requests execution of extracted tt commands.
type AskApplyCommandsMsg struct {
	Commands []string
}

// AskCommandResultMsg reports the result of executing a tt command.
type AskCommandResultMsg struct {
	Command string
	Output  string
	Err     error
}

// AskCommandBatchMsg reports the results of executing all tt commands.
type AskCommandBatchMsg struct {
	Results []AskCommandResultMsg
}
