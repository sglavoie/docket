package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectEditorModel handles editing an existing project via TUI.
type ProjectEditorModel struct {
	project           *models.Project
	configManager     *config.Manager
	runtime           *config.RuntimeState
	appService        *app.Service
	identityProtected bool

	// Original values for change detection
	originalName        string
	originalContext     string
	originalDescription string

	// Form fields
	nameInput        textinput.Model
	descriptionInput textinput.Model

	// Context selection (hybrid: selector + custom input)
	contextInput  textinput.Model
	contexts      []string // Available contexts from existing projects
	contextCursor int      // -1 means custom input mode, 0+ means selecting from list

	// Form state
	focusIndex int // 0=name, 1=context, 2=description
	width      int
	height     int
	err        error
	hasChanges bool

	discardConfirmState
}

// NewProjectEditorModel creates a domain-native project editor. Catalog metadata
// is encoded from RuntimeState and committed atomically with directory/reference
// changes through the application service.
func NewProjectEditorModel(
	project *models.Project,
	manager *config.Manager,
	runtime *config.RuntimeState,
	svc *app.Service,
	identityProtected bool,
) *ProjectEditorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInputWithValue(project.Name, "")
	nameInput.Focus()
	nameInput.CharLimit = 100

	descriptionInput := factory.NewTextInputWithValue(project.Description, "Project description (optional)")
	descriptionInput.CharLimit = 500

	contextInput := factory.NewTextInputWithValue(project.Context, "e.g., personal, work")
	contextInput.CharLimit = 50

	// Get all existing contexts
	contexts := runtime.AllContexts()

	// Find current context in list (or -1 for custom)
	contextCursor := -1
	for i, ctx := range contexts {
		if ctx == project.Context {
			contextCursor = i
			break
		}
	}

	return &ProjectEditorModel{
		project:             project,
		configManager:       manager,
		runtime:             runtime,
		appService:          svc,
		identityProtected:   identityProtected,
		originalName:        project.Name,
		originalContext:     project.Context,
		originalDescription: project.Description,
		nameInput:           nameInput,
		descriptionInput:    descriptionInput,
		contextInput:        contextInput,
		contexts:            contexts,
		contextCursor:       contextCursor,
		focusIndex:          0,
		width:               80,
		height:              24,
	}
}

// checkChanges detects if any field has been modified.
func (m *ProjectEditorModel) checkChanges() bool {
	if strings.TrimSpace(m.nameInput.Value()) != m.originalName {
		return true
	}
	if strings.TrimSpace(m.contextInput.Value()) != m.originalContext {
		return true
	}
	if strings.TrimSpace(m.descriptionInput.Value()) != m.originalDescription {
		return true
	}
	return false
}

// SetSize sets the view dimensions.
func (m *ProjectEditorModel) SetSize(width, height int) {
	const descriptionLabelReserve = 12

	m.width = width
	m.height = height
	formFactory.ResizeTextInputWithReserve(&m.nameInput, width, FormLabelWidth)
	formFactory.ResizeTextInputWithReserve(&m.descriptionInput, width, descriptionLabelReserve)
	formFactory.ResizeTextInputWithReserve(&m.contextInput, width, FormLabelWidth)
}

func (m *ProjectEditorModel) SupportsHelpShortcut() bool { return false }

func (m *ProjectEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ProjectEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle discard confirmation
	if m.confirmDiscard {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg {
					return BackMsg{}
				}
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
			if m.checkChanges() {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (3 fields: name, context, description)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 3
			} else {
				m.focusIndex = (m.focusIndex + 2) % 3
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field or save if on last field
			if m.focusIndex < 2 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Save changes
			return m, m.saveProject()

		case "left", "h":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == 0 {
					// Stay at first context
				} else {
					// From custom mode, go to last context
					m.contextCursor = len(m.contexts) - 1
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}

		case "right", "l":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
					m.contextCursor++
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
				// Stay at last context when at the end
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		oldValue := m.contextInput.Value()
		m.contextInput, cmd = m.contextInput.Update(msg)
		// If user typed something, switch to custom mode
		if m.contextInput.Value() != oldValue {
			m.contextCursor = -1
			// Check if typed value matches an existing context
			for i, ctx := range m.contexts {
				if ctx == m.contextInput.Value() {
					m.contextCursor = i
					break
				}
			}
		}
	case 2:
		m.descriptionInput, cmd = m.descriptionInput.Update(msg)
	}

	// Check for changes
	m.hasChanges = m.checkChanges()

	return m, cmd
}

func (m *ProjectEditorModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.nameInput.Blur()
	case 1:
		m.contextInput.Blur()
	case 2:
		m.descriptionInput.Blur()
	}
}

func (m *ProjectEditorModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.contextInput.Focus()
	case 2:
		m.descriptionInput.Focus()
	}
}

func (m *ProjectEditorModel) saveProject() tea.Cmd {
	name := strings.TrimSpace(m.nameInput.Value())
	contextValue := strings.TrimSpace(m.contextInput.Value())
	description := strings.TrimSpace(m.descriptionInput.Value())
	originalName := m.originalName
	project := m.project
	configManager := m.configManager
	runtime := m.runtime
	svc := m.appService
	identityProtected := m.identityProtected
	return func() tea.Msg {
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("project name cannot be empty")}
		}
		if identityProtected && name != originalName {
			return ErrorMsg{Err: fmt.Errorf("the system Unfiled name and slug are immutable")}
		}
		if configManager == nil || svc == nil {
			return ErrorMsg{Err: fmt.Errorf("project transaction service is unavailable")}
		}

		updated := *project
		newSlug := models.Slugify(name)
		if newSlug == "" {
			return ErrorMsg{Err: fmt.Errorf("project name must contain at least one ASCII letter or digit")}
		}
		oldSlug := project.Slug
		if name != originalName {
			// Check for duplicate slug (excluding self)
			if p := runtime.GetProject(newSlug); p != nil && p.ID != project.ID {
				return ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", newSlug)}
			}
		}

		// Get context value
		ctx := contextValue
		if ctx == "" {
			ctx = runtime.DefaultContext()
		}

		// Prepare on an isolated copy. Publication to the live project pointer
		// happens only after the journal and workspace ledger both commit.
		updated.Name = name
		updated.Slug = newSlug
		updated.Context = ctx
		updated.Description = description
		updated.UpdatedAt = time.Now()

		var tasksMarkdown []byte
		var tasksExpectedRevision *string
		if expected, err := storage.LoadProjectForRewrite(&updated); err == nil {
			content, renderErr := storage.WriteTasksMarkdown(updated.Name, updated.Tasks)
			if renderErr != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to render renamed project: %w", renderErr)}
			}
			tasksMarkdown = []byte(content)
			tasksExpectedRevision = expected
		}

		catalog := cloneWorkspaceCatalog(runtime.Catalog())
		if err := catalog.UpsertProject(&updated); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to update project catalog: %w", err)}
		}
		configArts, err := catalogDomainArtifact(configManager, catalog)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to encode config: %w", err)}
		}

		if _, err := svc.RenameProject(app.RenameProjectRequest{
			Project:                &updated,
			SystemUnfiledProjectID: runtime.Catalog().SystemUnfiledProjectID,
			DataDir:                runtime.DataDir(),
			OldName:                originalName,
			OldSlug:                oldSlug,
			NewSlug:                newSlug,
			TasksMarkdown:          tasksMarkdown,
			TasksExpectedRevision:  tasksExpectedRevision,
			Config:                 configArts,
		}); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		if err := runtime.RefreshAfterCommit(); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to refresh project catalog: %w", err)}
		}

		copyCatalogMetadata(project, &updated)
		return ProjectUpdatedMsg{Project: project}
	}
}

func (m *ProjectEditorModel) View() tea.View {
	var b strings.Builder

	// Name input with slug preview
	b.WriteString(renderFormLabelShared("Name:", 0, m.focusIndex))
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	padding := strings.Repeat(" ", FormLabelWidth)
	if name != "" {
		slug := models.Slugify(name)
		slugChanged := slug != models.Slugify(m.originalName)
		if slugChanged {
			b.WriteString(renderProjectSlugPreview(slug, m.width, padding, " (will change)"))
		} else {
			b.WriteString(renderProjectSlugPreview(slug, m.width, padding, ""))
		}
	}
	b.WriteString("\n\n")

	// Context input with available contexts hint
	b.WriteString(renderFormLabelShared("Context:", 1, m.focusIndex))
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	// Show available contexts using shared selector
	if len(m.contexts) > 0 {
		b.WriteString(padding)
		b.WriteString(renderContextSelectorShared(m.contexts, m.contextCursor, m.focusIndex == 1))
	}
	b.WriteString("\n\n")

	// Description input
	b.WriteString(renderFormLabelShared("Description:", 2, m.focusIndex))
	b.WriteString(m.descriptionInput.View())
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Build title
	title := "Edit Project"
	if m.hasChanges {
		title += " *"
	}

	// Footer
	left := HelpStyle.Render("Tab/Shift+Tab switch · ←/→ context · Ctrl+S save · Esc cancel")
	right := MutedStyle.Render(title)

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}

func (m *ProjectEditorModel) HasTextInput() bool     { return true }
func (m *ProjectEditorModel) ShouldCaptureEsc() bool { return true }
func (m *ProjectEditorModel) IsFullHeight() bool     { return false }

func (m *ProjectEditorModel) StatusBarLabel() string { return "Edit Project" }
