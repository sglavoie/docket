package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var blockersTabNames = []string{"Active", "Resolved", "All"}

const (
	blockersTabActive   = 0
	blockersTabResolved = 1
	blockersTabAll      = 2
)

// blockerItem is a unified view of a blocker with its project context.
type blockerItem struct {
	Blocker *models.Blocker
	Project *models.Project
}

// blockerViewState holds all the shared UI state for blocker views.
type blockerViewState struct {
	cursor int
	tabs   TabState

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Confirmation state
	confirm ConfirmDialog

	// Expanded view mode
	expandedView bool

	// Editor config
	editor string

	// Dimensions
	width  int
	height int

	// Error display
	err error
}

func newBlockerViewState(editor string) blockerViewState {
	ti := textarea.New()
	ti.Placeholder = "Enter blocker title..."
	ti.CharLimit = 500
	ti.SetHeight(2)
	ti.ShowLineNumbers = false

	editTi := textarea.New()
	editTi.Placeholder = "Edit blocker..."
	editTi.CharLimit = 2000
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false

	return blockerViewState{
		tabs:          TabState{Names: blockersTabNames},
		editor:        editor,
		textInput:     ti,
		editTextInput: editTi,
	}
}

func (s *blockerViewState) GetConfirmDialog() *ConfirmDialog { return &s.confirm }

func (s *blockerViewState) setSize(width, height int) {
	s.width = width
	s.height = height
	s.textInput.SetWidth(max(width-4, 1))
	s.editTextInput.SetWidth(max(width-4, 1))
}

func (s *blockerViewState) hasTextInput() bool {
	return s.addMode || s.editMode
}

func (s *blockerViewState) shouldCaptureEsc() bool {
	return s.expandedView || s.confirm.IsActive() || s.addMode || s.editMode
}

// blockerDataSource abstracts the differences between project-local and global blocker views.
type blockerDataSource interface {
	// getVisibleItems returns the visible blocker items for the current tab.
	getVisibleItems(tab int) []blockerItem
	// activeCount returns the number of active blockers.
	activeCount() int
	// resolvedCount returns the number of resolved blockers.
	resolvedCount() int
	// findItemIndex finds the index of a blocker in the underlying data.
	findItemIndex(blocker *models.Blocker) int
	// saveBlockerData saves a new blocker to the appropriate project.
	saveBlockerData(blocker *models.Blocker, project *models.Project) error
	// deleteBlockerData removes a blocker and saves.
	deleteBlockerData(idx int) (*models.Project, error)
	// updateBlockerData saves changes after edit/resolve.
	updateBlockerData(idx int) (*models.Project, error)
	// getAddProject returns the project to use when adding a new blocker.
	getAddProject() *models.Project
	// getItemBlocker returns the blocker at the given data index.
	getItemBlocker(idx int) *models.Blocker
	// addBlocker appends a blocker to the data source.
	addBlocker(blocker *models.Blocker, project *models.Project)
	// titleText returns the view title.
	titleText() string
	// showProjectName returns true if project names should be shown.
	showProjectName() bool
	// closeMsg returns the message to send when closing.
	closeMsg() tea.Msg
}

// blockerUpdateShared handles the Update logic shared between both blocker views.
func blockerUpdateShared(s *blockerViewState, ds blockerDataSource, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	var cmd tea.Cmd

	// Handle expanded view mode
	if s.expandedView {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "esc", "enter", "q":
				s.expandedView = false
			}
		}
		return nil, nil, true
	}

	// Handle confirmation mode
	if s.confirm.IsActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			if confirmed, handled := s.confirm.HandleKey(msg.String()); handled {
				if confirmed {
					return nil, blockerExecuteDelete(s, ds), true
				}
				s.err = nil
				return nil, nil, true
			}
		}
		return nil, nil, true
	}

	// Handle edit mode
	if s.editMode {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "esc":
				s.editMode = false
				s.editTextInput.Reset()
				return nil, nil, true
			case "ctrl+s":
				return nil, blockerSaveEdited(s, ds), true
			case "ctrl+e":
				return nil, blockerEditInExternalEditor(s, ds, true), true
			}
		}
		s.editTextInput, cmd = s.editTextInput.Update(msg)
		return nil, cmd, true
	}

	// Handle add mode
	if s.addMode {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "esc":
				s.addMode = false
				s.textInput.Reset()
				return nil, nil, true
			case "ctrl+s":
				return nil, blockerSaveNew(s, ds), true
			case "ctrl+e":
				return nil, blockerAddInExternalEditor(s, ds), true
			}
		}
		s.textInput, cmd = s.textInput.Update(msg)
		return nil, cmd, true
	}

	// Normal mode key handling
	if msg, ok := msg.(tea.KeyMsg); ok {
		visible := ds.getVisibleItems(s.tabs.Active)

		switch msg.String() {
		case "q", "esc":
			return nil, func() tea.Msg { return ds.closeMsg() }, true

		case "j", "down":
			if len(visible) > 0 && s.cursor < len(visible)-1 {
				s.cursor++
			}
			return nil, nil, true

		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
			return nil, nil, true

		case "g":
			if len(visible) > 0 && s.cursor < len(visible) {
				return nil, func() tea.Msg {
					return BlockerGoToProjectMsg{Project: visible[s.cursor].Project}
				}, true
			}
			return nil, nil, true

		case "G":
			if len(visible) > 0 {
				s.cursor = len(visible) - 1
			}
			return nil, nil, true

		case "a", "n":
			s.addMode = true
			s.textInput.Focus()
			return nil, textarea.Blink, true

		case "e":
			if len(visible) > 0 && s.cursor < len(visible) {
				return nil, blockerStartEdit(s, ds, visible[s.cursor]), true
			}
			return nil, nil, true

		case "E":
			if len(visible) > 0 && s.cursor < len(visible) {
				s.editIndex = ds.findItemIndex(visible[s.cursor].Blocker)
				return nil, blockerEditInExternalEditor(s, ds, false), true
			}
			return nil, nil, true

		case "R":
			if len(visible) > 0 && s.cursor < len(visible) {
				return nil, blockerToggleResolve(s, ds, visible[s.cursor]), true
			}
			return nil, nil, true

		case "d", "x":
			if len(visible) > 0 && s.cursor < len(visible) {
				s.confirm.Show("delete", "Delete blocker?")
			}
			return nil, nil, true

		case "[", "h", "]", "l":
			if handleTabCycleKey(msg.String(), &s.tabs, true, func() {
				blockerClampCursor(s, ds)
			}) {
				return nil, nil, true
			}

		case "enter":
			if len(visible) > 0 && s.cursor < len(visible) {
				s.expandedView = true
			}
			return nil, nil, true
		}
	}

	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		s.width = msg.Width
		s.height = msg.Height
		return nil, nil, true
	}

	return nil, nil, false
}

func blockerClampCursor(s *blockerViewState, ds blockerDataSource) {
	visible := ds.getVisibleItems(s.tabs.Active)
	if s.cursor >= len(visible) {
		s.cursor = max(len(visible)-1, 0)
	}
}

func blockerSaveNew(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	return func() tea.Msg {
		title := strings.TrimSpace(s.textInput.Value())
		if title == "" {
			s.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		project := ds.getAddProject()
		if project == nil {
			s.err = fmt.Errorf("no project available")
			return nil
		}

		blocker := models.NewBlocker(title)
		ds.addBlocker(blocker, project)

		if err := ds.saveBlockerData(blocker, project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		s.addMode = false
		s.textInput.Reset()
		s.err = nil
		s.cursor = max(ds.activeCount()-1, 0)

		return BlockerAddedMsg{Project: project}
	}
}

func blockerExecuteDelete(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	return func() tea.Msg {
		visible := ds.getVisibleItems(s.tabs.Active)
		if s.cursor >= len(visible) {
			return nil
		}

		item := visible[s.cursor]
		idx := ds.findItemIndex(item.Blocker)
		if idx < 0 {
			return nil
		}

		project, err := ds.deleteBlockerData(idx)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		visible = ds.getVisibleItems(s.tabs.Active)
		if s.cursor >= len(visible) && s.cursor > 0 {
			s.cursor--
		}

		return BlockerDeletedMsg{Project: project}
	}
}

func blockerStartEdit(s *blockerViewState, ds blockerDataSource, item blockerItem) tea.Cmd {
	s.editMode = true
	s.editIndex = ds.findItemIndex(item.Blocker)

	content := item.Blocker.Title
	if item.Blocker.Description != "" {
		content += "\n\n" + item.Blocker.Description
	}
	s.editTextInput.SetValue(content)
	s.editTextInput.Focus()
	return textarea.Blink
}

func blockerSaveEdited(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	return func() tea.Msg {
		content := strings.TrimSpace(s.editTextInput.Value())
		if content == "" {
			s.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		lines := strings.SplitN(content, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			s.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		blocker := ds.getItemBlocker(s.editIndex)
		blocker.Title = title
		blocker.Description = description

		project, err := ds.updateBlockerData(s.editIndex)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		s.editMode = false
		s.editTextInput.Reset()
		s.err = nil

		return BlockerEditedMsg{Project: project}
	}
}

func blockerToggleResolve(s *blockerViewState, ds blockerDataSource, item blockerItem) tea.Cmd {
	return func() tea.Msg {
		idx := ds.findItemIndex(item.Blocker)
		if idx < 0 {
			return nil
		}

		blocker := ds.getItemBlocker(idx)
		if blocker.IsResolved() {
			blocker.Unresolve()
		} else {
			blocker.Resolve()
		}

		project, err := ds.updateBlockerData(idx)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		if s.tabs.Active == blockersTabActive || s.tabs.Active == blockersTabResolved {
			visible := ds.getVisibleItems(s.tabs.Active)
			if s.cursor >= len(visible) && s.cursor > 0 {
				s.cursor--
			}
		}

		return BlockerEditedMsg{Project: project}
	}
}

// blockerBuildEditorCmd creates an exec.Cmd for the configured editor.
func blockerBuildEditorCmd(editor, tmpPath string) *exec.Cmd {
	editor = external.ResolveEditor(editor)
	return external.EditorCommand(editor, tmpPath)
}

func blockerEditInExternalEditor(s *blockerViewState, ds blockerDataSource, fromEditMode bool) tea.Cmd {
	blocker := ds.getItemBlocker(s.editIndex)

	var content string
	if fromEditMode {
		// Use current edit text input content
		content = fmt.Sprintf("# Blocker: %s\n\n", blocker.Title)
		content += "<!-- Edit the title on the first line below, then description after a blank line. -->\n\n"
		content += strings.TrimSpace(s.editTextInput.Value())
	} else {
		content = fmt.Sprintf("# Blocker: %s\n\n", blocker.Title)
		content += "<!-- Edit the title on the first line below, then description after a blank line. -->\n\n"
		content += blocker.Title
		if blocker.Description != "" {
			content += "\n\n" + blocker.Description
		}
	}

	tmpFile, err := os.CreateTemp("", "tt-blocker-*.md")
	if err != nil {
		s.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	c := blockerBuildEditorCmd(s.editor, tmpPath)

	editIndex := s.editIndex
	originalTitle := blocker.Title
	originalDesc := blocker.Description

	if fromEditMode {
		s.editMode = false
		s.editTextInput.Reset()
	}

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		newContent := util.ParseEditedCommentContent(string(modified))
		if newContent == "" {
			return ErrorMsg{Err: fmt.Errorf("blocker title cannot be empty")}
		}

		lines := strings.SplitN(newContent, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			return ErrorMsg{Err: fmt.Errorf("blocker title cannot be empty")}
		}

		if title == originalTitle && description == originalDesc {
			return nil
		}

		b := ds.getItemBlocker(editIndex)
		b.Title = title
		b.Description = description

		project, saveErr := ds.updateBlockerData(editIndex)
		if saveErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", saveErr)}
		}

		return BlockerEditedMsg{Project: project}
	})
}

func blockerAddInExternalEditor(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	project := ds.getAddProject()
	if project == nil {
		s.err = fmt.Errorf("no project available")
		return nil
	}

	currentText := strings.TrimSpace(s.textInput.Value())

	content := fmt.Sprintf("# New Blocker for %s\n\n", project.Name)
	content += "<!-- Enter the blocker title on the first line, then description after a blank line. -->\n\n"
	content += currentText

	tmpFile, err := os.CreateTemp("", "tt-blocker-*.md")
	if err != nil {
		s.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	c := blockerBuildEditorCmd(s.editor, tmpPath)

	s.addMode = false
	s.textInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		newContent := util.ParseEditedCommentContent(string(modified))
		if newContent == "" {
			return nil
		}

		lines := strings.SplitN(newContent, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			return nil
		}

		blocker := models.NewBlocker(title)
		blocker.Description = description
		ds.addBlocker(blocker, project)

		if saveErr := ds.saveBlockerData(blocker, project); saveErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", saveErr)}
		}

		return BlockerAddedMsg{Project: project}
	})
}

// blockerViewShared renders the shared blocker view.
func blockerViewShared(s *blockerViewState, ds blockerDataSource) string {
	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle(ds.titleText(), fmt.Sprintf("%d active, %d resolved", ds.activeCount(), ds.resolvedCount())))
	b.WriteString("\n\n")

	// Expanded view mode
	if s.expandedView {
		visible := ds.getVisibleItems(s.tabs.Active)
		if len(visible) > 0 && s.cursor < len(visible) {
			item := visible[s.cursor]
			blocker := item.Blocker

			b.WriteString(FieldLabelFocusedStyle.Render(blocker.Title))
			b.WriteString("\n")
			if ds.showProjectName() {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", item.Project.Name)))
				b.WriteString("\n")
			}
			b.WriteString(MutedStyle.Render(FormatTimestampLabeled("Created", blocker.CreatedAt)))
			if blocker.IsResolved() {
				b.WriteString("\n")
				b.WriteString(DoneStyle.Render(FormatTimestampLabeled("Resolved", *blocker.ResolvedAt)))
			}
			b.WriteString("\n\n")

			if blocker.Description != "" {
				renderWidth := max(s.width-4, 40)
				b.WriteString(renderMarkdown(blocker.Description, renderWidth))
				b.WriteString("\n")
			} else {
				b.WriteString(MutedStyle.Render("No description."))
				b.WriteString("\n")
			}

			b.WriteString("\n")
			b.WriteString(HelpStyle.Render("Enter/Esc: close expanded view"))
		}
		return b.String()
	}

	// Edit mode
	if s.editMode {
		blocker := ds.getItemBlocker(s.editIndex)
		b.WriteString(FieldLabelFocusedStyle.Render("Edit Blocker:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(FormatTimestampLabeled("Created", blocker.CreatedAt)))
		b.WriteString("\n\n")
		b.WriteString(s.editTextInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"))
		return b.String()
	}

	// Add mode
	if s.addMode {
		b.WriteString(FieldLabelFocusedStyle.Render("Add Blocker:"))
		b.WriteString("\n")
		if ds.showProjectName() {
			project := ds.getAddProject()
			if project != nil {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", project.Name)))
				b.WriteString("\n")
			}
		}
		b.WriteString(s.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"))
		return b.String()
	}

	// Error display
	if s.err != nil {
		b.WriteString(ValidationErrorStyle.Render(s.err.Error()))
		b.WriteString("\n\n")
	}

	visible := ds.getVisibleItems(s.tabs.Active)

	// Tab bar
	b.WriteString(RenderTabBar(blockersTabNames, s.tabs.Active, min(s.width-6, 54)-2, 1))
	b.WriteString("\n")

	if len(visible) == 0 {
		switch s.tabs.Active {
		case blockersTabActive:
			b.WriteString(MutedStyle.Render("No active blockers. Press 'a' to add one."))
		case blockersTabResolved:
			b.WriteString(MutedStyle.Render("No resolved blockers."))
		default:
			b.WriteString(MutedStyle.Render("No blockers. Press 'a' to add one."))
		}
		b.WriteString("\n")
	} else {
		headerLines := 7
		availableHeight := s.height - headerLines
		maxVisible := max(availableHeight/3, 3)

		scrollOffset := calcScrollOffset(s.cursor, maxVisible)

		for i := scrollOffset; i < len(visible) && i < scrollOffset+maxVisible; i++ {
			item := visible[i]
			blocker := item.Blocker

			cursor := "  "
			if i == s.cursor {
				cursor = "> "
			}

			relativeTime := storage.FormatRelativeTime(blocker.CreatedAt)

			var line strings.Builder
			line.WriteString(cursor)

			if i == s.cursor {
				if blocker.IsResolved() {
					line.WriteString(DoneStyle.Render("✓ "))
					line.WriteString(MutedStyle.Render(blocker.Title))
				} else {
					line.WriteString(DueSoonStyle.Render("🚧 "))
					line.WriteString(BoldStyle.Render(blocker.Title))
				}
			} else {
				if blocker.IsResolved() {
					line.WriteString(DoneStyle.Render("✓ "))
					line.WriteString(MutedStyle.Render(blocker.Title))
				} else {
					line.WriteString(DueSoonStyle.Render("🚧 "))
					line.WriteString(blocker.Title)
				}
			}
			line.WriteString("\n")

			if ds.showProjectName() {
				fmt.Fprintf(&line, "      %s", MutedStyle.Render(fmt.Sprintf("[%s] %s", item.Project.Name, relativeTime)))
			} else {
				fmt.Fprintf(&line, "      %s", MutedStyle.Render(relativeTime))
			}

			if blocker.Description != "" {
				desc := blocker.Description
				maxLen := max(s.width-10, 30)
				desc = strings.ReplaceAll(desc, "\n", " ")
				if len(desc) > maxLen {
					desc = desc[:maxLen-3] + "..."
				}
				line.WriteString(" - ")
				line.WriteString(MutedStyle.Render(desc))
			}
			line.WriteString("\n")

			b.WriteString(line.String())
		}

		if len(visible) > maxVisible {
			remaining := len(visible) - scrollOffset - maxVisible
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	helpText := "a: add · e: edit · R: resolve/unresolve · d: delete · g: go to project · Enter: expand · h/l/[/] tabs · Esc: close"
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, s.width)))

	return b.String()
}
