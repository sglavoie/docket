package taskform

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// createProjectConfirmAction is the action the offer's question is shown
// under; the shell renders it as the ordinary confirm dialog.
const createProjectConfirmAction = "create-project"

// composerProjectOffer is what a refused submit offers when the only thing
// wrong with the draft is projects nobody holds yet: the projects to create and
// the submit to repeat once they exist. It holds the requests the draft parses
// to with those projects in place, so Yes commits exactly what was asked about
// and the draft itself is never rewritten.
type composerProjectOffer struct {
	slugs        []string
	projects     []app.NewProject
	placeholders []*models.Project
	reqs         []app.CaptureRequest
	openContent  bool
	keepOpen     bool
}

func (o *composerProjectOffer) prompt() string {
	noun := "project"
	if len(o.slugs) > 1 {
		noun = "projects"
	}
	return "Create " + noun + " " + strings.Join(o.slugs, ", ") + " and capture?"
}

// offerProjectCreation arms the offer after a refused submit, and only when
// every refusal names an unknown %project already spelled as a canonical slug
// that no project, archived ones included, holds, and the draft parses whole
// once those projects exist. Anything else is refused exactly as before.
func (m *TaskComposerModel) offerProjectCreation(openContent, keepOpen bool) {
	m.projectOffer = nil
	if m.runtime == nil || m.Preset.Project == nil || len(m.Errors) == 0 {
		return
	}
	var slugs []string
	for _, problem := range m.Errors {
		slug, ok := problem.CreatableProject()
		if !ok || m.runtime.GetProject(slug) != nil || m.presetHoldsSlug(slug) {
			return
		}
		if !slices.Contains(slugs, slug) {
			slugs = append(slugs, slug)
		}
	}
	offer := &composerProjectOffer{slugs: slugs, openContent: openContent, keepOpen: keepOpen}
	for _, slug := range slugs {
		id := uuid.New().String()
		placeholder, err := models.NewProject(id, slug, m.runtime.ProjectPath(slug))
		if err != nil {
			return
		}
		placeholder.NextShortID = 1
		offer.projects = append(offer.projects, app.NewProject{ID: id, Name: slug})
		offer.placeholders = append(offer.placeholders, placeholder)
	}
	projects := append(slices.Clone(m.Preset.AllProjects), offer.placeholders...)
	reqs, problems := capturetext.Requests(m.Input.Value(), m.Preset.Project, projects, m.captureDefaults())
	if len(problems) > 0 {
		return
	}
	offer.reqs = reqs
	m.projectOffer = offer
}

func (m *TaskComposerModel) presetHoldsSlug(slug string) bool {
	for _, project := range append([]*models.Project{m.Preset.Project}, m.Preset.AllProjects...) {
		if project != nil && project.Slug == slug {
			return true
		}
	}
	return false
}

// updateProjectOffer answers the offer. Yes repeats the submit with the
// projects created in the same transaction; No and Esc put the question down
// and leave the draft and its refusal as they were. Every other key waits.
func (m *TaskComposerModel) updateProjectOffer(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "y", "Y":
		offer := m.projectOffer
		m.projectOffer = nil
		return m, m.submitProjectOffer(offer)
	case "n", "N", "esc":
		m.projectOffer = nil
	}
	return m, nil
}

func (m *TaskComposerModel) submitProjectOffer(offer *composerProjectOffer) tea.Cmd {
	if offer == nil || m.SubmissionInFlight {
		return nil
	}
	cmd := m.capture.CaptureCreatingProjects(m.runtime, offer.projects, offer.reqs, offer.openContent)
	if cmd == nil {
		return nil
	}
	m.pendingCreatedProjects = offer.placeholders
	return m.markSubmission(cmd, offer.keepOpen)
}

// adoptCreatedProjects lets a composer that stays open route to the projects
// its last capture created, so the next line naming one resolves instead of
// being offered a second creation.
func (m *TaskComposerModel) adoptCreatedProjects() {
	if len(m.pendingCreatedProjects) == 0 {
		return
	}
	m.Preset.AllProjects = append(slices.Clone(m.Preset.AllProjects), m.pendingCreatedProjects...)
	m.SetAllProjects(m.Preset.AllProjects)
	m.pendingCreatedProjects = nil
}

func (m *TaskComposerModel) projectOfferDialog() *chrome.ConfirmDialog {
	m.ConfirmOverlay.Show(createProjectConfirmAction, m.projectOffer.prompt())
	return &m.ConfirmOverlay
}
