package template

import "github.com/sglavoie/tt/internal/models"

// GetDefaultTemplates returns the built-in default task templates.
// All default templates are created disabled so users can review and enable selectively.
func GetDefaultTemplates() []TaskTemplate {
	return []TaskTemplate{
		// Category: Bug
		{
			ID:        "def-bug-report",
			Name:      "Bug Report",
			Enabled:   false,
			Source:    "default",
			Category:  "bug",
			Type:      models.TaskTypeBug,
			Tags:      []string{"bug"},
			Checklist: []string{"Steps to Reproduce", "Expected Behavior", "Actual Behavior", "Environment"},
		},
		{
			ID:        "def-production-incident",
			Name:      "Production Incident",
			Enabled:   false,
			Source:    "default",
			Category:  "bug",
			Type:      models.TaskTypeBug,
			Priority:  1,
			Tags:      []string{"incident", "urgent"},
			Section:   models.SectionNext,
			Checklist: []string{"Impact Assessment", "Root Cause", "Mitigation", "Post-mortem"},
		},

		// Category: Feature
		{
			ID:        "def-feature-request",
			Name:      "Feature Request",
			Enabled:   false,
			Source:    "default",
			Category:  "feature",
			Type:      models.TaskTypeStory,
			Tags:      []string{"feature"},
			Checklist: []string{"User Story", "Acceptance Criteria", "Technical Notes"},
			Children: []TemplateChild{
				{Title: "Implementation"},
				{Title: "Write tests"},
				{Title: "Update documentation", Optional: true},
			},
		},
		{
			ID:       "def-api-endpoint",
			Name:     "API Endpoint",
			Enabled:  false,
			Source:   "default",
			Category: "feature",
			Type:     models.TaskTypeStory,
			Tags:     []string{"api", "backend"},
			Children: []TemplateChild{
				{Title: "Define contract"},
				{Title: "Implement handler"},
				{Title: "Add tests"},
				{Title: "Update docs", Optional: true},
			},
		},
		{
			ID:       "def-ui-component",
			Name:     "UI Component",
			Enabled:  false,
			Source:   "default",
			Category: "feature",
			Type:     models.TaskTypeStory,
			Tags:     []string{"frontend", "ui"},
			Children: []TemplateChild{
				{Title: "Design review"},
				{Title: "Implementation"},
				{Title: "Unit tests"},
				{Title: "Accessibility audit", Optional: true},
			},
		},

		// Category: Process
		{
			ID:        "def-code-review",
			Name:      "Code Review Checklist",
			Enabled:   false,
			Source:    "default",
			Category:  "process",
			Type:      models.TaskTypeTask,
			Tags:      []string{"review"},
			Checklist: []string{"Logic", "Tests", "Edge Cases", "Performance", "Docs"},
		},
		{
			ID:       "def-release-checklist",
			Name:     "Release Checklist",
			Enabled:  false,
			Source:   "default",
			Category: "process",
			Type:     models.TaskTypeStory,
			Tags:     []string{"release"},
			Children: []TemplateChild{
				{Title: "Run tests"},
				{Title: "Update changelog"},
				{Title: "Deploy staging"},
				{Title: "Smoke tests"},
				{Title: "Deploy production"},
			},
		},
		{
			ID:       "def-sprint-planning",
			Name:     "Sprint Planning",
			Enabled:  false,
			Source:   "default",
			Category: "process",
			Type:     models.TaskTypeEpic,
			Tags:     []string{"planning"},
			Children: []TemplateChild{
				{Title: "Backlog grooming", Type: models.TaskTypeStory},
				{Title: "Sprint goal definition", Type: models.TaskTypeStory},
				{Title: "Capacity planning"},
			},
		},
	}
}
