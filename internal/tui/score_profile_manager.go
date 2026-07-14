package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/scoring"
)

// ScoreProfileManagerModel manages scoring profiles and project assignment. It
// reads scoring profiles and per-project assignments from the automation domain
// (runtime.Automation()) and commits mutations through the automation committer
// so the in-memory snapshot refreshes on save — no config.Config facade.
type ScoreProfileManagerModel struct {
	runtime *config.RuntimeState
	// coordinator routes the durable automation write through the secondary
	// feature-coordinator seam (feature_coordinator_secondary.go).
	coordinator      WorkflowCoordinator
	profiles         []scoring.ScoringProfile
	cursor           int
	confirm          ConfirmDialog
	currentProjectID string
	statusMessage    string
	pendingPrefix    string

	// Inline preset picker sub-mode
	pickingPreset bool
	presetOptions []string
	presetCursor  int

	width  int
	height int
}

// NewScoreProfileManagerModel creates a new score profile manager view backed by
// the runtime automation snapshot.
func NewScoreProfileManagerModel(runtime *config.RuntimeState, currentProjectID string) *ScoreProfileManagerModel {
	profiles := slices.Clone(runtime.Automation().ScoringProfiles)
	return &ScoreProfileManagerModel{
		runtime:          runtime,
		coordinator:      newWorkflowCoordinator(runtime),
		profiles:         profiles,
		currentProjectID: currentProjectID,
	}
}

// setStatus sets the status message.
func (m *ScoreProfileManagerModel) setStatus(msg string) {
	m.statusMessage = msg
}

// SetSize implements View.
func (m *ScoreProfileManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ScoreProfileManagerModel) GetConfirmDialog() *ConfirmDialog { return &m.confirm }

func (m *ScoreProfileManagerModel) Init() tea.Cmd {
	return nil
}

// assignedProfileID returns the profile ID assigned to the current project, or "".
func (m *ScoreProfileManagerModel) assignedProfileID() string {
	assignments := m.runtime.Automation().ProjectScoringProfile
	if assignments == nil || m.currentProjectID == "" {
		return ""
	}
	return assignments[m.currentProjectID]
}

// ShouldCaptureEsc returns true when confirm or preset picker is active.
func (m *ScoreProfileManagerModel) ShouldCaptureEsc() bool {
	return m.confirm.IsActive() || m.pickingPreset
}

// HasActiveSearch returns false.
func (m *ScoreProfileManagerModel) HasActiveSearch() bool {
	return false
}

// GetHelpItems returns help items for this view.
func (m *ScoreProfileManagerModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewScoreProfileManager)
}

func (m *ScoreProfileManagerModel) StatusBarLabel() string { return "Score Profiles" }
