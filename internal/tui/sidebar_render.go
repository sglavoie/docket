package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// renderSidebarCard renders a card with a titled border for sidebar sections.
// Uses the shared renderCard utility with sidebar-specific configuration.
// accentColor is optional - pass empty string for default styling.
func renderSidebarCard(title, content string, width int, focused bool, accentColor string) string {
	return renderSidebarCardWithHeight(title, content, width, focused, accentColor, 0)
}

func renderSidebarCardWithHeight(title, content string, width int, focused bool, accentColor string, height int) string {
	config := DefaultCardConfig()
	config.Focused = focused
	config.AccentColor = GetSectionColorValue(accentColor)
	config.Height = height
	return renderCard(title, content, width, config)
}

// renderSidebarContent renders the detail sidebar for the currently selected task.
func renderSidebarContent(host SidebarHost, task *models.Task, width, height int) string {
	if task == nil {
		emptyCard := renderSidebarCard("Task", MutedStyle.Render("No task selected"), width, false, "")
		return padSidebarToHeight(emptyCard, nil, width, height)
	}

	project := host.SidebarProject()
	cfg := host.SidebarConfig()
	sidebar := host.SidebarGetSidebar()

	// Get section color for accent on Task and Info cards
	sectionColor := project.GetSectionColor(task.Section)
	if task.CustomSection != "" {
		sectionColor = project.GetSectionColor(task.CustomSection)
	}

	// Calculate inner width for card content (width minus card borders and padding)
	cardInnerWidth := width - 6 // 2 border + 2 padding + 2 margin

	var cards []string

	// Task card: title, type/status, ID/priority (uses section color)
	cards = append(cards, renderSidebarTaskCard(project, task, width, sectionColor))

	// Due card (only if due date set)
	if task.DueDate != nil {
		cards = append(cards, renderSidebarDueCard(task, width, cfg.GetTimeFormat()))
	}

	// Dependencies card (blockers are actionable — show early)
	hasDeps := len(task.DependsOn) > 0 || len(project.GetBlockedByTasks(task)) > 0
	if hasDeps {
		cards = append(cards, renderSidebarDependenciesCard(host, task, width, cardInnerWidth))
	}

	// Checklist card (progress tracking — actionable)
	if len(task.Checklist) > 0 {
		cards = append(cards, renderSidebarChecklistCard(task, width))
	}

	// Description card (content — show before metadata)
	if task.Description != "" {
		cards = append(cards, renderSidebarDescriptionCard(task, width))
	}

	// Score card (only if scoring profile assigned)
	if cfg != nil {
		if profile := cfg.GetProjectScoringProfile(project.ID); profile != nil {
			cards = append(cards, renderSidebarScoreCard(task, width, profile))
		}
	}

	// Metadata card: tags + section (uses section color)
	cards = append(cards, renderSidebarMetadataCard(project, task, width, sectionColor))

	// Comments card (only if has comments)
	if len(task.Comments) > 0 {
		cards = append(cards, renderSidebarCommentsCard(task, width))
	}

	// Activity card
	cards = append(cards, renderSidebarActivityCard(task, width))

	// Related tasks card (only if has related tasks)
	if len(task.RelatedTo) > 0 {
		cards = append(cards, renderSidebarRelatedCard(host, task, width, cardInnerWidth))
	}

	// Attachments card (only if has attachments)
	if task.HasAttachments() {
		cards = append(cards, renderSidebarAttachmentsCard(task, width))
	}

	// Help hint at bottom if sidebar focused
	helpHint := ""
	if sidebar.IsFocused() && sidebarHasLinkedTasks(project, task) {
		helpHint = HelpStyle.Render("j/k: nav · h/l: section · Enter: jump · x: unlink")
	} else if sidebar.IsFocused() {
		helpHint = HelpStyle.Render("j/k: scroll · Esc: back")
	}

	// Join cards with no spacing (cards have their own borders)
	content := strings.Join(cards, "\n")
	if helpHint != "" {
		content += "\n" + helpHint
	}

	return padSidebarToHeight(content, sidebar, width, height)
}
