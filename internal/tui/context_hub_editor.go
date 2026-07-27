package tui

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// contextEditor is the Context Hub's inline create/edit surface. It collects a
// name and a selection over the *active* projects only; members the list never
// shows stay hidden and ride along with the save, so an edit never silently
// drops them. Custom contexts hand that merge to
// ContextMutator.SaveCustomContext; regular labels carry hiddenMembers here
// because SaveRegularContext takes the full membership from its caller.
type contextEditor struct {
	kind         ContextScopeKind
	originalName string
	nameInput    textinput.Model
	searchInput  textinput.Model
	searching    bool
	query        string

	projects []*models.Project
	filtered []*models.Project
	selected map[string]bool
	initial  map[string]bool

	// hiddenMembers are stored members the project list omits: archived,
	// sandbox, internal, and (for custom contexts) no-longer-cataloged
	// projects. A regular label folds them back into memberIDs because
	// SaveRegularContext takes the request as the complete membership; a custom
	// context carries them for display only, since SaveCustomContext merges
	// them itself and would double-add a pre-merged selection.
	hiddenMembers []string

	// destination state drives the prompt shown when projects leave the default
	// context, the only label with no fallback of its own.
	destinations []string
	destCursor   int
	destPrompt   bool
	destination  string

	cursor    int
	scroll    int
	listFocus bool
	err       string
}

// newContextEditor builds the shared editor over the runtime's active projects.
// Each context kind then fills in the selection it implies.
func newContextEditor(runtime *config.RuntimeState, kind ContextScopeKind, editName string) *contextEditor {
	placeholder := "Custom context name"
	if kind == ContextScopeRegular {
		placeholder = "Context label"
	}
	nameInput := formFactory.NewTextInput(placeholder)
	nameInput.CharLimit = 50
	nameInput.Focus()
	if editName != "" {
		nameInput.SetValue(editName)
		nameInput.CursorEnd()
	}

	searchInput := formFactory.NewTextInput("Search projects")
	searchInput.CharLimit = 64

	projects := runtime.ActiveProjects()
	return &contextEditor{
		kind:         kind,
		originalName: editName,
		nameInput:    nameInput,
		searchInput:  searchInput,
		projects:     projects,
		filtered:     projects,
		selected:     make(map[string]bool),
		initial:      make(map[string]bool),
	}
}

// newCustomContextEditor builds an editor over a custom context. An empty
// editName creates a new context; otherwise the stored membership is split
// against the project list: listed members are pre-checked and the rest are
// recorded as hidden, so "N selected" counts what the user can actually see and
// toggle while the members it never showed stay accounted for.
func newCustomContextEditor(runtime *config.RuntimeState, editName string) *contextEditor {
	editor := newContextEditor(runtime, ContextScopeCustom, editName)
	if editName != "" {
		listed := make(map[string]bool, len(editor.projects))
		for _, project := range editor.projects {
			if project != nil {
				listed[project.ID] = true
			}
		}
		for _, id := range runtime.Machine().GetCustomContext(editName) {
			if listed[id] {
				editor.selected[id] = true
				continue
			}
			editor.hiddenMembers = append(editor.hiddenMembers, id)
		}
	}
	maps.Copy(editor.initial, editor.selected)
	return editor
}

func (e *contextEditor) isNew() bool { return e.originalName == "" }

// savedName is the name a save asserts, which differs from originalName exactly
// when the edit renames the context.
func (e *contextEditor) savedName() string { return strings.TrimSpace(e.nameInput.Value()) }

func (e *contextEditor) isRegular() bool { return e.kind == ContextScopeRegular }

// hasChanges reports whether the editor holds unsaved edits, which is what
// arms the discard confirmation on Esc.
func (e *contextEditor) hasChanges() bool {
	if strings.TrimSpace(e.nameInput.Value()) != e.originalName {
		return true
	}
	if len(e.selected) != len(e.initial) {
		return true
	}
	for id := range e.selected {
		if !e.initial[id] {
			return true
		}
	}
	return false
}

// selectedIDs returns the checked active projects in catalog order, so a saved
// membership does not reshuffle on every edit.
func (e *contextEditor) selectedIDs() []string {
	ids := make([]string, 0, len(e.selected))
	for _, project := range e.projects {
		if project != nil && e.selected[project.ID] {
			ids = append(ids, project.ID)
		}
	}
	return ids
}

func (e *contextEditor) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(e.query))
	if query == "" {
		e.filtered = e.projects
	} else {
		filtered := make([]*models.Project, 0, len(e.projects))
		for _, project := range e.projects {
			if project == nil {
				continue
			}
			if strings.Contains(strings.ToLower(project.Name), query) ||
				strings.Contains(strings.ToLower(project.Context), query) {
				filtered = append(filtered, project)
			}
		}
		e.filtered = filtered
	}
	e.cursor = 0
	e.scroll = 0
}

func (e *contextEditor) setCursor(index int) {
	e.cursor = min(max(index, 0), max(len(e.filtered)-1, 0))
}

func (e *contextEditor) toggleCursor() {
	if e.cursor < 0 || e.cursor >= len(e.filtered) {
		return
	}
	id := e.filtered[e.cursor].ID
	if e.selected[id] {
		delete(e.selected, id)
		return
	}
	e.selected[id] = true
}

func (e *contextEditor) setFocus(list bool) {
	e.listFocus = list
	if list {
		e.nameInput.Blur()
		return
	}
	e.nameInput.Focus()
}

func (e *contextEditor) resize(width int) {
	formFactory.ResizeTextInput(&e.nameInput, width)
	formFactory.ResizeTextInput(&e.searchInput, width)
}

// openEditor arms the inline editor on a custom context. editName is empty for
// a create.
func (m *ContextHubModel) openEditor(editName string) {
	m.armEditor(newCustomContextEditor(m.runtime, editName))
}

func (m *ContextHubModel) armEditor(editor *contextEditor) {
	editor.resize(m.width)
	m.editor = editor
	m.notice = ""
}

func (m *ContextHubModel) closeEditor() {
	m.editor = nil
	m.confirmDiscard = false
	m.discardOverlay.Cancel()
}

// updateEditor owns every key while the inline editor is open. The hub's browse
// keys are unreachable here, so `n`/`d`/`j`/`k` type normally into the name
// field and act as list keys only when the project list holds focus.
func (m *ContextHubModel) updateEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	editor := m.editor
	if editor.destPrompt {
		return m, m.updateDestinationPrompt(msg)
	}
	if editor.searching {
		return m, m.updateEditorSearch(msg)
	}

	switch msg.String() {
	case "esc":
		// Same ladder as the browse list: a live project filter clears first, so
		// Esc never discards an edit the user cannot see all of.
		if editor.query != "" {
			editor.searchInput.SetValue("")
			editor.query = ""
			editor.applyFilter()
			return m, nil
		}
		if editor.listFocus {
			editor.setFocus(false)
			return m, nil
		}
		if editor.hasChanges() {
			m.confirmDiscard = true
			return m, nil
		}
		m.closeEditor()
		return m, nil
	case "ctrl+s":
		return m, m.saveEditor()
	case "tab":
		editor.setFocus(!editor.listFocus)
		return m, nil
	}

	if !editor.listFocus {
		if msg.String() == "enter" {
			editor.setFocus(true)
			return m, nil
		}
		var cmd tea.Cmd
		editor.nameInput, cmd = editor.nameInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "j", "down":
		editor.setCursor(editor.cursor + 1)
	case "k", "up":
		editor.setCursor(editor.cursor - 1)
	case "g", "home":
		editor.setCursor(0)
	case "G", "end":
		editor.setCursor(len(editor.filtered) - 1)
	case "enter", " ", "space":
		editor.toggleCursor()
	case "a":
		for _, project := range editor.filtered {
			editor.selected[project.ID] = true
		}
	case "A":
		for _, project := range editor.filtered {
			delete(editor.selected, project.ID)
		}
	case "/":
		editor.searching = true
		editor.searchInput.SetValue(editor.query)
		editor.searchInput.CursorEnd()
		editor.searchInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *ContextHubModel) updateEditorSearch(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.editor
	switch msg.String() {
	case "esc":
		editor.searching = false
		editor.searchInput.Blur()
		editor.searchInput.SetValue("")
		editor.query = ""
		editor.applyFilter()
		return nil
	case "enter":
		editor.searching = false
		editor.searchInput.Blur()
		return nil
	}
	var cmd tea.Cmd
	editor.searchInput, cmd = editor.searchInput.Update(msg)
	if editor.searchInput.Value() != editor.query {
		editor.query = editor.searchInput.Value()
		editor.applyFilter()
	}
	return cmd
}

// saveEditor commits through ContextMutator, which owns the validation rules
// for both context kinds. Failures stay in the editor so the user can correct
// them.
func (m *ContextHubModel) saveEditor() tea.Cmd {
	if m.editor.isRegular() {
		return m.saveRegularEditor()
	}
	return m.saveCustomEditor()
}

func (m *ContextHubModel) saveCustomEditor() tea.Cmd {
	editor := m.editor
	previous := editor.originalName
	result, err := m.mutator.SaveCustomContext(CustomContextSaveRequest{
		OriginalName:     previous,
		Name:             strings.TrimSpace(editor.nameInput.Value()),
		ActiveProjectIDs: editor.selectedIDs(),
	})
	if err != nil {
		editor.err = err.Error()
		return nil
	}

	saved := CustomContextScope(result.Name)
	if !result.IsNew && m.active == CustomContextScope(previous) {
		m.active = saved
	}
	m.closeEditor()
	m.refreshScopes(saved)
	return func() tea.Msg {
		return CustomContextSavedMsg{
			Name:         result.Name,
			PreviousName: previous,
			IsNew:        result.IsNew,
			Renamed:      result.Renamed,
			ProjectCount: result.ProjectCount,
			WasActive:    m.active == saved,
		}
	}
}

// deleteConfirmedContext routes the confirmed delete to the mutation its kind
// needs: a custom context is dropped outright, a regular label is reassigned.
func (m *ContextHubModel) deleteConfirmedContext() tea.Cmd {
	scope := m.pendingDelete
	m.pendingDelete = ContextScope{}
	if scope.Name == "" {
		return nil
	}
	if scope.IsCustom() {
		return m.deleteCustomContext(scope.Name)
	}
	return m.deleteRegularContext(scope.Name)
}

// deleteCustomContext commits the delete first and only then reads the active
// scope, so a failed commit never leaves the hub pointing at All.
func (m *ContextHubModel) deleteCustomContext(name string) tea.Cmd {
	if err := m.mutator.DeleteCustomContext(name); err != nil {
		m.notice = err.Error()
		return nil
	}

	wasActive := m.active == CustomContextScope(name)
	if wasActive {
		m.active = AllContextScope()
	}
	m.refreshScopes(m.active)
	return func() tea.Msg {
		return CustomContextDeletedMsg{Name: name, WasActive: wasActive}
	}
}

// refreshScopes re-derives the browse list from the mutated machine domain and
// parks the cursor on prefer when it survives, so a save or delete leaves the
// user where they were working.
func (m *ContextHubModel) refreshScopes(prefer ContextScope) {
	m.rebuildScopeSurface(contextScopeListItemID(prefer))
}

func (m *ContextHubModel) selectedScope() (ContextScope, bool) {
	entry, ok := m.selectedScopeHubEntry()
	if !ok || entry.Kind != scopeHubEntryBase {
		return ContextScope{}, false
	}
	return entry.Scope, true
}

// editSelected opens the membership editor for either context kind. All is the
// only scope with nothing to edit: it is derived from the whole catalog.
func (m *ContextHubModel) editSelected() {
	scope, ok := m.selectedScope()
	if !ok {
		return
	}
	switch {
	case scope.IsCustom():
		m.openEditor(scope.Name)
	case scope.Kind == ContextScopeRegular:
		m.openRegularEditor(scope.Name)
	default:
		m.notice = "All spans every project and has no membership to edit"
	}
}

func (m *ContextHubModel) requestDeleteSelected() {
	scope, ok := m.selectedScope()
	if !ok {
		return
	}
	if scope.IsAll() {
		m.notice = "All spans every project and cannot be deleted"
		return
	}
	prompt := fmt.Sprintf("Delete custom context %q?", scope.Name)
	if !scope.IsCustom() {
		if strings.EqualFold(scope.Name, m.runtime.DefaultContext()) {
			m.notice = "The default context cannot be deleted"
			return
		}
		prompt = fmt.Sprintf("Delete context %q? Its projects move to %q.", scope.Name, m.runtime.DefaultContext())
	}
	m.pendingDelete = scope
	m.notice = ""
	m.confirm.Show("delete", prompt)
}

// renameSelected reuses the editor: a rename is the same checked save with a
// changed name, so there is no separate rename prompt to keep in sync with the
// duplicate-name rules. It only differs from edit by starting on the name field.
func (m *ContextHubModel) renameSelected() {
	m.editSelected()
	if m.editor != nil {
		m.editor.setFocus(false)
	}
}

func (m *ContextHubModel) editorView(width, height int) string {
	editor := m.editor
	if editor.destPrompt {
		return m.destinationPromptView(width)
	}

	var b strings.Builder
	b.WriteString(SectionHeaderStyle.Render(m.editorTitle()))
	b.WriteString("\n\n")

	nameLabel := FieldLabelStyle.Render("Name: ")
	if !editor.listFocus {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	}
	b.WriteString(nameLabel + editor.nameInput.View())
	if editor.isRegular() && strings.EqualFold(editor.originalName, m.runtime.DefaultContext()) {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("  This is the default context; renaming it moves the default too."))
	}
	b.WriteString("\n\n")

	projectsLabel := fmt.Sprintf("Projects (%d selected)", len(editor.selected))
	if editor.listFocus {
		b.WriteString(FieldLabelFocusedStyle.Render(projectsLabel))
	} else {
		b.WriteString(FieldLabelStyle.Render(projectsLabel))
	}
	b.WriteString("\n")

	// The list shows active projects only, so members it cannot offer would look
	// like a save that deleted them. Naming the count makes the preservation
	// visible instead of silent.
	if hidden := len(editor.hiddenMembers); hidden > 0 {
		b.WriteString(MutedStyle.Render(truncateToVisualWidth(
			fmt.Sprintf("  + %d hidden member(s) kept on save", hidden), width)))
		b.WriteString("\n")
	}

	switch {
	case editor.searching:
		b.WriteString("  /" + editor.searchInput.View() + "\n")
	case editor.query != "":
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  /%s  (%d/%d)", editor.query, len(editor.filtered), len(editor.projects))))
		b.WriteString("\n")
	}

	b.WriteString(m.editorProjectList(width, height-lipgloss.Height(b.String())-3))
	if editor.err != "" {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(truncateToVisualWidth(editor.err, width)))
	}
	return b.String()
}

func (m *ContextHubModel) editorProjectList(width, height int) string {
	editor := m.editor
	if len(editor.filtered) == 0 {
		if editor.query != "" {
			return renderEmptyState("No projects match the filter.", "Esc clears it.")
		}
		return renderEmptyState("No active projects.", "Create a project before saving a custom context.")
	}

	lines := make([]string, 0, len(editor.filtered))
	for i, project := range editor.filtered {
		checkbox := "[ ]"
		if editor.selected[project.ID] {
			checkbox = "[x]"
		}
		label := project.Name
		if project.Context != "" {
			label += " (" + project.Context + ")"
		}
		if count, ok := m.taskCounts[project.ID]; ok {
			label += fmt.Sprintf(" · %d task(s)", count)
		}
		if editor.listFocus && i == editor.cursor {
			line := IconCursorRight + " " + checkbox + " " + label
			lines = append(lines, SelectionRowStyle.Width(width).Render(truncateToVisualWidth(line, width)))
			continue
		}
		box := MutedStyle.Render(checkbox)
		if editor.selected[project.ID] {
			box = SelectionCheckStyle.Render(checkbox)
		}
		lines = append(lines, truncateToVisualWidth("  "+box+" "+label, width))
	}

	visible, scroll := contextHubWindow(lines, editor.cursor, editor.scroll, max(height, 3))
	editor.scroll = scroll
	return strings.Join(visible, "\n")
}

// editorTitle names the surface by context kind so a regular-label edit is never
// mistaken for a custom-context one, which has different membership semantics.
func (m *ContextHubModel) editorTitle() string {
	editor := m.editor
	switch {
	case editor.isRegular():
		return "Edit Context: " + editor.originalName
	case editor.isNew():
		return "New Custom Context"
	default:
		return "Edit Custom Context: " + editor.originalName
	}
}

func (m *ContextHubModel) editorFooterHelpVariants() []string {
	editor := m.editor
	switch {
	case editor.destPrompt:
		return []string{"j/k choose · Enter move projects there · Esc cancel"}
	case editor.searching:
		return []string{"Type to filter · Enter keep · Esc clear"}
	case editor.listFocus:
		return []string{
			"j/k move · Enter/Space toggle · a/A all/none · / search · Tab name · Ctrl+S save · Esc back",
			"j/k · Space toggle · a/A all/none · / search · Ctrl+S save · Esc back",
			"j/k · Space toggle · a/A all/none · Ctrl+S save",
		}
	default:
		return []string{
			"Type a name · Tab/Enter projects · Ctrl+S save · Esc cancel",
			"Tab projects · Ctrl+S save · Esc cancel",
		}
	}
}

func (m *ContextHubModel) editorHelpItems() []HelpItem {
	save := "Save the custom context"
	if m.editor.isRegular() {
		save = "Save the context label and its membership"
	}
	return []HelpItem{
		{Key: "tab", Description: "Switch between name and project list", Category: "Navigation"},
		{Key: "j/k", Description: "Move through projects", Category: "Navigation"},
		{Key: "enter", Description: "Toggle the highlighted project (or leave the name field)", Category: "Selection"},
		{Key: "space", Description: "Toggle the highlighted project", Category: "Selection"},
		{Key: "a/A", Description: "Select or deselect every filtered project", Category: "Selection"},
		{Key: "/", Description: "Search the project list by name or context", Category: "Filters"},
		{Key: "ctrl+s", Description: save, Category: "Actions"},
		{Key: "esc", Description: "Leave the project list, then discard the edit", Category: "Navigation"},
	}
}
