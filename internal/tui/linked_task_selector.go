package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
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
	sourceTask    *models.Task
	project       *models.Project
	config        *config.Config
	relations     []LinkedTaskRelation
	filtered      []LinkedTaskRelation // filtered relations based on search
	cursor        int
	cursorFlash   bool
	scrollOffset  int
	pendingPrefix string
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
	missing := func(project *models.Project) LinkedTaskRelation {
		return LinkedTaskRelation{
			Project:      project,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
			IsMissing:    true,
		}
	}

	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		task := m.project.GetTaskByID(taskRef.TaskID)
		if task == nil {
			return missing(nil)
		}
		return LinkedTaskRelation{
			Task:         task,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
		}
	}

	// Cross-project reference
	if m.config == nil {
		return missing(nil)
	}

	targetProject := m.config.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return missing(nil)
	}

	if len(targetProject.Tasks) == 0 {
		if err := storage.LoadProject(targetProject); err != nil {
			logging.Warn("failed to load project %s: %v", targetProject.Name, err)
		}
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return missing(targetProject)
	}

	return LinkedTaskRelation{
		Task:         task,
		Project:      targetProject,
		RelationType: relationType,
		Direction:    direction,
		TaskRef:      ref,
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
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeypress(msg)
	}
	return m, nil
}

func (m *LinkedTaskSelectorModel) handleKeypress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys
	if m.pendingPrefix != "" && isModifierOnly(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		prefix := m.pendingPrefix
		m.pendingPrefix = ""
		if key != "esc" && prefix == "g" {
			_, cmd := handleGotoTopPrefix(key, &m.cursor, &m.cursorFlash)
			return m, cmd
		}
		return m, nil
	}

	// Handle search mode input first
	if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterRelations); handled {
		return model, cmd
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

	case "g":
		m.pendingPrefix = "g"
		return m, nil
	}

	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
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

	project := m.project
	return func() tea.Msg {
		// Save the source project
		if err := storage.BackupProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to backup: %w", err)}
		}
		if err := storage.SaveProjectLocked(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		// Reload and close
		if err := storage.LoadProject(project); err != nil {
			logging.Warn("failed to reload project %s: %v", project.Name, err)
		}
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

func (m *LinkedTaskSelectorModel) View() tea.View { //nolint:gocyclo
	taskSubtitle := fmt.Sprintf("#%d %s", m.sourceTask.ShortID, truncateToVisualWidth(m.sourceTask.Title, 30))

	var b strings.Builder

	if len(m.relations) == 0 {
		b.WriteString(renderFullscreenTitle("Linked Tasks", taskSubtitle, m.AccentColor()))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("No linked tasks."))
		b.WriteString("\n")
		left := HelpStyle.Render("q close")
		b.WriteString(renderStatusBarFooter(left, "", m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	// Max visible ITEMS - use available height
	maxVisibleItems := max(m.height-10, 8)

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
	visibleEnd := min(m.scrollOffset+maxVisibleItems, len(m.filtered))

	// Title
	b.WriteString(renderFullscreenTitle("Navigate Linked Tasks", taskSubtitle, m.AccentColor()))
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
		left := HelpStyle.Render("Esc clear filter · q close")
		b.WriteString(renderStatusBarFooter(left, "", m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	// Render sections, but only items within the visible range
	var sectionBuilder strings.Builder
	itemIndex := 0
	needSpacer := false

	type relSection struct {
		title string
		items []LinkedTaskRelation
	}
	sections := []relSection{
		{"→ Blocks", blocksItems},
		{"← Blocked By", blockedByItems},
		{"↔ Related", relatedItems},
	}

	for _, sec := range sections {
		if len(sec.items) == 0 {
			continue
		}
		sectionStart := itemIndex
		sectionEnd := itemIndex + len(sec.items)

		if sectionEnd > visibleStart && sectionStart < visibleEnd {
			if needSpacer {
				sectionBuilder.WriteString("\n")
			}
			sectionBuilder.WriteString(SubtitleStyle.Render(sec.title))
			sectionBuilder.WriteString("\n")
			needSpacer = true

			for i, rel := range sec.items {
				idx := sectionStart + i
				if idx >= visibleStart && idx < visibleEnd {
					cursor := "  "
					if idx == m.cursor {
						cursor = "  "
					}
					sectionBuilder.WriteString(cursor + m.renderRelationLine(rel, false) + "\n")
				}
			}
		}
		itemIndex = sectionEnd
	}

	scrollbar := RenderScrollbar(len(m.filtered), visibleEnd-visibleStart, visibleStart)
	b.WriteString(AttachScrollbar(sectionBuilder.String(), scrollbar))

	// Footer
	var footerLeft string
	if m.IsSearchMode() {
		footerLeft = HelpStyle.Render("Type to filter · Enter confirm · Esc cancel")
	} else {
		footerLeft = HelpStyle.Render("/ search · j/k navigate · Enter select · x unlink · Esc cancel")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d links", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(footerLeft, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
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

	// Stage badge (after type icon)
	stageBadge := GetStageBadge(rel.Task.Stage)
	stageSegment := ""
	if stageBadge != "" {
		stageSegment = stageBadge + " "
	}

	availableWidth := max(m.width-4, 20)

	// Show project name for cross-project links
	var projectLabel string
	if rel.Project != nil {
		projectName := truncateToVisualWidth(rel.Project.Name, 12)
		projectLabel = MutedStyle.Render(fmt.Sprintf("[%s] ", projectName))
	}

	prefix := fmt.Sprintf("%s %s %s%s#%d ", statusIcon, typeIcon, stageSegment, projectLabel, rel.Task.ShortID)
	maxTitleLen := max(availableWidth-lipgloss.Width(prefix), 1)
	title := truncateToVisualWidth(rel.Task.Title, maxTitleLen)
	line := prefix + title
	if lipgloss.Width(line) > availableWidth {
		excess := lipgloss.Width(line) - availableWidth
		title = truncateToVisualWidth(rel.Task.Title, max(maxTitleLen-excess, 1))
		line = prefix + title
	}
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

func (m *LinkedTaskSelectorModel) StatusBarLabel() string { return "Linked Tasks" }
