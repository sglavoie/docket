package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TutorTopic represents a single topic in the tutor view.
type TutorTopic struct {
	Title    string
	Category string
	Content  string
}

// TutorModel implements the tutor view with a two-pane topic browser.
type TutorModel struct {
	topics         []TutorTopic
	filteredTopics []TutorTopic
	selectedIndex  int
	activePane     int // 0 = list, 1 = content
	viewport       viewport.Model
	searchInput    textinput.Model
	searchMode     bool
	searchQuery    string
	activeTab      int
	categories     []string
	width          int
	height         int
	ready          bool
}

// NewTutorModel creates a new tutor view.
func NewTutorModel() *TutorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search topics..."
	searchInput.CharLimit = 50
	searchInput.Width = 25

	m := &TutorModel{
		topics:      tutorTopics(),
		searchInput: searchInput,
	}
	m.filteredTopics = m.topics
	m.categories = m.buildCategories()
	return m
}

func (m *TutorModel) buildCategories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, t := range m.topics {
		if !seen[t.Category] {
			seen[t.Category] = true
			cats = append(cats, t.Category)
		}
	}
	return cats
}

// HasTextInput returns true when search is active.
func (m *TutorModel) HasTextInput() bool {
	return m.searchMode
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *TutorModel) ShouldCaptureEsc() bool {
	return m.searchMode || m.searchQuery != ""
}

func (m *TutorModel) Init() tea.Cmd {
	return nil
}

func (m *TutorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.filterTopics()
				m.selectedIndex = 0
				m.updateViewport()
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.filterTopics()
				m.selectedIndex = 0
				m.updateViewport()
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "tab":
			m.activePane = 1 - m.activePane
			return m, nil
		case "h":
			if m.activePane == 0 {
				// Switch tab left
				if m.activeTab > 0 {
					m.activeTab--
				} else {
					m.activeTab = len(m.categories)
				}
				m.filterTopics()
				m.selectedIndex = 0
				m.updateViewport()
			}
			return m, nil
		case "l":
			if m.activePane == 0 {
				// Switch tab right
				if m.activeTab < len(m.categories) {
					m.activeTab++
				} else {
					m.activeTab = 0
				}
				m.filterTopics()
				m.selectedIndex = 0
				m.updateViewport()
			}
			return m, nil
		case "[":
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(m.categories)
			}
			m.filterTopics()
			m.selectedIndex = 0
			m.updateViewport()
			return m, nil
		case "]":
			if m.activeTab < len(m.categories) {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.filterTopics()
			m.selectedIndex = 0
			m.updateViewport()
			return m, nil
		case "up", "k":
			if m.activePane == 0 {
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.updateViewport()
				}
			} else {
				m.viewport.ScrollUp(1)
			}
			return m, nil
		case "down", "j":
			if m.activePane == 0 {
				if m.selectedIndex < len(m.filteredTopics)-1 {
					m.selectedIndex++
					m.updateViewport()
				}
			} else {
				m.viewport.ScrollDown(1)
			}
			return m, nil
		case "g":
			if m.activePane == 1 {
				m.viewport.GotoTop()
			}
			return m, nil
		case "G":
			if m.activePane == 1 {
				m.viewport.GotoBottom()
			}
			return m, nil
		case "pgup", "ctrl+u":
			if m.activePane == 1 {
				m.viewport.HalfPageUp()
			}
			return m, nil
		case "pgdown", "ctrl+d":
			if m.activePane == 1 {
				m.viewport.HalfPageDown()
			}
			return m, nil
		case "enter":
			if m.activePane == 0 {
				m.activePane = 1
				m.viewport.GotoTop()
			}
			return m, nil
		}
	}

	if m.activePane == 1 {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *TutorModel) filterTopics() {
	source := m.topics
	if m.activeTab > 0 && m.activeTab <= len(m.categories) {
		cat := m.categories[m.activeTab-1]
		source = nil
		for _, t := range m.topics {
			if t.Category == cat {
				source = append(source, t)
			}
		}
	}

	if m.searchQuery == "" {
		m.filteredTopics = source
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []TutorTopic
	for _, t := range source {
		if strings.Contains(strings.ToLower(t.Title), query) ||
			strings.Contains(strings.ToLower(t.Content), query) ||
			strings.Contains(strings.ToLower(t.Category), query) {
			filtered = append(filtered, t)
		}
	}
	m.filteredTopics = filtered
}

func (m *TutorModel) updateViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderTopicContent())
	m.viewport.GotoTop()
}

func (m *TutorModel) innerWidth() int {
	w := m.width - 2 // 1 char padding each side
	if w < 40 {
		w = 40
	}
	return w
}

func (m *TutorModel) listWidth() int {
	lw := m.innerWidth() * 30 / 100
	if lw < 18 {
		lw = 18
	}
	if lw > 30 {
		lw = 30
	}
	return lw
}

func (m *TutorModel) contentWidth() int {
	return m.innerWidth() - m.listWidth() - 1 // -1 for divider
}

func (m *TutorModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpWidth := m.contentWidth() - 1 // 1 char padding in split pane
	tabBarHeight := 2
	searchHeight := 2
	footerHeight := 2
	vpHeight := height - tabBarHeight - searchHeight - footerHeight - 1

	if vpWidth < 30 {
		vpWidth = 30
	}
	if vpHeight < 5 {
		vpHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.viewport.YPosition = 0
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}

	m.viewport.SetContent(m.renderTopicContent())
}

func (m *TutorModel) renderTopicList() string {
	listW := m.listWidth()
	var b strings.Builder

	cursorStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	mutedCat := lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	if len(m.filteredTopics) == 0 {
		return MutedStyle.Render("No topics found.")
	}

	// Group by category for display
	lastCat := ""
	idx := 0
	for _, t := range m.filteredTopics {
		if t.Category != lastCat {
			if lastCat != "" {
				b.WriteString("\n")
			}
			header := t.Category
			if lipgloss.Width(header) > listW {
				header = header[:listW-1] + "…"
			}
			b.WriteString(mutedCat.Render(header))
			b.WriteString("\n")
			lastCat = t.Category
		}

		title := t.Title
		maxTitleW := listW - 3 // cursor + space + padding
		if lipgloss.Width(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}

		if idx == m.selectedIndex {
			if m.activePane == 0 {
				b.WriteString(cursorStyle.Render("▸ " + title))
			} else {
				b.WriteString(normalStyle.Bold(true).Render("▸ " + title))
			}
		} else {
			b.WriteString("  " + normalStyle.Render(title))
		}
		b.WriteString("\n")
		idx++
	}

	return b.String()
}

func (m *TutorModel) renderTopicContent() string {
	if len(m.filteredTopics) == 0 {
		return MutedStyle.Render("No topic selected.")
	}

	idx := m.selectedIndex
	if idx >= len(m.filteredTopics) {
		idx = len(m.filteredTopics) - 1
	}
	topic := m.filteredTopics[idx]

	contentW := m.contentWidth() - 2 // some padding
	if contentW < 20 {
		contentW = 20
	}

	return renderTutorContent(topic, contentW)
}

func renderTutorContent(topic TutorTopic, width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	headingStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	proseStyle := lipgloss.NewStyle()

	var b strings.Builder
	b.WriteString(titleStyle.Render(topic.Title))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render(topic.Category))
	b.WriteString("\n\n")

	for _, line := range strings.Split(topic.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString(headingStyle.Render(trimmed[2:]))
		case strings.HasPrefix(trimmed, "tt ") || strings.HasPrefix(trimmed, "$ "):
			b.WriteString("  " + codeStyle.Render(trimmed))
		case trimmed == "":
			// blank line
		default:
			b.WriteString(proseStyle.Render(wrapTutorText(trimmed, width)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// wrapText wraps text to the given width.
func wrapTutorText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}

	var lines []string
	remaining := text
	for len(remaining) > width {
		bp := width
		for i := width; i > 0; i-- {
			if remaining[i-1] == ' ' {
				bp = i - 1
				break
			}
		}
		lines = append(lines, remaining[:bp])
		remaining = strings.TrimLeft(remaining[bp:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return strings.Join(lines, "\n")
}

func (m *TutorModel) renderTabBar() string {
	tabNames := append([]string{"All"}, m.categories...)
	availWidth := m.innerWidth()

	plainStyle := lipgloss.NewStyle().Padding(0, 1)
	totalWidth := 0
	for _, name := range tabNames {
		totalWidth += lipgloss.Width(plainStyle.Render(name))
	}
	fitsSingleLine := totalWidth <= availWidth

	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)

	if fitsSingleLine {
		activeStyle = activeStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(ColorPrimary)
		inactiveStyle = inactiveStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder)
	}

	var tabs []string
	for i, name := range tabNames {
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	if fitsSingleLine {
		tabWidth := lipgloss.Width(row)
		remaining := availWidth - tabWidth
		if remaining > 0 {
			filler := lipgloss.NewStyle().
				BorderBottom(true).
				BorderStyle(lipgloss.Border{Bottom: "─"}).
				BorderForeground(ColorBorder).
				Width(remaining).
				Render("")
			row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
		}
	}

	return row
}

func (m *TutorModel) View() string {
	pad := " " // 1 char left padding
	var b strings.Builder

	// Tab bar
	b.WriteString(pad)
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Search bar
	if m.searchMode {
		b.WriteString(pad)
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	} else if m.searchQuery != "" {
		filterInfo := fmt.Sprintf("Search: \"%s\" ", m.searchQuery)
		b.WriteString(pad)
		b.WriteString(HelpFilterActiveStyle.Render(filterInfo))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n\n")
	}

	// Split pane
	if m.ready {
		leftContent := m.renderTopicList()
		rightContent := m.viewport.View()

		lw := m.listWidth()
		rw := m.innerWidth() - lw - 1
		vpH := m.viewport.Height

		pane := renderOverlaySplitPane(leftContent, rightContent, lw, rw, vpH)
		// Add left padding to each line of the split pane
		for i, line := range strings.Split(pane, "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(pad)
			b.WriteString(line)
		}
	}

	// Footer
	b.WriteString("\n\n")
	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	paneHint := "list"
	if m.activePane == 1 {
		paneHint = "content"
	}
	footer := HelpStyle.Render(fmt.Sprintf("Tab: switch pane (%s) | [/]: tabs | /: search | j/k: navigate | q: close%s", paneHint, scrollPercent))
	b.WriteString(pad)
	b.WriteString(footer)

	return b.String()
}

// tutorTopics returns all tutor topics organized by category.
func tutorTopics() []TutorTopic {
	return []TutorTopic{
		// Getting Started
		{
			Title:    "Projects",
			Category: "Getting Started",
			Content: `# Creating Projects

Every task in tt belongs to a project. Create your first project and
start organizing work immediately.

  tt project new "My Project"
  tt project new "Work Tasks" --context work

# Selecting Projects

Switch between projects to focus on different areas of work. The
selected project is used by all task commands.

  tt select
  tt project list

# Pinning and Archiving

Pin frequently-used projects so they appear at the top of your
dashboard. Archive completed projects to keep things tidy.

  tt project pin "My Project"
  tt project archive "Old Project"`,
		},
		{
			Title:    "Quick Capture",
			Category: "Getting Started",
			Content: `# Inline Syntax

Add tasks quickly with inline modifiers. Priority is set with !,
tags with #, and section assignment with @.

  tt add "Fix login bug !1 #backend @active"
  tt add "Update docs !3 #frontend"
  tt add "Research caching #perf @next"

# Priority Shorthand

Priority ranges from 1 (highest) to 4 (lowest). Use ! followed by
the number anywhere in the task title.

  tt add "Critical fix !1"
  tt add "Nice to have !4"

# Tags and Sections

Tags help you categorize tasks across projects. Sections follow
GTD methodology for workflow organization.

  tt add "Deploy service #ops #urgent @active"`,
		},
		{
			Title:    "TUI Overview",
			Category: "Getting Started",
			Content: `# View Stack

The TUI uses a stack-based navigation model. Each screen you open
is pushed onto a stack. Press q to go back to the previous screen,
or Esc to dismiss the current action.

# Command Palette

Press Ctrl+K from any view to open the command palette. Type to
filter commands and press Enter to execute. This is the fastest
way to access any feature.

# Getting Help

Press ? in any view to see all available keybindings for that
screen. Help is context-sensitive and shows only relevant actions.

# Navigation Basics

Most views use j/k or arrow keys for movement. Enter opens or
selects items. Tab switches between panes in split views.`,
		},

		// Task Management
		{
			Title:    "Task Types",
			Category: "Task Management",
			Content: `# Type Hierarchy

tt supports five task types organized in a hierarchy. Epics are
the largest unit, containing stories, tasks, bugs, and ideas.
Stories can contain tasks and bugs. Tasks, bugs, and ideas are
leaf nodes that cannot contain children.

  tt task add --type epic "User Authentication"
  tt task add --type story "Login Flow" --parent <epic-id>
  tt task add --type bug "Fix timeout" --parent <story-id>

# When to Use Each

Epics represent large features or initiatives spanning weeks.
Stories are user-facing deliverables within an epic. Tasks are
concrete work items. Bugs track defects. Ideas capture future
possibilities without commitment.

  tt task add --type idea "Consider GraphQL migration"`,
		},
		{
			Title:    "Statuses",
			Category: "Task Management",
			Content: `# Available Statuses

Tasks move through five statuses: todo, in_progress, done,
blocked, and canceled.

# Transitions

Use dedicated commands for common transitions. These update the
status and record the timestamp.

  tt task start <id>
  tt task complete <id>
  tt task cancel <id>
  tt task block <id>

# Manual Status Setting

You can also set status directly for any transition.

  tt task status <id> todo
  tt task status <id> in_progress

In the TUI, press s on a selected task to cycle through
statuses, or use the command palette for direct access.`,
		},
		{
			Title:    "Priority",
			Category: "Task Management",
			Content: `# Priority Scale

tt uses a 1-4 priority scale where 1 is the highest (critical)
and 4 is the lowest (someday). Priority affects sort order in
all views, with higher-priority tasks appearing first.

# Setting Priority

Set priority when creating a task or update it later.

  tt add "Fix crash !1"
  tt task priority <id> 2

# Display

In the TUI, priority is shown with colored indicators next to
each task. Priority 1 tasks are highlighted in red, priority 2
in orange, priority 3 in yellow, and priority 4 uses the
default color.`,
		},
		{
			Title:    "Dependencies",
			Category: "Task Management",
			Content: `# Blocking Relationships

Tasks can depend on other tasks. When a task is blocked by
another, it cannot progress until the blocker is resolved.

  tt task depends <task-id> <blocker-id>

# How Blocking Works

When all blockers of a task are completed or canceled, the
task automatically becomes unblocked. The TUI shows blocked
tasks with a distinct indicator.

# Viewing Dependencies

In the project tree view, dependencies are visible in the
task detail sidebar. Use the task detail view to see the
full dependency chain.`,
		},

		// Organization
		{
			Title:    "Sections (GTD)",
			Category: "Organization",
			Content: `# Section Overview

Sections follow the Getting Things Done methodology to organize
your workflow. Each task belongs to one section that indicates
its current stage.

# Available Sections

Active contains tasks you are working on right now. Next holds
tasks queued up to start soon. Inbox is for unsorted incoming
tasks. Waiting is for tasks blocked on external input. Someday
captures tasks you might do eventually.

# Assigning Sections

  tt task section <id> active
  tt task section <id> next
  tt add "New idea @someday"

In the TUI, press @ on a selected task to open the section
picker and reassign quickly.`,
		},
		{
			Title:    "Tags",
			Category: "Organization",
			Content: `# Creating Tags

Tags are lightweight labels for cross-cutting categorization.
Create them inline when adding tasks, or add them to existing
tasks.

  tt add "Setup CI #devops #infrastructure"
  tt task tag <id> backend

# Filtering by Tag

Filter your task list to show only tasks with a specific tag.
This works in both CLI and TUI.

  tt task list --tag backend

In the TUI, use the filter view (f key) to filter by tags
across all tasks in the current project.`,
		},
		{
			Title:    "Contexts",
			Category: "Organization",
			Content: `# What Are Contexts

Contexts group related projects together. You might have a
"work" context, a "personal" context, and a "side-projects"
context. Switching contexts changes which projects appear
in your dashboard.

  tt context new work
  tt context new personal

# Managing Projects in Contexts

  tt context add work "Project Alpha"
  tt context add work "Project Beta"

# Switching Contexts

  tt context select work

When a context is active, only projects in that context
appear in the dashboard and project list.`,
		},
		{
			Title:    "Notes",
			Category: "Organization",
			Content: `# Project Notes

Each project can have associated notes for documentation,
meeting minutes, or reference material. Notes are stored
as markdown files alongside your tasks.

  tt note add "Sprint Planning Notes"
  tt note list

# Global Notes

Global notes are not tied to any project. Use them for
cross-project references or general documentation.

  tt note add --global "Architecture Decisions"

# Linking Notes to Tasks

Notes can be linked to specific tasks for context. This
creates a bidirectional reference visible in both the
note and task views.`,
		},

		// Automation
		{
			Title:    "Workflow Rules",
			Category: "Automation",
			Content: `# What Are Workflow Rules

Workflow rules automate repetitive actions. When a trigger
condition is met, the associated action executes automatically.
Triggers can be state changes or events.

  tt workflow add

# Practical Examples

Auto-move tasks to Active section when started:

  Trigger: status changes to in_progress
  Action: set section to active

Auto-tag completed tasks:

  Trigger: status changes to done
  Action: add tag "completed"

# Managing Rules

  tt workflow list
  tt workflow delete <id>

In the TUI, access workflow rules through the command palette
or the project settings.`,
		},
		{
			Title:    "Recurrence",
			Category: "Automation",
			Content: `# Recurring Tasks

Create tasks that automatically repeat on a schedule. When a
recurring task is completed, a new instance is created with
the next due date.

  tt task add --recur daily "Daily standup"
  tt task add --recur weekly "Weekly review"
  tt task add --recur 3d "Check metrics every 3 days"
  tt task add --recur 2w "Biweekly retrospective"

# Recurrence Patterns

Supported patterns include daily, weekly, monthly, and custom
intervals using number + unit: d for days, w for weeks.

# How It Works

When you complete a recurring task, tt creates a new copy with
the status reset to todo and the due date advanced by the
recurrence interval. The completed instance remains in your
history.`,
		},
		{
			Title:    "Snooze",
			Category: "Automation",
			Content: `# Snoozing Tasks

Snooze a task to temporarily hide it from active views. The
task reappears automatically when the snooze period expires.

  tt task snooze <id> 2d
  tt task snooze <id> 1w

# How Snoozed Tasks Reappear

Snoozed tasks are excluded from normal list views and the
dashboard. When the snooze time passes, the task returns to
its previous section and appears in your task list again.

# Use Cases

Snooze is useful for tasks you cannot act on right now but
do not want to forget. Instead of leaving them in your active
list as noise, snooze them until the right time.`,
		},

		// Storage & Advanced
		{
			Title:    "Markdown Format",
			Category: "Storage & Advanced",
			Content: `# How Tasks Are Stored

tt stores all tasks in human-readable markdown files. Each
project has a tasks.md file in the data directory. You can
read and even edit these files directly if needed.

# Heading Depth = Hierarchy

Task hierarchy is represented by heading depth. Epics use ##,
stories use ###, and leaf tasks use #### (relative to their
parent depth).

  ## Epic: User Authentication
  <!-- id: abc123, status: in_progress, priority: 1 -->

  Description of the epic goes here.

  ### Story: Login Flow
  <!-- id: def456, status: todo, priority: 2 -->

# Metadata in HTML Comments

Task metadata like ID, status, priority, tags, and dates is
stored in HTML comments immediately after the heading. This
keeps the markdown readable while preserving all task data.`,
		},
		{
			Title:    "Search",
			Category: "Storage & Advanced",
			Content: `# Searching Tasks

Search across all tasks in the current project or globally
across all projects.

  tt search "login bug"
  tt search --all "authentication"

# Filtering Options

Combine search with filters to narrow results by status,
priority, tag, or section.

  tt task list --status todo --tag backend
  tt task list --priority 1

# TUI Search

In the TUI, press Ctrl+F or use the command palette to
open global search. Results update as you type and you
can jump directly to any matching task.`,
		},
	}
}
