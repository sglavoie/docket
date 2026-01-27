package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// LinkedTaskRelation represents a task and its relationship to the source task.
type LinkedTaskRelation struct {
	Task         *models.Task
	RelationType string // "blocks", "blocked_by", "related"
	Direction    string // "→", "←", "↔"
}

// LinkedTaskSelectorModel is a picker for navigating to linked tasks.
type LinkedTaskSelectorModel struct {
	sourceTask   *models.Task
	project      *models.Project
	relations    []LinkedTaskRelation
	cursor       int
	scrollOffset int
	width        int
	height       int
}

// NewLinkedTaskSelectorModel creates a new linked task selector.
func NewLinkedTaskSelectorModel(task *models.Task, project *models.Project) *LinkedTaskSelectorModel {
	m := &LinkedTaskSelectorModel{
		sourceTask: task,
		project:    project,
	}
	m.loadRelations()
	return m
}

// loadRelations collects all linked tasks and their relationship types.
func (m *LinkedTaskSelectorModel) loadRelations() {
	m.relations = nil

	// Tasks that this task blocks (i.e., tasks that depend on this task)
	blockedTasks := m.project.GetBlockedByTasks(m.sourceTask)
	for _, t := range blockedTasks {
		m.relations = append(m.relations, LinkedTaskRelation{
			Task:         t,
			RelationType: "blocks",
			Direction:    "→",
		})
	}

	// Tasks that this task depends on (blockers)
	for _, depID := range m.sourceTask.DependsOn {
		if t := m.project.GetTaskByID(depID); t != nil {
			m.relations = append(m.relations, LinkedTaskRelation{
				Task:         t,
				RelationType: "blocked by",
				Direction:    "←",
			})
		}
	}

	// Related tasks (bidirectional)
	for _, relID := range m.sourceTask.RelatedTo {
		if t := m.project.GetTaskByID(relID); t != nil {
			m.relations = append(m.relations, LinkedTaskRelation{
				Task:         t,
				RelationType: "related",
				Direction:    "↔",
			})
		}
	}
}

// HasLinkedTasks returns true if there are any linked tasks.
func (m *LinkedTaskSelectorModel) HasLinkedTasks() bool {
	return len(m.relations) > 0
}

// GetSingleLinkedTask returns the only linked task if there's exactly one.
func (m *LinkedTaskSelectorModel) GetSingleLinkedTask() *models.Task {
	if len(m.relations) == 1 {
		return m.relations[0].Task
	}
	return nil
}

// SetSize sets the view dimensions.
func (m *LinkedTaskSelectorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *LinkedTaskSelectorModel) Init() tea.Cmd {
	return nil
}

func (m *LinkedTaskSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeypress(msg)
	}
	return m, nil
}

func (m *LinkedTaskSelectorModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return CloseLinkedTaskSelectorMsg{}
		}

	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.relations) {
			rel := m.relations[m.cursor]
			return m, func() tea.Msg {
				return LinkedTaskSelectedMsg{Task: rel.Task, Project: m.project}
			}
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.relations)-1 {
			m.cursor++
		}
		return m, nil

	case "g":
		m.cursor = 0
		return m, nil

	case "G":
		if len(m.relations) > 0 {
			m.cursor = len(m.relations) - 1
		}
		return m, nil
	}

	return m, nil
}

func (m *LinkedTaskSelectorModel) View() string {
	if len(m.relations) == 0 {
		return renderOverlayCard(
			MutedStyle.Render("No linked tasks."),
			m.width, m.height,
			OverlayCardConfig{
				Title:    "Linked Tasks",
				Subtitle: fmt.Sprintf("#%d %s", m.sourceTask.ShortID, truncateTitle(m.sourceTask.Title, 30)),
				Centered: true,
			},
		)
	}

	// Max visible ITEMS (not lines) - cap at 8 for reasonable overlay size
	const maxVisibleItems = 8

	// Update scroll offset to keep cursor visible within the item window
	if m.cursor >= m.scrollOffset+maxVisibleItems {
		m.scrollOffset = m.cursor - maxVisibleItems + 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}

	// Group relations by type
	var blocksItems, blockedByItems, relatedItems []LinkedTaskRelation
	for _, rel := range m.relations {
		switch rel.RelationType {
		case "blocks":
			blocksItems = append(blocksItems, rel)
		case "blocked by":
			blockedByItems = append(blockedByItems, rel)
		case "related":
			relatedItems = append(relatedItems, rel)
		}
	}

	// Determine visible item range
	visibleStart := m.scrollOffset
	visibleEnd := m.scrollOffset + maxVisibleItems
	if visibleEnd > len(m.relations) {
		visibleEnd = len(m.relations)
	}

	var b strings.Builder

	// Show scroll indicator at top if items are hidden above
	if visibleStart > 0 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ↑ %d more above", visibleStart)))
		b.WriteString("\n\n")
	}

	// Render sections, but only items within the visible range
	itemIndex := 0
	needSpacer := false

	// Blocks section
	if len(blocksItems) > 0 {
		sectionStart := itemIndex
		sectionEnd := itemIndex + len(blocksItems)

		// Check if any items from this section are in visible range
		if sectionEnd > visibleStart && sectionStart < visibleEnd {
			if needSpacer {
				b.WriteString("\n")
			}
			b.WriteString(SubtitleStyle.Render("→ Blocks"))
			b.WriteString("\n")
			needSpacer = true

			for i, rel := range blocksItems {
				idx := sectionStart + i
				if idx >= visibleStart && idx < visibleEnd {
					cursor := "  "
					if idx == m.cursor {
						cursor = "> "
					}
					b.WriteString(cursor + m.renderRelationLine(rel, false) + "\n")
				}
			}
		}
		itemIndex = sectionEnd
	}

	// Blocked By section
	if len(blockedByItems) > 0 {
		sectionStart := itemIndex
		sectionEnd := itemIndex + len(blockedByItems)

		if sectionEnd > visibleStart && sectionStart < visibleEnd {
			if needSpacer {
				b.WriteString("\n")
			}
			b.WriteString(SubtitleStyle.Render("← Blocked By"))
			b.WriteString("\n")
			needSpacer = true

			for i, rel := range blockedByItems {
				idx := sectionStart + i
				if idx >= visibleStart && idx < visibleEnd {
					cursor := "  "
					if idx == m.cursor {
						cursor = "> "
					}
					b.WriteString(cursor + m.renderRelationLine(rel, false) + "\n")
				}
			}
		}
		itemIndex = sectionEnd
	}

	// Related section
	if len(relatedItems) > 0 {
		sectionStart := itemIndex
		sectionEnd := itemIndex + len(relatedItems)

		if sectionEnd > visibleStart && sectionStart < visibleEnd {
			if needSpacer {
				b.WriteString("\n")
			}
			b.WriteString(SubtitleStyle.Render("↔ Related"))
			b.WriteString("\n")

			for i, rel := range relatedItems {
				idx := sectionStart + i
				if idx >= visibleStart && idx < visibleEnd {
					cursor := "  "
					if idx == m.cursor {
						cursor = "> "
					}
					b.WriteString(cursor + m.renderRelationLine(rel, false) + "\n")
				}
			}
		}
	}

	// Show scroll indicator at bottom if items are hidden below
	if visibleEnd < len(m.relations) {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("\n  ↓ %d more below", len(m.relations)-visibleEnd)))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("j/k navigate • Enter select • Esc cancel"))

	cardWidth := m.width - 4
	if cardWidth > 70 {
		cardWidth = 70
	}

	return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Navigate Linked Tasks",
		Subtitle: fmt.Sprintf("#%d %s", m.sourceTask.ShortID, truncateTitle(m.sourceTask.Title, 30)),
		Width:    cardWidth,
		Centered: true,
	})
}

func (m *LinkedTaskSelectorModel) renderRelationLine(rel LinkedTaskRelation, isSelected bool) string {
	statusIcon := GetStatusBadge(string(rel.Task.Status), rel.Task.Status.Icon())
	typeIcon := GetTypeBadge(string(rel.Task.Type), rel.Task.Type.Icon())

	maxTitleLen := m.width - 30
	if maxTitleLen < 20 {
		maxTitleLen = 20
	}
	title := truncateTitle(rel.Task.Title, maxTitleLen)

	line := fmt.Sprintf("%s %s #%d %s", statusIcon, typeIcon, rel.Task.ShortID, title)
	if isSelected {
		line = SelectedStyle.Render(line)
	}
	return line
}

// HasActiveSearch returns false (no search in this view).
func (m *LinkedTaskSelectorModel) HasActiveSearch() bool {
	return false
}

// HasTextInput returns false for the linked task selector.
func (m *LinkedTaskSelectorModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns false so the app can close the view on esc.
func (m *LinkedTaskSelectorModel) ShouldCaptureEsc() bool {
	return false
}
