package tui

import (
	"io"
	"os/exec"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
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

// AskConsentSavedMsg resumes a pending query after remembered consent is saved.
type AskConsentSavedMsg struct {
	Err error
}

// AskActionPlanAppliedMsg contains the one transaction produced by applying an
// accepted structured plan.
type AskActionPlanAppliedMsg struct {
	Result         app.ActionPlanResult
	PendingDomains *config.Domains
	Err            error
}
