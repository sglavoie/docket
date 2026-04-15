package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// SaveFilterInputModel handles naming a saved filter preset.
type SaveFilterInputModel struct {
	summary        string // Summary of active filters being saved
	input          textinput.Model
	err            string
	width          int
	height         int
	canceled       bool
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewSaveFilterInputModel creates a new save view input model.
func NewSaveFilterInputModel(summary string) *SaveFilterInputModel {
	input := textinput.New()
	input.Placeholder = "Enter a name for this view"
	input.Focus()
	input.CharLimit = 50
	input.SetWidth(40)

	return &SaveFilterInputModel{
		summary: summary,
		input:   input,
	}
}

// SetSize sets the view dimensions.
func (m *SaveFilterInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(min(width-20, 40))
}

func (m *SaveFilterInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SaveFilterInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.input.Value()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			m.canceled = true
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.err = "Name cannot be empty"
				return m, nil
			}

			return m, func() tea.Msg {
				return SaveFilterMsg{Name: name}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = ""
	return m, cmd
}

func (m *SaveFilterInputModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Save View", ""))
	b.WriteString("\n\n")

	// Show summary of what is being saved
	if m.summary != "" {
		b.WriteString(MutedStyle.Render("Saving: "))
		b.WriteString(BoldStyle.Render(m.summary))
		b.WriteString("\n\n")
	}

	b.WriteString(SubtitleStyle.Render("Name"))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(ErrorIconStyle.Render(m.err))
		b.WriteString("\n")
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter to save · Esc to cancel"), "", m.width))

	return tea.NewView(b.String())
}

// HasTextInput returns true because the save view input is active.
func (m *SaveFilterInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *SaveFilterInputModel) ShouldCaptureEsc() bool { return true }

func (m *SaveFilterInputModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *SaveFilterInputModel) IsFullHeight() bool { return false }

// describeSavedFilterSummary builds a human-readable summary of a SavedFilter.
func describeSavedFilterSummary(sf *config.SavedFilter) string {
	if sf.Mode == "criteria" {
		return describeCriteriaFilter(sf)
	}
	return describeSnapshotFilters(
		sf.FilterType, sf.FilterStatus, sf.FilterPriority,
		sf.FilterTags, sf.FilterSections, sf.FilterDueDate,
		sf.FilterActionable, sf.FilterHideDimmed, sf.FilterAttrs, sf.FilterScoreMin,
		sf.SortField, sf.ViewMode,
	)
}

// describeCriteriaFilter builds a summary for a criteria-mode saved filter.
func describeCriteriaFilter(sf *config.SavedFilter) string {
	if len(sf.Criteria) == 0 {
		return "No criteria"
	}
	var parts []string
	for _, c := range sf.Criteria {
		parts = append(parts, c.Field+" "+c.Operation+" "+c.Value)
	}
	logic := sf.EffectiveLogic()
	sep := " AND "
	if logic == "or" {
		sep = " OR "
	}
	return strings.Join(parts, sep)
}

// describeSnapshotFilters builds a human-readable summary of a snapshot filter's state.
func describeSnapshotFilters(filterType string, filterStatus string, filterPriority int, filterTags []string, filterSections []string, filterDueDate int, filterActionable bool, filterHideDimmed bool, filterAttrs map[string]string, filterScoreMin float64, sortField int, viewMode int) string {
	var parts []string

	switch viewMode {
	case 1:
		parts = append(parts, "Next")
	case 2:
		parts = append(parts, "Inbox")
	case 3:
		parts = append(parts, "Waiting")
	case 4:
		parts = append(parts, "Someday")
	case 5:
		parts = append(parts, "Completed")
	case 6:
		parts = append(parts, "Snoozed")
	case 7:
		parts = append(parts, "All")
	}

	if filterPriority == 1 {
		parts = append(parts, "!Hi")
	}
	if filterType != "" {
		parts = append(parts, "type:"+filterType)
	}
	if filterStatus != "" {
		parts = append(parts, "status:"+filterStatus)
	}
	if len(filterTags) > 0 {
		for _, tag := range filterTags {
			parts = append(parts, "#"+tag)
		}
	}
	if len(filterSections) > 0 {
		for _, sec := range filterSections {
			parts = append(parts, "@"+sec)
		}
	}
	switch filterDueDate {
	case 1:
		parts = append(parts, "due:overdue")
	case 2:
		parts = append(parts, "due:today")
	case 3:
		parts = append(parts, "due:week")
	case 4:
		parts = append(parts, "due:none")
	}
	if filterActionable {
		parts = append(parts, "ready")
	}
	if filterHideDimmed {
		parts = append(parts, "no-dim")
	}
	if len(filterAttrs) > 0 {
		for k, v := range filterAttrs {
			parts = append(parts, k+"="+v)
		}
	}
	if filterScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", filterScoreMin))
	}

	sortNames := map[int]string{1: "priority", 2: "due date", 3: "created", 4: "status", 5: "score"}
	if name, ok := sortNames[sortField]; ok {
		parts = append(parts, "sort:"+name)
	}

	if len(parts) == 0 {
		return "Active (default)"
	}
	return strings.Join(parts, " | ")
}

func (m *SaveFilterInputModel) StatusBarLabel() string { return "Save Filter" }
