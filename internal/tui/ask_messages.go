package tui

import (
	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// AskPreferencesSavedMsg resumes submission only after the selected harness
// and all valid per-harness directory drafts have been persisted.
type AskPreferencesSavedMsg struct {
	OperationID uint64
	Harness     aiharness.Name
	ConfigDir   string
	Drafts      map[aiharness.Name]string
	Err         error
}

// AskStreamStartMsg signals the start of a streaming response.
type AskStreamStartMsg struct {
	RequestID uint64
	Stream    external.Stream
}

// AskStreamChunkMsg contains a chunk of streamed text.
type AskStreamChunkMsg struct {
	RequestID uint64
	Stream    external.Stream
	Text      string
	ReadErr   error
}

// AskStreamDoneMsg signals the end of streaming.
type AskStreamDoneMsg struct {
	RequestID uint64
	Err       error
}

// AskConsentSavedMsg resumes a pending query after remembered consent is saved.
type AskConsentSavedMsg struct {
	OperationID uint64
	Err         error
}

// AskWorkspaceLoadedMsg carries the active-project task snapshot built after a
// contextual query has cleared remembered-consent gating.
type AskWorkspaceLoadedMsg struct {
	RequestID uint64
	Projects  []*models.Project
	Err       error
}

// AskActionPlanAppliedMsg contains the one transaction produced by applying an
// accepted structured plan.
type AskActionPlanAppliedMsg struct {
	OperationID    uint64
	Result         app.ActionPlanResult
	PendingDomains *config.Domains
	RepairWarning  error
	Err            error
}
