package tui

import (
	"io"
	"os/exec"
)

// AskStreamStartMsg signals the start of a streaming response.
type AskStreamStartMsg struct {
	Reader    io.ReadCloser
	Cmd       *exec.Cmd
	FirstText string
}

// AskStreamChunkMsg contains a chunk of streamed text.
type AskStreamChunkMsg struct {
	Text string
}

// AskStreamDoneMsg signals the end of streaming.
type AskStreamDoneMsg struct {
	Err error
}

// AskCommandResultMsg contains the result of executing a single command.
type AskCommandResultMsg struct {
	Command string
	Output  string
	Err     error
}

// AskCommandBatchMsg contains results from multiple commands.
type AskCommandBatchMsg struct {
	Results []AskCommandResultMsg
}
