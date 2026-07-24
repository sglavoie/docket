package tui

import (
	"sort"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ContextInfo holds a context name and its project count.
type ContextInfo struct {
	Name                 string
	ProjectCount         int
	ArchivedProjectCount int
	IsCustom             bool // True if this is a custom context (project grouping)
}

// ContextPickerModel allows selecting a single context to filter by.
type ContextPickerModel struct {
	*BasePicker[ContextInfo]
	projects            []*models.Project    // Catalog snapshots used to build and rename contexts
	runtime             *config.RuntimeState // Machine-local navigation/context state
	currentFilter       string               // Current filter context for highlighting
	currentCustomFilter string               // Current custom context filter for highlighting
	confirm             ConfirmDialog        // confirmation dialog
	statusMessage       string               // Status message to display
	renameMode          bool                 // True when renaming a context
	renameInput         textinput.Model
	renameOldName       string // The context name being renamed
	confirmDiscard      bool
	confirmOverlay      ConfirmDialog
}

// ShowContextPickerMsg is sent to open the context picker.
type ShowContextPickerMsg struct{}

// ContextPickerDoneMsg is sent when the context picker is done.
type ContextPickerDoneMsg struct {
	SelectedContext string
	IsCustomContext bool // True if a custom context was selected
	Canceled        bool
}

// ShowCustomContextEditorMsg is sent to open the custom context editor.
type ShowCustomContextEditorMsg struct {
	EditName string // Empty for new, non-empty for edit
}

// CustomContextDeletedMsg is sent when a custom context is deleted.
type CustomContextDeletedMsg struct {
	Name      string
	WasActive bool // True if the deleted context was the active filter
}

// ContextRenamedMsg is sent when a project context is renamed.
type ContextRenamedMsg struct {
	OldName      string
	NewName      string
	ProjectCount int
}

// NewContextPickerModelFromRuntime creates a context picker from the cached
// catalog and machine domains.
func NewContextPickerModelFromRuntime(runtime *config.RuntimeState) *ContextPickerModel {
	projects := runtime.ActiveProjects()
	contexts := buildContextListWithCustom(projects, runtime)

	picker := NewBasePicker(contexts, PickerConfig{
		Title:          "Filter by Context",
		EmptyMessage:   "No contexts available.",
		NoMatchMessage: "No matching contexts.",
		MultiSelect:    false,
		ShowCounts:     true,
	})

	return &ContextPickerModel{
		BasePicker: picker,
		projects:   projects,
		runtime:    runtime,
	}
}

// buildContextListWithCustom gathers contexts including custom contexts. Project
// contexts derive from the passed catalog projects; custom contexts and their
// project counts are machine-local state read from the runtime.
func buildContextListWithCustom(projects []*models.Project, runtime *config.RuntimeState) []ContextInfo {
	// Count projects per context
	contextCounts := make(map[string]int)
	for _, p := range projects {
		if p.Context != "" {
			contextCounts[p.Context]++
		}
	}

	// Build sorted list
	contexts := make([]ContextInfo, 0, len(contextCounts)+1)

	// Add "All Contexts" option first (empty string value)
	contexts = append(contexts, ContextInfo{
		Name:         "",
		ProjectCount: len(projects),
		IsCustom:     false,
	})

	// Add custom contexts with "+" prefix, sorted alphabetically
	customNames := runtime.Machine().GetCustomContextNames()
	for _, name := range customNames {
		contexts = append(contexts, ContextInfo{
			Name:                 name,
			ProjectCount:         runtime.CustomContextProjectCount(name),
			ArchivedProjectCount: runtime.CustomContextArchivedProjectCount(name),
			IsCustom:             true,
		})
	}

	// Add actual project contexts sorted alphabetically
	names := make([]string, 0, len(contextCounts))
	for name := range contextCounts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		contexts = append(contexts, ContextInfo{
			Name:         name,
			ProjectCount: contextCounts[name],
			IsCustom:     false,
		})
	}

	return contexts
}

// setStatus sets the status message.
func (m *ContextPickerModel) setStatus(msg string) {
	m.statusMessage = msg
}

// clearStatus clears the status message.
func (m *ContextPickerModel) clearStatus() {
	m.statusMessage = ""
}

// Init initializes the model.
func (m *ContextPickerModel) Init() tea.Cmd {
	return nil
}

// ShouldCaptureEsc returns true when in rename mode or delete confirmation.
func (m *ContextPickerModel) ShouldCaptureEsc() bool {
	return m.renameMode || m.confirm.IsActive() || m.confirmDiscard
}

// HasTextInput returns true when in rename mode.
func (m *ContextPickerModel) HasTextInput() bool {
	return m.renameMode
}

func (m *ContextPickerModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
		return &m.confirmOverlay
	}
	m.confirmOverlay.Cancel()
	return &m.confirm
}

func (m *ContextPickerModel) StatusBarLabel() string { return "Filter by Context" }
