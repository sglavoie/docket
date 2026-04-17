package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
)

// blockerViewShared renders the shared blocker view.
func blockerViewShared(s *blockerViewState, ds blockerDataSource) string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle(ds.titleText(), fmt.Sprintf("%d active, %d resolved", ds.activeCount(), ds.resolvedCount())))
	b.WriteString("\n\n")

	if s.expandedView {
		return blockerExpandedView(s, ds, &b)
	}

	if s.editMode {
		return blockerEditView(s, ds, &b)
	}

	if s.addMode {
		return blockerAddView(s, ds, &b)
	}

	if s.err != nil {
		b.WriteString(ValidationErrorStyle.Render(s.err.Error()))
		b.WriteString("\n\n")
	}

	visible := ds.getVisibleItems(s.tabs.Active)
	b.WriteString(RenderTabBar(blockersTabNames, s.tabs.Active, min(s.width-6, 54)-2, 1))
	b.WriteString("\n")

	if len(visible) == 0 {
		b.WriteString(blockerEmptyStateText(s.tabs.Active))
		b.WriteString("\n")
	} else {
		blockerListView(s, ds, visible, &b)
	}

	helpText := "a: add · e: edit · R: resolve/unresolve · d: delete · g: go to project · Enter: expand · h/l/[/] tabs · Esc: close"
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, s.width)))

	return b.String()
}

func blockerExpandedView(s *blockerViewState, ds blockerDataSource, b *strings.Builder) string {
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

func blockerEditView(s *blockerViewState, ds blockerDataSource, b *strings.Builder) string {
	blocker := ds.getItemBlocker(s.editIndex)
	b.WriteString(FieldLabelFocusedStyle.Render("Edit Blocker:"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(FormatTimestampLabeled("Created", blocker.CreatedAt)))
	b.WriteString("\n\n")
	applyThemedTextarea(&s.editTextInput)
	b.WriteString(s.editTextInput.View())
	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"))
	return b.String()
}

func blockerAddView(s *blockerViewState, ds blockerDataSource, b *strings.Builder) string {
	b.WriteString(FieldLabelFocusedStyle.Render("Add Blocker:"))
	b.WriteString("\n")
	if ds.showProjectName() {
		project := ds.getAddProject()
		if project != nil {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", project.Name)))
			b.WriteString("\n")
		}
	}
	applyThemedTextarea(&s.textInput)
	b.WriteString(s.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"))
	return b.String()
}

func blockerEmptyStateText(activeTab int) string {
	switch activeTab {
	case blockersTabActive:
		return MutedStyle.Render("No active blockers. Press 'a' to add one.")
	case blockersTabResolved:
		return MutedStyle.Render("No resolved blockers.")
	default:
		return MutedStyle.Render("No blockers. Press 'a' to add one.")
	}
}

func blockerListView(s *blockerViewState, ds blockerDataSource, visible []blockerItem, b *strings.Builder) {
	headerLines := 7
	availableHeight := s.height - headerLines
	maxVisible := max(availableHeight/3, 3)
	scrollOffset := calcScrollOffset(s.cursor, maxVisible)

	for i := scrollOffset; i < len(visible) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(blockerListItemView(s, ds, visible[i], i == s.cursor))
	}

	if len(visible) <= maxVisible {
		return
	}

	remaining := len(visible) - scrollOffset - maxVisible
	if remaining > 0 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
		b.WriteString("\n")
	}
}

func blockerListItemView(s *blockerViewState, ds blockerDataSource, item blockerItem, selected bool) string {
	blocker := item.Blocker
	relativeTime := storage.FormatRelativeTime(blocker.CreatedAt)

	var line strings.Builder
	line.WriteString("  ")

	if blocker.IsResolved() {
		line.WriteString(DoneStyle.Render("✓ "))
		if selected {
			line.WriteString(MutedStyle.Render(blocker.Title))
		} else {
			line.WriteString(MutedStyle.Render(blocker.Title))
		}
	} else {
		line.WriteString(DueSoonStyle.Render("🚧 "))
		if selected {
			line.WriteString(BoldStyle.Render(blocker.Title))
		} else {
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
		desc := strings.ReplaceAll(blocker.Description, "\n", " ")
		desc = truncateToVisualWidth(desc, max(s.width-10, 1))
		line.WriteString(" - ")
		line.WriteString(MutedStyle.Render(desc))
	}
	line.WriteString("\n")

	return line.String()
}
