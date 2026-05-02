package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// NewTemplatePickerModel creates a new template picker model.
func NewTemplatePickerModel(cfg *config.Config, project *models.Project) *TemplatePickerModel {
	templates := cfg.GetEnabledTaskTemplates()

	// Build category order (bug, feature, process, other)
	categoryOrder := []string{}
	categorySet := make(map[string]bool)
	for _, t := range templates {
		cat := t.Category
		if cat == "" {
			cat = "other"
		}
		if !categorySet[cat] {
			categorySet[cat] = true
			categoryOrder = append(categoryOrder, cat)
		}
	}

	// Sort category order: bug, feature, process, then others alphabetically
	sortedOrder := []string{}
	preferredOrder := []string{"bug", "feature", "process"}
	for _, cat := range preferredOrder {
		if categorySet[cat] {
			sortedOrder = append(sortedOrder, cat)
		}
	}
	for _, cat := range categoryOrder {
		if cat != "bug" && cat != "feature" && cat != "process" {
			sortedOrder = append(sortedOrder, cat)
		}
	}

	m := &TemplatePickerModel{
		config:            cfg,
		project:           project,
		templates:         templates,
		filteredTemplates: templates,
		categoryOrder:     sortedOrder,
	}
	m.SetSize(60, 20)
	m.InitSearchInput("Search templates...", 40, 25)
	return m
}

// SetSize sets the view dimensions.
func (m *TemplatePickerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *TemplatePickerModel) Init() tea.Cmd {
	return m.ActivateSearch()
}
