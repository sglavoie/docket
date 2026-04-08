package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/scoring"
)

// ScoringProfileOption represents a selectable scoring profile.
type ScoringProfileOption struct {
	ProfileID string
	Name      string
	Formula   scoring.Formula
	IsCurrent bool
}

// FilterText returns the text used for case-insensitive filtering.
func (o ScoringProfileOption) FilterText() string {
	if o.ProfileID == "" {
		return "none"
	}
	return fmt.Sprintf("%s %s", o.Name, o.Formula)
}

// DisplayText returns the primary text to display for this item.
func (o ScoringProfileOption) DisplayText() string {
	if o.ProfileID == "" {
		return "None"
	}
	return fmt.Sprintf("%s (%s)", o.Name, o.Formula)
}

// CountValue returns 0 (not used).
func (o ScoringProfileOption) CountValue() int {
	return 0
}

// ItemID returns the profile ID as a unique identifier.
func (o ScoringProfileOption) ItemID() string {
	return o.ProfileID
}

// ShowScoringProfilePickerMsg is sent to open the scoring profile picker.
type ShowScoringProfilePickerMsg struct {
	ProjectID string
}

// ScoringProfilePickerDoneMsg is sent when the picker is closed.
type ScoringProfilePickerDoneMsg struct {
	ProjectID string
	ProfileID string
	Canceled  bool
}

// ScoringProfilePickerModel allows selecting a scoring profile for a project.
type ScoringProfilePickerModel struct {
	*BasePicker[ScoringProfileOption]
	projectID     string
	config        *config.Config
	configManager *config.Manager
	currentName   string
}

// NewScoringProfilePickerModel creates a new scoring profile picker.
func NewScoringProfilePickerModel(cfg *config.Config, mgr *config.Manager, projectID string) *ScoringProfilePickerModel {
	current := cfg.GetProjectScoringProfile(projectID)
	currentID := ""
	currentName := "None"
	if current != nil {
		currentID = current.ID
		currentName = current.Name
	}

	// Build options: "None" first, then all profiles
	options := []ScoringProfileOption{
		{ProfileID: "", Name: "None", IsCurrent: currentID == ""},
	}
	for _, p := range cfg.ScoringProfiles {
		options = append(options, ScoringProfileOption{
			ProfileID: p.ID,
			Name:      p.Name,
			Formula:   p.Formula,
			IsCurrent: p.ID == currentID,
		})
	}

	picker := NewBasePicker(options, PickerConfig{
		Title:          "Scoring Profile",
		EmptyMessage:   "No scoring profiles configured.",
		NoMatchMessage: "No matching profiles.",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	// Position cursor at current selection
	picker.SetCursorToID(currentID)

	return &ScoringProfilePickerModel{
		BasePicker:    picker,
		projectID:     projectID,
		config:        cfg,
		configManager: mgr,
		currentName:   currentName,
	}
}

// Init initializes the model.
func (m *ScoringProfilePickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *ScoringProfilePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			projectID := m.projectID
			return m, func() tea.Msg {
				return ScoringProfilePickerDoneMsg{
					ProjectID: projectID,
					Canceled:  true,
				}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				selected := m.Filtered[m.Cursor]
				projectID := m.projectID
				profileID := selected.ProfileID
				return m, func() tea.Msg {
					return ScoringProfilePickerDoneMsg{
						ProjectID: projectID,
						ProfileID: profileID,
						Canceled:  false,
					}
				}
			}
			projectID := m.projectID
			return m, func() tea.Msg {
				return ScoringProfilePickerDoneMsg{
					ProjectID: projectID,
					Canceled:  true,
				}
			}

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the scoring profile picker.
func (m *ScoringProfilePickerModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Scoring Profile", ""))
	b.WriteString("\n\n")

	// Current assignment
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", m.currentName)))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(m.RenderSearchBar())

	// List with checkmark on current
	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 10
	}
	if maxVisible < 5 {
		maxVisible = 5
	}

	if len(m.Filtered) == 0 {
		if len(m.Items) == 0 {
			b.WriteString(MutedStyle.Render(m.Config.EmptyMessage))
		} else {
			b.WriteString(MutedStyle.Render(m.Config.NoMatchMessage))
		}
		b.WriteString("\n")
	} else {
		start := 0
		if m.Cursor >= maxVisible {
			start = m.Cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.Filtered))

		for i := start; i < end; i++ {
			opt := m.Filtered[i]
			cursor := "  "
			if i == m.Cursor {
				cursor = "  "
			}

			check := "  "
			if opt.IsCurrent {
				check = "✓ "
			}

			line := fmt.Sprintf("%s%s%s", cursor, check, opt.DisplayText())
			if i == m.Cursor {
				b.WriteString(TaskJumperSelectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}

		if len(m.Filtered) > maxVisible {
			remaining := len(m.Filtered) - end
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	// Footer
	left := HelpStyle.Render("↑/↓ navigate · Enter select · / search · Esc cancel")
	right := MutedStyle.Render(fmt.Sprintf("%d profiles", len(m.Filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return tea.NewView(b.String())
}

func (m *ScoringProfilePickerModel) StatusBarLabel() string { return "Scoring Profile" }
