package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// renderProjectTitle renders the project title line with context, notes indicator,
// filter badges, and archived status. Shared across all project-level views.
func renderProjectTitle(project *models.Project, cfg *config.Config) string {
	var b strings.Builder
	title := fmt.Sprintf("%s - Tasks", project.Name)
	b.WriteString(TitleStyle.Render(title))
	if project.Context != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [%s]", project.Context)))
	}
	if storage.NotesExist(project) {
		b.WriteString(" ✎ Notes")
	}
	if cfg.FilterCustomContext != "" {
		b.WriteString(" ")
		b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[+ %s]", cfg.FilterCustomContext)))
	} else if cfg.FilterContext != "" {
		b.WriteString(" ")
		b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[%s]", cfg.FilterContext)))
	}
	if project.Archived {
		b.WriteString(" ")
		b.WriteString(ArchivedBadgeStyle.Render("ARCHIVED"))
		if project.ArchivedAt != nil {
			b.WriteString(" ")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("since %s", project.ArchivedAt.Format(util.DateFormat))))
		}
	}
	return b.String()
}
