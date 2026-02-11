package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// LinkedTaskRelation represents a task and its relationship to the source task.
type LinkedTaskRelation struct {
	Task         *models.Task    // nil if cross-project task not found (broken link)
	Project      *models.Project // nil for same-project links
	RelationType string          // "blocks", "blocked_by", "related"
	Direction    string          // "→", "←", "↔"
	TaskRef      string          // Original reference string (for cross-project navigation)
	IsMissing    bool            // True if the linked task/project couldn't be found
}

// LinkedTaskSelectorModel is a picker for navigating to linked tasks.
type LinkedTaskSelectorModel struct {
	BaseSearchableView
	sourceTask   *models.Task
	project      *models.Project
	config       *config.Config
	relations    []LinkedTaskRelation
	filtered     []LinkedTaskRelation // filtered relations based on search
	cursor       int
	scrollOffset int
}

// NewLinkedTaskSelectorModelWithConfig creates a new linked task selector with config for cross-project support.
func NewLinkedTaskSelectorModelWithConfig(task *models.Task, project *models.Project, cfg *config.Config) *LinkedTaskSelectorModel {
	m := &LinkedTaskSelectorModel{
		sourceTask: task,
		project:    project,
		config:     cfg,
	}
	m.InitSearchInput("Search linked tasks...", 50, 30)
	m.loadRelations()
	m.filtered = m.relations
	return m
}

// loadRelations collects all linked tasks and their relationship types.
func (m *LinkedTaskSelectorModel) loadRelations() {
	m.relations = nil

	// Tasks that this task blocks (i.e., tasks that depend on this task)
	// These are same-project only (we don't track reverse cross-project deps)
	blockedTasks := m.project.GetBlockedByTasks(m.sourceTask)
	for _, t := range blockedTasks {
		m.relations = append(m.relations, LinkedTaskRelation{
			Task:         t,
			Project:      nil, // same project
			RelationType: "blocks",
			Direction:    "→",
			TaskRef:      t.ID,
		})
	}

	// Tasks that this task depends on (blockers) - may be cross-project
	for _, depRef := range m.sourceTask.DependsOn {
		rel := m.resolveTaskRef(depRef, "blocked by", "←")
		m.relations = append(m.relations, rel)
	}

	// Related tasks (bidirectional) - may be cross-project
	for _, relRef := range m.sourceTask.RelatedTo {
		rel := m.resolveTaskRef(relRef, "related", "↔")
		m.relations = append(m.relations, rel)
	}
}

// resolveTaskRef resolves a task reference (local or cross-project) to a LinkedTaskRelation.
func (m *LinkedTaskSelectorModel) resolveTaskRef(ref, relationType, direction string) LinkedTaskRelation {
	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		// Same-project reference
		task := m.project.GetTaskByID(taskRef.TaskID)
		if task != nil {
			return LinkedTaskRelation{
				Task:         task,
				Project:      nil,
				RelationType: relationType,
				Direction:    direction,
				TaskRef:      ref,
				IsMissing:    false,
			}
		}
		// Task not found in same project
		return LinkedTaskRelation{
			Task:         nil,
			Project:      nil,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
			IsMissing:    true,
		}
	}

	// Cross-project reference
	if m.config == nil {
		// No config available, can't resolve cross-project refs
		return LinkedTaskRelation{
			Task:         nil,
			Project:      nil,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
			IsMissing:    true,
		}
	}

	targetProject := m.config.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return LinkedTaskRelation{
			Task:         nil,
			Project:      nil,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
			IsMissing:    true,
		}
	}

	// Load the project if not already loaded
	if len(targetProject.Tasks) == 0 {
		_ = storage.LoadProject(targetProject)
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return LinkedTaskRelation{
			Task:         nil,
			Project:      targetProject,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
			IsMissing:    true,
		}
	}

	return LinkedTaskRelation{
		Task:         task,
		Project:      targetProject,
		RelationType: relationType,
		Direction:    direction,
		TaskRef:      ref,
		IsMissing:    false,
	}
}

// HasLinkedTasks returns true if there are any linked tasks.
func (m *LinkedTaskSelectorModel) HasLinkedTasks() bool {
	return len(m.relations) > 0
}

// GetSingleLinkedTask returns the only linked task if there's exactly one and it's not missing.
func (m *LinkedTaskSelectorModel) GetSingleLinkedTask() *models.Task {
	if len(m.relations) == 1 && !m.relations[0].IsMissing {
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

	// Handle search mode input first
	if m.IsSearchMode() {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged {
			m.filterRelations()
		}
		if handled {
			return m, cmd
		}
		// Pass through to cursor navigation while typing
		return m, cmd
	}

	switch key {
	case "/":
		// Activate search mode
		return m, m.ActivateSearch()

	case "esc":
		// If there's a search query, clear it first
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.filterRelations()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseLinkedTaskSelectorMsg{}
		}

	case "q":
		return m, func() tea.Msg {
			return CloseLinkedTaskSelectorMsg{}
		}

	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			rel := m.filtered[m.cursor]
			if rel.IsMissing {
				// For missing links, offer to unlink
				return m, m.unlinkMissingTaskFromFiltered(m.cursor)
			}
			// Navigate to the task
			project := rel.Project
			if project == nil {
				project = m.project // same project
			}
			taskRef := rel.TaskRef
			return m, func() tea.Msg {
				return LinkedTaskSelectedMsg{Task: rel.Task, Project: project, TaskRef: taskRef}
			}
		}
		return m, nil

	case "x", "d":
		// Unlink the selected task
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			return m, m.unlinkTaskFromFiltered(m.cursor)
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case "g":
		m.cursor = 0
		return m, nil

	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}
		return m, nil
	}

	return m, nil
}

// filterRelations filters the relations based on the current search query.
func (m *LinkedTaskSelectorModel) filterRelations() {
	query := strings.ToLower(strings.TrimSpace(m.SearchQuery()))
	if query == "" {
		m.filtered = m.relations
		m.cursor = 0
		m.scrollOffset = 0
		return
	}

	m.filtered = nil
	for _, rel := range m.relations {
		// Build searchable text
		var searchText string
		if rel.IsMissing {
			ref := models.ParseTaskRef(rel.TaskRef)
			if ref.IsCrossProject() {
				if rel.Project != nil {
					searchText = strings.ToLower(rel.Project.Name + " missing")
				} else {
					searchText = strings.ToLower(ref.ProjectID + " missing")
				}
			} else {
				searchText = "missing"
			}
		} else {
			searchText = strings.ToLower(rel.Task.Title + " " + fmt.Sprintf("#%d", rel.Task.ShortID))
			if rel.Project != nil {
				searchText += " " + strings.ToLower(rel.Project.Name)
			}
		}

		// Check if query matches
		if strings.Contains(searchText, query) {
			m.filtered = append(m.filtered, rel)
		}
	}

	m.cursor = 0
	m.scrollOffset = 0
}

// unlinkTask removes the link at the given index.
func (m *LinkedTaskSelectorModel) unlinkTask(idx int) tea.Cmd {
	return func() tea.Msg {
		if idx < 0 || idx >= len(m.relations) {
			return nil
		}
		rel := m.relations[idx]

		// Remove from source task's dependencies/related
		switch rel.RelationType {
		case "blocked by":
			m.sourceTask.RemoveDependency(rel.TaskRef)
		case "related":
			m.sourceTask.RemoveRelatedTo(rel.TaskRef)
		case "blocks":
			// For "blocks", we need to remove from the other task's DependsOn
			if rel.Task != nil {
				rel.Task.RemoveDependency(m.sourceTask.ID)
			}
		}

		// Save the source project
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		// Reload and close
		_ = storage.LoadProject(m.project)
		return CloseLinkedTaskSelectorMsg{}
	}
}

// unlinkTaskFromFiltered removes the link at the given index in the filtered list.
func (m *LinkedTaskSelectorModel) unlinkTaskFromFiltered(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.filtered) {
		return nil
	}
	// Find the original index in m.relations
	filteredRel := m.filtered[idx]
	for origIdx, rel := range m.relations {
		if rel.TaskRef == filteredRel.TaskRef && rel.RelationType == filteredRel.RelationType {
			return m.unlinkTask(origIdx)
		}
	}
	return nil
}

// unlinkMissingTaskFromFiltered handles unlinking a broken/missing link from the filtered list.
func (m *LinkedTaskSelectorModel) unlinkMissingTaskFromFiltered(idx int) tea.Cmd {
	return m.unlinkTaskFromFiltered(idx)
}

func (m *LinkedTaskSelectorModel) View() string {
	taskSubtitle := fmt.Sprintf("#%d %s", m.sourceTask.ShortID, truncateTitle(m.sourceTask.Title, 30))

	var b strings.Builder

	if len(m.relations) == 0 {
		b.WriteString(renderFullscreenTitle("Linked Tasks", taskSubtitle))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("No linked tasks."))
		b.WriteString("\n")
		left := HelpStyle.Render("q close")
		b.WriteString(renderStatusBarFooter(left, "", m.width))
		return b.String()
	}

	// Max visible ITEMS - use available height
	maxVisibleItems := m.height - 10
	if maxVisibleItems < 8 {
		maxVisibleItems = 8
	}

	// Update scroll offset to keep cursor visible within the item window
	if m.cursor >= m.scrollOffset+maxVisibleItems {
		m.scrollOffset = m.cursor - maxVisibleItems + 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}

	// Group filtered relations by type
	var blocksItems, blockedByItems, relatedItems []LinkedTaskRelation
	for _, rel := range m.filtered {
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
	if visibleEnd > len(m.filtered) {
		visibleEnd = len(m.filtered)
	}

	// Title
	b.WriteString(renderFullscreenTitle("Navigate Linked Tasks", taskSubtitle))
	b.WriteString("\n\n")

	// Show search input when active
	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filtered))))
		b.WriteString("\n\n")
	}

	// Show "no matches" if filtering resulted in empty list
	if len(m.filtered) == 0 && len(m.relations) > 0 {
		b.WriteString(MutedStyle.Render("No matching linked tasks."))
		b.WriteString("\n")
		left := HelpStyle.Render("Esc clear filter • q close")
		b.WriteString(renderStatusBarFooter(left, "", m.width))
		return b.String()
	}

	// Show scroll indicator at top if items are hidden above
	if visibleStart > 0 {
		b.WriteString("  " + ScrollUpIndicator(visibleStart))
		b.WriteString("\n\n")
	}

	// Render sections, but only items within the visible range
	itemIndex := 0
	needSpacer := false

	// Blocks section
	if len(blocksItems) > 0 {
		sectionStart := itemIndex
		sectionEnd := itemIndex + len(blocksItems)

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
	if visibleEnd < len(m.filtered) {
		b.WriteString("\n  " + ScrollDownIndicator(len(m.filtered)-visibleEnd))
		b.WriteString("\n")
	}

	// Footer
	var footerLeft string
	if m.IsSearchMode() {
		footerLeft = HelpStyle.Render("Type to filter • Enter confirm • Esc cancel")
	} else {
		footerLeft = HelpStyle.Render("/ search • j/k navigate • Enter select • x unlink • Esc cancel")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d links", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(footerLeft, right, m.width))

	return b.String()
}

func (m *LinkedTaskSelectorModel) renderRelationLine(rel LinkedTaskRelation, isSelected bool) string {
	if rel.IsMissing {
		// Show broken link indicator
		ref := models.ParseTaskRef(rel.TaskRef)
		var label string
		if ref.IsCrossProject() {
			label = fmt.Sprintf("[%s] (missing)", ref.ProjectID)
		} else {
			label = "(missing)"
		}
		line := ValidationErrorStyle.Render(label)
		if isSelected {
			line = SelectedStyle.Render(line)
		}
		return line
	}

	statusIcon := GetStatusBadge(string(rel.Task.Status), rel.Task.Status.Icon())
	typeIcon := GetTypeBadge(string(rel.Task.Type), rel.Task.Type.Icon())

	maxTitleLen := m.width - 35
	if maxTitleLen < 20 {
		maxTitleLen = 20
	}

	// Show project name for cross-project links
	var projectLabel string
	if rel.Project != nil {
		projectLabel = MutedStyle.Render(fmt.Sprintf("[%s] ", rel.Project.Name))
		maxTitleLen -= len(rel.Project.Name) + 3
		if maxTitleLen < 15 {
			maxTitleLen = 15
		}
	}

	title := truncateTitle(rel.Task.Title, maxTitleLen)
	line := fmt.Sprintf("%s %s %s#%d %s", statusIcon, typeIcon, projectLabel, rel.Task.ShortID, title)
	if isSelected {
		line = SelectedStyle.Render(line)
	}
	return line
}

// HasActiveSearch returns true if search mode is active or a filter is applied.
func (m *LinkedTaskSelectorModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when in search mode.
func (m *LinkedTaskSelectorModel) HasTextInput() bool {
	return m.IsSearchMode()
}

// ShouldCaptureEsc returns true when search is active so esc clears search first.
func (m *LinkedTaskSelectorModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch()
}
