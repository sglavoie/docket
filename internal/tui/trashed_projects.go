package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

// TrashedProjectItem represents a project in the trash.
type TrashedProjectItem struct {
	Slug string
}

// FilterText returns the text used for filtering.
func (t TrashedProjectItem) FilterText() string {
	return t.Slug
}

// DisplayText returns the display text.
func (t TrashedProjectItem) DisplayText() string {
	return t.Slug
}

// CountValue returns 0 (not applicable for trashed projects).
func (t TrashedProjectItem) CountValue() int {
	return 0
}

// ItemID returns the slug as the unique identifier.
func (t TrashedProjectItem) ItemID() string {
	return t.Slug
}

// TrashedProjectsModel displays trashed projects and allows restoration.
type TrashedProjectsModel struct {
	*BasePicker[TrashedProjectItem]
	dataDir     string
	confirm     ConfirmDialog
	purgeTarget *TrashedProjectItem
}

// NewTrashedProjectsModel creates a new trashed projects picker.
func NewTrashedProjectsModel(dataDir string) *TrashedProjectsModel {
	slugs, _ := storage.ListTrashedProjects(dataDir)

	items := make([]TrashedProjectItem, len(slugs))
	for i, slug := range slugs {
		items[i] = TrashedProjectItem{Slug: slug}
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Restore from Trash",
		EmptyMessage:   "No deleted projects in trash",
		NoMatchMessage: "No matching projects",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	return &TrashedProjectsModel{
		BasePicker: picker,
		dataDir:    dataDir,
	}
}

// Init initializes the model.
func (m *TrashedProjectsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TrashedProjectsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle confirmation mode first
		if m.confirm.IsActive() {
			target := m.purgeTarget
			if confirmed, handled := m.confirm.HandleKey(msg.String()); handled {
				if confirmed && target != nil {
					slug := target.Slug
					m.purgeTarget = nil
					return m, func() tea.Msg {
						return PurgeFromTrashMsg{Slug: slug}
					}
				}
				m.purgeTarget = nil
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return CloseTrashedProjectsMsg{}
			}

		case "d":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				target := m.Filtered[m.Cursor]
				m.purgeTarget = &target
				m.confirm.Show("purge", fmt.Sprintf("PERMANENTLY delete '%s'? (y/n)", target.Slug))
			}
			return m, nil

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				slug := m.Filtered[m.Cursor].Slug
				return m, func() tea.Msg {
					return RestoreFromTrashMsg{Slug: slug}
				}
			}
			return m, func() tea.Msg {
				return CloseTrashedProjectsMsg{}
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

// View renders the trashed projects picker.
func (m *TrashedProjectsModel) View() string {
	boxWidth := 50
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Confirmation prompt or keybinding hints
	if m.confirm.IsActive() {
		content.WriteString(m.confirm.Render())
	} else {
		content.WriteString(MutedStyle.Render("Enter: restore | d: delete | q: close"))
	}
	content.WriteString("\n\n")

	// Search bar
	content.WriteString(m.RenderSearchBar())

	// List
	maxVisible := 12
	if m.Height() > 0 {
		maxVisible = m.Height() - 14
	}
	content.WriteString(m.RenderList(maxVisible))

	// Footer
	content.WriteString("\n")
	content.WriteString(m.RenderFooter())

	// Build subtitle
	subtitle := fmt.Sprintf("%d projects", len(m.Items))

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    m.Config.Title,
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}
