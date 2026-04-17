package tui

import "github.com/sglavoie/tt/internal/models"

// ExportProjectMsg triggers a project-to-markdown export.
// If Project is nil, the handler resolves it from the active view.
type ExportProjectMsg struct{ Project *models.Project }
