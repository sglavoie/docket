package tui

// ShowAskMsg opens the AI assistant view.
type ShowAskMsg struct{}

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
