package config

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/storage/format"
)

func appendV5MigrationTaskAudit(
	target []v5MigrationTaskAuditRow,
	projectID string,
	sourceID string,
	entries []format.V5TaskMigrationAuditEntry,
) []v5MigrationTaskAuditRow {
	for _, entry := range entries {
		target = append(target, v5MigrationTaskAuditRow{
			ProjectID: projectID,
			SourceID:  sourceID,
			Entry:     entry,
		})
	}
	return target
}

func renderV5MigrationConversionAudit(
	domainAudit []V5DomainMigrationAuditEntry,
	noteAudit []storage.V5NoteMigrationAuditEntry,
	taskAudit []v5MigrationTaskAuditRow,
	reintegration storage.V5TaskReintegrationReport,
) []byte {
	var out strings.Builder
	out.WriteString("\n## Conversion and repair report\n")

	out.WriteString("\n### Configuration\n\n")
	if len(domainAudit) == 0 {
		out.WriteString("No configuration conversions or repairs were required.\n")
	} else {
		out.WriteString("| Domain | Entity | Action | Field | From | To | Reason |\n")
		out.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, entry := range domainAudit {
			fmt.Fprintf(
				&out,
				"| %s | %s | %s | %s | %s | %s | %s |\n",
				escapePreSimplificationV5AuditCell(entry.Domain),
				escapePreSimplificationV5AuditCell(entry.EntityID),
				escapePreSimplificationV5AuditCell(entry.Action),
				escapePreSimplificationV5AuditCell(entry.Field),
				escapePreSimplificationV5AuditCell(entry.From),
				escapePreSimplificationV5AuditCell(entry.To),
				escapePreSimplificationV5AuditCell(entry.Reason),
			)
		}
	}

	out.WriteString("\n### Notes\n\n")
	if len(noteAudit) == 0 {
		out.WriteString("No note fields were removed.\n")
	} else {
		out.WriteString("| Note | Action | Field | From | Reason |\n")
		out.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, entry := range noteAudit {
			fmt.Fprintf(
				&out,
				"| %s | %s | %s | %s | %s |\n",
				escapePreSimplificationV5AuditCell(entry.Filename),
				escapePreSimplificationV5AuditCell(entry.Action),
				escapePreSimplificationV5AuditCell(entry.Field),
				escapePreSimplificationV5AuditCell(entry.From),
				escapePreSimplificationV5AuditCell(entry.Reason),
			)
		}
	}

	out.WriteString("\n### Task fields and standalone blockers\n\n")
	if len(taskAudit) == 0 {
		out.WriteString("No task-field conversions were required.\n")
	} else {
		out.WriteString("| Project | Source | Task | Action | Field | From | To | Reason |\n")
		out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		for _, row := range taskAudit {
			fmt.Fprintf(
				&out,
				"| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				escapePreSimplificationV5AuditCell(row.ProjectID),
				escapePreSimplificationV5AuditCell(row.SourceID),
				escapePreSimplificationV5AuditCell(row.Entry.TaskID),
				escapePreSimplificationV5AuditCell(row.Entry.Action),
				escapePreSimplificationV5AuditCell(row.Entry.Field),
				escapePreSimplificationV5AuditCell(row.Entry.From),
				escapePreSimplificationV5AuditCell(row.Entry.To),
				escapePreSimplificationV5AuditCell(row.Entry.Reason),
			)
		}
	}

	out.WriteString("\n### Archive reintegration\n\n")
	fmt.Fprintf(&out, "- Input tasks: %d\n", reintegration.InputTasks)
	fmt.Fprintf(&out, "- Retained tasks: %d\n", reintegration.RetainedTasks)
	fmt.Fprintf(&out, "- Omitted duplicate tasks: %d\n", reintegration.OmittedTasks)
	if len(reintegration.Entries) == 0 {
		out.WriteString("\nNo identity or reference repairs were required.\n")
		return []byte(out.String())
	}
	out.WriteString("\n| Project | Task | Action | Field | Source | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range reintegration.Entries {
		fmt.Fprintf(
			&out,
			"| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.TaskID),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Source),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
	return []byte(out.String())
}
