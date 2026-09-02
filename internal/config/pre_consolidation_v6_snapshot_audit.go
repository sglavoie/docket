package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"time"
)

func renderPreConsolidationV6Audit(
	createdAt time.Time,
	plan V6WorkspaceConversionPlan,
	inventory *preConsolidationV6Inventory,
	facts []preConsolidationV6TaskFact,
) []byte {
	var totalBytes int64
	kindCounts := make(map[string]int)
	for _, source := range inventory.files {
		totalBytes += source.Size
		kindCounts[source.Kind]++
	}
	stateCounts := make(map[string]int)
	priorityCounts := make(map[int]int)
	decisionCount := 0
	for _, fact := range facts {
		stateCounts[fact.state]++
		priorityCounts[fact.priority]++
		decisionCount += fact.decisions
	}
	dependencyCount := 0
	recurrenceCount := 0
	for _, entry := range plan.TaskAudit {
		switch {
		case strings.HasPrefix(entry.Field, "depends_on["):
			dependencyCount++
		case entry.Field == "recurrence":
			recurrenceCount++
		}
	}
	planningRepairs := filterV6DomainAudit(plan.Domains.Audit, "planning", "")
	unfiledChanges := filterV6DomainAudit(plan.Domains.Audit, "workspace", "")
	customContexts := filterV6DomainAudit(plan.Domains.Audit, "machine", "custom_context")

	var out strings.Builder
	out.WriteString("# Pre-consolidation schema-6 migration audit\n\n")
	fmt.Fprintf(&out, "- Created at: `%s`\n", createdAt.UTC().Format(time.RFC3339))
	out.WriteString("- Schema-6 candidate validation: complete\n")
	fmt.Fprintf(&out, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&out, "- Snapshotted source bytes: %d\n", totalBytes)
	fmt.Fprintf(&out, "- Project task documents: %d\n", kindCounts["project document"])
	fmt.Fprintf(&out, "- Notes: %d\n", kindCounts["note"])
	fmt.Fprintf(&out, "- Attachments: %d\n", kindCounts["attachment"])
	fmt.Fprintf(&out, "- Transaction-state files: %d\n", kindCounts["transaction state"])
	fmt.Fprintf(&out, "- Undo-ledger files: %d\n", kindCounts["undo ledger"])
	fmt.Fprintf(&out, "- Dependency references discarded: %d\n", dependencyCount)
	fmt.Fprintf(&out, "- Recurrences discarded: %d\n", recurrenceCount)
	fmt.Fprintf(&out, "- Decisions converted to comments: %d\n", decisionCount)
	fmt.Fprintf(&out, "- Planning repairs: %d\n", len(planningRepairs))
	fmt.Fprintf(&out, "- Unfiled catalog changes: %d\n", len(unfiledChanges))
	fmt.Fprintf(&out, "- Custom contexts discarded: %d\n", len(customContexts))

	out.WriteString("\n## Source-relative SHA-256 inventory\n\n")
	out.WriteString("| Archive path | Kind | Bytes | SHA-256 |\n")
	out.WriteString("| --- | --- | ---: | --- |\n")
	for _, source := range inventory.files {
		fmt.Fprintf(&out, "| `%s` | %s | %d | `%s` |\n",
			escapePreSimplificationV5AuditCell(source.Path),
			escapePreSimplificationV5AuditCell(source.Kind),
			source.Size,
			source.SHA256,
		)
	}

	out.WriteString("\n## Lifecycle state mappings\n\n")
	out.WriteString("| Schema 5 | Schema 6 lifecycle | Planning effect | Tasks |\n")
	out.WriteString("| --- | --- | --- | ---: |\n")
	stateMappings := []struct{ from, to, planning string }{
		{"inbox", "inbox", "none"},
		{"ready", "backlog", "none"},
		{"next", "backlog", "add to Next"},
		{"in_progress", "doing", "none"},
		{"waiting", "waiting", "none"},
		{"paused", "paused", "none"},
		{"someday", "someday", "none"},
		{"done", "done", "none"},
		{"canceled", "canceled", "none"},
	}
	for _, mapping := range stateMappings {
		fmt.Fprintf(&out, "| `%s` | `%s` | %s | %d |\n", mapping.from, mapping.to, mapping.planning, stateCounts[mapping.from])
	}

	out.WriteString("\n## Priority mappings\n\n")
	out.WriteString("| Schema 5 | Schema 6 | Label | Tasks |\n")
	out.WriteString("| ---: | ---: | --- | ---: |\n")
	priorityMappings := []struct {
		from, to int
		label    string
	}{
		{0, 0, "Unset"}, {1, 1, "High"}, {2, 1, "High"},
		{3, 0, "Unset"}, {4, 3, "Low"}, {5, 3, "Low"},
	}
	for _, mapping := range priorityMappings {
		fmt.Fprintf(&out, "| %d | %d | %s | %d |\n", mapping.from, mapping.to, mapping.label, priorityCounts[mapping.from])
	}

	renderPreConsolidationV6TaskAudit(&out, plan, "discarded", "## Retired task values")
	renderPreConsolidationV6TaskAudit(&out, plan, "converted", "## Decision conversions")
	renderPreConsolidationV6DomainAudit(&out, "## Planning repairs", planningRepairs)
	renderPreConsolidationV6DomainAudit(&out, "## Unfiled changes", unfiledChanges)
	renderPreConsolidationV6DomainAudit(&out, "## Custom-context removals", customContexts)
	return []byte(out.String())
}

func filterV6DomainAudit(entries []V6DomainMigrationAuditEntry, domain, field string) []V6DomainMigrationAuditEntry {
	filtered := make([]V6DomainMigrationAuditEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Domain == domain && (field == "" || entry.Field == field) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func renderPreConsolidationV6TaskAudit(out *strings.Builder, plan V6WorkspaceConversionPlan, action, heading string) {
	out.WriteString("\n" + heading + "\n\n")
	count := 0
	for _, entry := range plan.TaskAudit {
		if entry.Action == action {
			count++
		}
	}
	if count == 0 {
		out.WriteString("None.\n")
		return
	}
	out.WriteString("| Project | Task | Field | Action | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range plan.TaskAudit {
		if entry.Action != action {
			continue
		}
		fmt.Fprintf(out, "| `%s` | `%s` | `%s` | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.TaskID),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
}

func renderPreConsolidationV6DomainAudit(
	out *strings.Builder,
	heading string,
	entries []V6DomainMigrationAuditEntry,
) {
	out.WriteString("\n" + heading + "\n\n")
	if len(entries) == 0 {
		out.WriteString("None.\n")
		return
	}
	out.WriteString("| Domain | Project | Entity | Field | Action | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range entries {
		fmt.Fprintf(out, "| %s | `%s` | `%s` | `%s` | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.Domain),
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.EntityID),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
}

func buildPreConsolidationV6Archive(sources []preConsolidationV6Source, audit []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create pre-consolidation v6 snapshot compressor: %w", err)
	}
	gzipWriter.ModTime = time.Time{}
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	writeEntry := func(name string, data []byte) error {
		header := &tar.Header{
			Name: name, Mode: int64(FilePermissions), Size: int64(len(data)),
			ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatPAX, Typeflag: tar.TypeReg,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err := tarWriter.Write(data)
		return err
	}
	for _, source := range sources {
		if err := writeEntry(source.Path, source.data); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("write pre-consolidation v6 archive entry %s: %w", source.Path, err)
		}
	}
	if err := writeEntry(preConsolidationV6AuditEntry, audit); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("write pre-consolidation v6 audit entry: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("finalize pre-consolidation v6 snapshot tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("finalize pre-consolidation v6 snapshot gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

func publicPreConsolidationV6Files(sources []preConsolidationV6Source) []PreConsolidationV6SnapshotFile {
	files := make([]PreConsolidationV6SnapshotFile, len(sources))
	for index := range sources {
		files[index] = sources[index].PreConsolidationV6SnapshotFile
	}
	return files
}
