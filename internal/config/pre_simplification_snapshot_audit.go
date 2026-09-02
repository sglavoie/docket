package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"time"
)

func renderPreSimplificationV5Audit(createdAt time.Time, inventory *preSimplificationV5Inventory) []byte {
	var archivedBytes, attachmentBytes int64
	for _, source := range inventory.archived {
		archivedBytes += source.Size
	}
	for _, source := range inventory.attachments {
		attachmentBytes += source.Size
	}

	var out strings.Builder
	out.WriteString("# Pre-simplification v5 migration audit\n\n")
	fmt.Fprintf(&out, "- Created at: `%s`\n", createdAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&out, "- Archived files: %d\n", len(inventory.archived))
	fmt.Fprintf(&out, "- Archived source bytes: %d\n", archivedBytes)
	fmt.Fprintf(&out, "- Attachment binaries retained in place: %d\n", len(inventory.attachments))
	fmt.Fprintf(&out, "- Retained attachment bytes: %d\n", attachmentBytes)
	out.WriteString("\nThe archive is a byte-exact rollback source. Attachment binaries were checksummed but remain in their original workspace locations.\n")

	out.WriteString("\n## Archived files\n\n")
	out.WriteString("| Archive path | Kind | Bytes | SHA-256 |\n")
	out.WriteString("| --- | --- | ---: | --- |\n")
	for _, source := range inventory.archived {
		fmt.Fprintf(
			&out,
			"| `%s` | %s | %d | `%s` |\n",
			escapePreSimplificationV5AuditCell(source.Path),
			escapePreSimplificationV5AuditCell(source.Kind),
			source.Size,
			source.SHA256,
		)
	}

	out.WriteString("\n## Attachment binaries retained in place\n\n")
	if len(inventory.attachments) == 0 {
		out.WriteString("No attachment binaries were found.\n")
		return []byte(out.String())
	}
	out.WriteString("| Workspace path | Bytes | SHA-256 |\n")
	out.WriteString("| --- | ---: | --- |\n")
	for _, source := range inventory.attachments {
		fmt.Fprintf(
			&out,
			"| `%s` | %d | `%s` |\n",
			escapePreSimplificationV5AuditCell(source.Path),
			source.Size,
			source.SHA256,
		)
	}
	return []byte(out.String())
}

func escapePreSimplificationV5AuditCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func buildPreSimplificationV5Archive(sources []preSimplificationV5Source, audit []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create snapshot compressor: %w", err)
	}
	gzipWriter.ModTime = time.Time{}
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)

	writeEntry := func(name string, data []byte) error {
		header := &tar.Header{
			Name:       name,
			Mode:       int64(FilePermissions),
			Size:       int64(len(data)),
			ModTime:    time.Unix(0, 0).UTC(),
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Typeflag:   tar.TypeReg,
			Format:     tar.FormatPAX,
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
			return nil, fmt.Errorf("write snapshot archive entry %s: %w", source.Path, err)
		}
	}
	if err := writeEntry(preSimplificationV5AuditEntry, audit); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("write snapshot audit entry: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("finalize snapshot tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("finalize snapshot gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

func publicPreSimplificationV5Files(sources []preSimplificationV5Source) []PreSimplificationV5SnapshotFile {
	files := make([]PreSimplificationV5SnapshotFile, len(sources))
	for i := range sources {
		files[i] = sources[i].PreSimplificationV5SnapshotFile
	}
	return files
}
