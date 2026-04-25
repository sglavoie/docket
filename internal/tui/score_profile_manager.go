package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/scoring"
)

// ScoreProfileManagerModel manages scoring profiles and project assignment.
type ScoreProfileManagerModel struct {
	config           *config.Config
	configManager    *config.Manager
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

// NewScoreProfileManagerModel creates a new score profile manager view.
func NewScoreProfileManagerModel(cfg *config.Config, manager *config.Manager, currentProjectID string) *ScoreProfileManagerModel {
	profiles := make([]scoring.ScoringProfile, len(cfg.ScoringProfiles))
	copy(profiles, cfg.ScoringProfiles)
	return &ScoreProfileManagerModel{
		config:           cfg,
		configManager:    manager,
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
	if m.config.ProjectScoringProfile == nil || m.currentProjectID == "" {
		return ""
	}
	return m.config.ProjectScoringProfile[m.currentProjectID]
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
