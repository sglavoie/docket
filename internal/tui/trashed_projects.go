package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

// TrashedProjectItem represents a single deleted generation in the trash.
type TrashedProjectItem struct {
	EntryID   string
	Slug      string
	Name      string
	DeletedAt string
}

// FilterText returns the text used for filtering.
func (t TrashedProjectItem) FilterText() string {
	return t.Slug + " " + t.Name
}

// DisplayText returns the display text.
func (t TrashedProjectItem) DisplayText() string {
	if t.DeletedAt != "" {
		return fmt.Sprintf("%s  (deleted %s)", t.Slug, t.DeletedAt)
	}
	return t.Slug
}

// CountValue returns 0 (not applicable for trashed projects).
func (t TrashedProjectItem) CountValue() int {
	return 0
}

// ItemID returns the trash entry ID as the unique identifier.
func (t TrashedProjectItem) ItemID() string {
	return t.EntryID
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
	entries, _ := storage.ListTrashEntries(dataDir)

	items := make([]TrashedProjectItem, len(entries))
	for i, e := range entries {
		items[i] = TrashedProjectItem{
			EntryID:   e.EntryID,
			Slug:      e.Slug,
			Name:      e.Name,
			DeletedAt: storage.FormatRelativeTime(e.DeletedAt),
		}
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
					entryID := target.EntryID
					slug := target.Slug
					m.purgeTarget = nil
					return m, func() tea.Msg {
						return PurgeFromTrashMsg{EntryID: entryID, Slug: slug}
					}
				}
				m.purgeTarget = nil
				return m, nil
			}
		}

		switch msg.String() {
		case "esc":
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
				selected := m.Filtered[m.Cursor]
				return m, func() tea.Msg {
					return RestoreFromTrashMsg{EntryID: selected.EntryID, Slug: selected.Slug}
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

	out.WriteString(MutedStyle.Render("Enter: restore · d: delete · Esc: close"))
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
