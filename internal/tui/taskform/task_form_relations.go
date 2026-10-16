package taskform

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

type formRelationKind int

const (
	formRelationDefer formRelationKind = iota
	formRelationChildren
	formRelationDependencies
)

func (kind formRelationKind) label() string {
	switch kind {
	case formRelationDefer:
		return "Defer until"
	case formRelationChildren:
		return "Adopt children"
	case formRelationDependencies:
		return "Dependencies"
	}
	return ""
}

func (m *TaskFormModel) relationProject(id string) *models.Project {
	if m.project != nil && m.project.ID == id {
		return m.project
	}
	for _, project := range m.allProjects {
		if project != nil && project.ID == id {
			return project
		}
	}
	return nil
}

// Keep canonical identities in the draft. Display labels and editable short
// references are derived from them, never re-parsed on an unchanged save.
func (m *TaskFormModel) relationReference(ref app.TaskLocator) (token, label string) {
	token = ref.ProjectID + ":" + ref.TaskID
	label = token
	if project := m.relationProject(ref.ProjectID); project != nil {
		if task := project.GetTaskByID(ref.TaskID); task != nil {
			token = fmt.Sprint(task.ShortID)
			if project.ID != m.project.ID {
				token = project.Slug + ":" + token
			}
			label = token + " " + task.Title
		}
	}
	return token, label
}

func (m *TaskFormModel) relationValue(kind formRelationKind, editing bool) string {
	if kind == formRelationDefer {
		if m.draft.deferredUntil == nil {
			return ""
		}
		if editing {
			return m.draft.deferredUntil.Format(time.RFC3339Nano)
		}
		return m.draft.deferredUntil.Format("Mon Jan 2, 2006 15:04 MST")
	}
	refs := m.draft.dependencies
	if kind == formRelationChildren {
		refs = make([]app.TaskLocator, len(m.draft.childIDs))
		for i, id := range m.draft.childIDs {
			refs[i] = app.TaskLocator{ProjectID: m.project.ID, TaskID: id}
		}
	}
	values := make([]string, 0, len(refs))
	for _, ref := range refs {
		token, label := m.relationReference(ref)
		if editing {
			values = append(values, token)
		} else {
			values = append(values, label)
		}
	}
	if editing {
		return strings.Join(values, " ")
	}
	return strings.Join(values, " · ")
}

func (m *TaskFormModel) renderRelationField(b *formBodyBuilder, index int, kind formRelationKind, width int) {
	b.field(index)
	label := kind.label() + ":"
	if m.focusIndex == index {
		label += " Enter edit"
	}
	b.WriteString(chrome.RenderFormLabelWithWidth(label, min(width, 32), m.focusIndex == index) + "\n")
	value := m.relationValue(kind, false)
	if value == "" {
		value = "(none)"
	}
	for _, line := range textfit.WrapStyled(value, max(width-4, 1)) {
		b.WriteString("  " + line + "\n")
	}
	if kind == formRelationDefer && m.draft.deferredUntil != nil {
		b.WriteString(theme.MutedStyle.Render("  Deferred work saves to Backlog, without Today or Next.") + "\n")
	}
}

func (m *TaskFormModel) openRelationEditor(kind formRelationKind) tea.Cmd {
	m.relationKind = kind
	m.relationError = nil
	m.relationInput = textinput.New()
	m.relationInput.SetValue(m.relationValue(kind, true))
	m.activeOverlay = formOverlayRelation
	return m.relationInput.Focus()
}

func (m *TaskFormModel) renderRelationEditor() string {
	title := theme.TitleStyle.Render(m.relationKind.label())
	width := formOverlayInnerWidth(m.width, max(m.width*2/3, 40), formOverlayTitleMinInner(title))
	m.relationInput.SetWidth(max(width-5, 1))
	theme.ApplyThemedTextInput(&m.relationInput)
	guidance := "Task references separated by spaces, e.g. 12. Empty clears."
	if m.relationKind == formRelationDependencies {
		guidance = "Task references, e.g. 12 web:4. Empty clears."
	}
	if m.relationKind == formRelationDefer {
		guidance = "Tomorrow, +4h, or an ISO timestamp. Empty clears."
	}
	content := strings.Join(textfit.WrapStyled(guidance, max(width-4, 1)), "\n") + "\n\n " + m.relationInput.View()
	if m.relationError != nil {
		content += "\n\n" + theme.ValidationErrorStyle.Render(strings.Join(textfit.WrapStyled(m.relationError.Error(), max(width-4, 1)), "\n"))
	}
	footer := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec("Enter apply", "Esc cancel"), max(width-4, 1))
	return chrome.RenderOverlayBox(content, width, title, footer, lipgloss.NewStyle().Foreground(theme.ColorPrimary))
}

func (m *TaskFormModel) handleRelationEditor(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.closeInlineOverlay()
		return nil
	case "enter":
		if err := m.applyRelationInput(); err != nil {
			m.relationError = err
			return nil
		}
		m.hasChanges = true
		m.err = nil
		m.closeInlineOverlay()
		return nil
	default:
		before := m.relationInput.Value()
		var cmd tea.Cmd
		m.relationInput, cmd = m.relationInput.Update(msg)
		if before != m.relationInput.Value() {
			m.relationError = nil
		}
		return cmd
	}
}

func (m *TaskFormModel) applyRelationInput() error {
	value := strings.TrimSpace(m.relationInput.Value())
	if m.relationKind == formRelationDefer {
		if value == "" {
			m.draft.deferredUntil = nil
			return nil
		}
		instant, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			instant, err = util.ParseDeferInstant(value)
		}
		if err != nil {
			return err
		}
		m.draft.deferredUntil = &instant
		return nil
	}
	prefix := "~"
	if m.relationKind == formRelationChildren {
		prefix = "<"
	}
	var tokens []string
	for _, token := range strings.Fields(value) {
		tokens = append(tokens, prefix+strings.TrimPrefix(token, prefix))
	}
	requests, problems := capturetext.Requests("Task "+strings.Join(tokens, " "), m.project, m.allProjects, capturetext.Defaults{})
	if len(problems) > 0 {
		return fmt.Errorf("%s", problems[0].Message)
	}
	if len(requests) != 1 || requests[0].Title != "Task" {
		return fmt.Errorf("enter task references only; empty clears the list")
	}
	if m.relationKind == formRelationChildren {
		m.draft.childIDs = append([]string(nil), requests[0].ChildIDs...)
	} else {
		m.draft.dependencies = append([]app.TaskLocator(nil), requests[0].Dependencies...)
	}
	return nil
}
