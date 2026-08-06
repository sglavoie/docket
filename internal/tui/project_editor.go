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

// projectEditorField names one editable row. The editor cycles, focuses, routes
// input, and renders by this enum so a field can never be taught to one switch
// and missed by another.
type projectEditorField int

const (
	projectEditorFieldName projectEditorField = iota
	projectEditorFieldDescription
	projectEditorFieldTags
	projectEditorFieldCount
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
	originalDescription string
	originalTags        string

	// Form fields
	nameInput        textinput.Model
	descriptionInput textinput.Model
	tagsInput        textinput.Model

	// Form state
	focusIndex projectEditorField
	width      int
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

	tags := strings.Join(project.Tags, ", ")
	tagsInput := factory.NewTextInputWithValue(tags, "Comma-separated tags (optional)")
	tagsInput.CharLimit = 200

	return &ProjectEditorModel{
		project:             project,
		configManager:       manager,
		runtime:             runtime,
		appService:          svc,
		identityProtected:   identityProtected,
		originalName:        project.Name,
		originalDescription: project.Description,
		originalTags:        tags,
		nameInput:           nameInput,
		descriptionInput:    descriptionInput,
		tagsInput:           tagsInput,
		focusIndex:          projectEditorFieldName,
		width:               80,
	}
}

// parseProjectEditorTags reads the comma-separated tag field into a canonical
// project tag set. Validation runs here so a rejected tag is reported on the
// form instead of surfacing later as a catalog load failure.
func parseProjectEditorTags(raw string) ([]string, error) {
	fields := strings.Split(raw, ",")
	tags := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return models.NormalizeProjectTags(tags)
}

// checkChanges detects if any field has been modified.
func (m *ProjectEditorModel) checkChanges() bool {
	if strings.TrimSpace(m.nameInput.Value()) != m.originalName {
		return true
	}
	if strings.TrimSpace(m.descriptionInput.Value()) != m.originalDescription {
		return true
	}
	return strings.TrimSpace(m.tagsInput.Value()) != m.originalTags
}

// SetSize sets the view dimensions.
func (m *ProjectEditorModel) SetSize(width, height int) {
	const descriptionLabelReserve = 12

	m.width = width
	formFactory.ResizeTextInputWithReserve(&m.nameInput, width, FormLabelWidth)
	formFactory.ResizeTextInputWithReserve(&m.descriptionInput, width, descriptionLabelReserve)
	formFactory.ResizeTextInputWithReserve(&m.tagsInput, width, descriptionLabelReserve)
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

			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % projectEditorFieldCount
			} else {
				m.focusIndex = (m.focusIndex + projectEditorFieldCount - 1) % projectEditorFieldCount
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field or save if on last field
			if m.focusIndex < projectEditorFieldTags {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Save changes
			return m, m.saveProject()
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case projectEditorFieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case projectEditorFieldDescription:
		m.descriptionInput, cmd = m.descriptionInput.Update(msg)
	case projectEditorFieldTags:
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	case projectEditorFieldCount:
	}

	// Check for changes
	m.hasChanges = m.checkChanges()

	return m, cmd
}

func (m *ProjectEditorModel) blurCurrentField() {
	switch m.focusIndex {
	case projectEditorFieldName:
		m.nameInput.Blur()
	case projectEditorFieldDescription:
		m.descriptionInput.Blur()
	case projectEditorFieldTags:
		m.tagsInput.Blur()
	case projectEditorFieldCount:
	}
}

func (m *ProjectEditorModel) focusCurrentField() {
	switch m.focusIndex {
	case projectEditorFieldName:
		m.nameInput.Focus()
	case projectEditorFieldDescription:
		m.descriptionInput.Focus()
	case projectEditorFieldTags:
		m.tagsInput.Focus()
	case projectEditorFieldCount:
	}
}

// projectEditorSave is one editor save resolved down to the values it will
// write. Resolving before anything commits is what lets the shell hold a parking
// edit behind the shared confirmation and then commit the identical metadata.
type projectEditorSave struct {
	project      *models.Project
	runtime      *config.RuntimeState
	appService   *app.Service
	originalName string
	name         string
	newSlug      string
	description  string
	tags         []string
	parks        bool
}

func (m *ProjectEditorModel) saveProject() tea.Cmd {
	save, err := m.resolveSave()
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	if save.parks {
		// The park confirmation, its cursor anchor, and its recovery status are
		// shell-owned, so a parking save is handed over whole instead of writing
		// here and leaving the user to discover the disappearance.
		return func() tea.Msg { return projectEditorParkRequestMsg{save: save} }
	}
	return save.commit
}

// resolveSave validates every field the save depends on before anything is
// written, so a parking edit raises the confirmation only once it is certain to
// commit and a rejected edit never prompts at all.
func (m *ProjectEditorModel) resolveSave() (*projectEditorSave, error) {
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	tags, err := parseProjectEditorTags(m.tagsInput.Value())
	if err != nil {
		return nil, fmt.Errorf("project tags: %w", err)
	}
	if m.identityProtected {
		if name != m.originalName {
			return nil, fmt.Errorf("the system Unfiled name and slug are immutable")
		}
		if models.ProjectTagsParked(tags) {
			return nil, fmt.Errorf("cannot park the system Unfiled project")
		}
	}
	if m.configManager == nil || m.appService == nil {
		return nil, fmt.Errorf("project transaction service is unavailable")
	}
	newSlug := models.Slugify(name)
	if newSlug == "" {
		return nil, fmt.Errorf("project name must contain at least one ASCII letter or digit")
	}
	if name != m.originalName {
		// Check for duplicate slug (excluding self)
		if p := m.runtime.GetProject(newSlug); p != nil && p.ID != m.project.ID {
			return nil, fmt.Errorf("project with slug '%s' already exists", newSlug)
		}
	}
	return &projectEditorSave{
		project:      m.project,
		runtime:      m.runtime,
		appService:   m.appService,
		originalName: m.originalName,
		name:         name,
		newSlug:      newSlug,
		description:  strings.TrimSpace(m.descriptionInput.Value()),
		tags:         tags,
		parks:        models.ProjectTagsParked(tags) && !models.ProjectTagsParked(m.project.Tags),
	}, nil
}

func (s *projectEditorSave) commit() tea.Msg {
	project := s.project
	oldSlug := project.Slug

	// Prepare on an isolated copy. Publication to the live project pointer
	// happens only after the journal and workspace ledger both commit.
	updated := *project
	updated.Name = s.name
	updated.Slug = s.newSlug
	updated.Description = s.description
	updated.Tags = s.tags
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

	catalog := cloneWorkspaceCatalog(s.runtime.Catalog())
	if err := catalog.UpsertProject(&updated); err != nil {
		return ErrorMsg{Err: fmt.Errorf("failed to update project catalog: %w", err)}
	}

	// One artifact set, one transaction: the directory move, the tasks.md
	// rewrite, and the catalog record either all commit or none of them do.
	configArts, err := catalogDomainArtifact(s.runtime, catalog)
	if err != nil {
		return ErrorMsg{Err: fmt.Errorf("failed to encode config: %w", err)}
	}

	if err := s.appService.RenameProject(app.RenameProjectRequest{
		Project:                &updated,
		SystemUnfiledProjectID: s.runtime.Catalog().SystemUnfiledProjectID,
		DataDir:                s.runtime.DataDir(),
		OldName:                s.originalName,
		OldSlug:                oldSlug,
		NewSlug:                s.newSlug,
		TasksMarkdown:          tasksMarkdown,
		TasksExpectedRevision:  tasksExpectedRevision,
		Config:                 configArts,
	}); err != nil {
		return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
	}
	if err := s.runtime.RefreshAfterCommit(); err != nil {
		return ErrorMsg{Err: fmt.Errorf("failed to refresh project catalog: %w", err)}
	}

	copyCatalogMetadata(project, &updated)
	return ProjectUpdatedMsg{Project: project, Parked: s.parks}
}

func (m *ProjectEditorModel) View() tea.View {
	var b strings.Builder

	// Name input with slug preview
	b.WriteString(renderFormLabelShared("Name:", int(projectEditorFieldName), int(m.focusIndex)))
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

	// Description input
	b.WriteString(renderFormLabelShared("Description:", int(projectEditorFieldDescription), int(m.focusIndex)))
	b.WriteString(m.descriptionInput.View())
	b.WriteString("\n\n")

	// Tags input. The behavioral explainer sits under the field because typing
	// one of these words changes what the project does, and the form is the one
	// place that offers them without a picker to say so.
	b.WriteString(renderFormLabelShared("Tags:", int(projectEditorFieldTags), int(m.focusIndex)))
	b.WriteString(m.tagsInput.View())
	b.WriteString("\n")
	if explainer := behavioralProjectTagExplainer(
		ValueProviderProjectTags,
		projectEditorExplainerWidth(m.width),
	); explainer != "" {
		for _, line := range strings.Split(explainer, "\n") {
			b.WriteString(padding + line + "\n")
		}
	}
	b.WriteString("\n")

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
	left := HelpStyle.Render("Tab/Shift+Tab switch · Ctrl+S save · Esc cancel")
	right := MutedStyle.Render(title)

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}

// projectEditorExplainerWidth is the width the Tags explainer is fitted to: the
// form width left of the label gutter.
func projectEditorExplainerWidth(width int) int {
	return width - FormLabelWidth
}

func (m *ProjectEditorModel) HasTextInput() bool     { return true }
func (m *ProjectEditorModel) ShouldCaptureEsc() bool { return true }
func (m *ProjectEditorModel) IsFullHeight() bool     { return false }

func (m *ProjectEditorModel) StatusBarLabel() string { return "Edit Project" }
