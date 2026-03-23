package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
		EmptyMessage:   "No deleted projects in trash.",
		NoMatchMessage: "No matching projects.",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	return &TrashedProjectsModel{
		BasePicker: picker,
		dataDir:    dataDir,
	}
}

func (m *TrashedProjectsModel) GetConfirmDialog() *ConfirmDialog { return &m.confirm }

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

	case tea.KeyPressMsg:
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
				m.confirm.Show("purge", fmt.Sprintf("PERMANENTLY delete '%s'?", target.Slug))
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
func (m *TrashedProjectsModel) View() tea.View {
	subtitle := fmt.Sprintf("%d projects", len(m.Items))

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(m.Config.Title, subtitle))
	out.WriteString("\n\n")

	out.WriteString(MutedStyle.Render("Enter: restore · d: delete · q: close"))
	out.WriteString("\n\n")

	// Search bar
	out.WriteString(m.RenderSearchBar())

	// List
	maxVisible := 12
	if m.Height() > 0 {
		maxVisible = m.Height() - 8
	}
	out.WriteString(m.RenderList(maxVisible))

	// Footer
	out.WriteString("\n")
	out.WriteString(m.RenderFooter())

	return tea.NewView(out.String())
}

func (m *TrashedProjectsModel) StatusBarLabel() string { return "Trash" }
