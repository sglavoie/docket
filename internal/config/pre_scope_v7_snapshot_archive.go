package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func precheckPreScopeV7Targets(paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect pre-scope v7 snapshot destination %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("pre-scope v7 snapshot destination is a symlink, refusing to replace: %s", path)
		default:
			return fmt.Errorf("pre-scope v7 snapshot destination already exists, refusing to overwrite: %s", path)
		}
	}
	return nil
}

//nolint:govet // Walk callbacks use scoped errors independently of the outer traversal result.
func inventoryPreScopeV7(root string, captureData bool) (*preScopeV7Inventory, error) {
	inventory := &preScopeV7Inventory{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if err := validatePreSimplificationV5RelativePath(rel); err != nil {
			return fmt.Errorf("unsafe pre-scope v7 source path %s: %w", path, err)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-scope v7 snapshot refuses symlink source: %s", path)
		}
		if isPreConsolidationV6Excluded(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect pre-scope v7 source %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pre-scope v7 snapshot refuses non-regular source: %s", path)
		}
		source, err := readPreScopeV7Source(
			path,
			filepath.ToSlash(filepath.Join("workspace", rel)),
			preConsolidationV6Kind(rel),
			captureData,
		)
		if err != nil {
			return err
		}
		inventory.files = append(inventory.files, source)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory workspace for pre-scope v7 snapshot: %w", err)
	}

	sort.Slice(inventory.files, func(left, right int) bool {
		return inventory.files[left].Path < inventory.files[right].Path
	})
	for index := 1; index < len(inventory.files); index++ {
		if inventory.files[index-1].Path == inventory.files[index].Path {
			return nil, fmt.Errorf(
				"pre-scope v7 archive path collision %q from %s and %s",
				inventory.files[index].Path,
				inventory.files[index-1].sourcePath,
				inventory.files[index].sourcePath,
			)
		}
	}
	return inventory, nil
}

func readPreScopeV7Source(
	path string,
	archivePath string,
	kind string,
	captureData bool,
) (preScopeV7Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return preScopeV7Source{}, fmt.Errorf("read pre-scope v7 source %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	source := preScopeV7Source{
		PreScopeV7SnapshotFile: PreScopeV7SnapshotFile{
			Path: archivePath, Kind: kind, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
		},
		sourcePath: filepath.Clean(path),
	}
	if captureData {
		source.data = data
	}
	return source, nil
}

func comparePreScopeV7Inventories(expected *preScopeV7Inventory, current *preScopeV7Inventory) error {
	if len(expected.files) != len(current.files) {
		return fmt.Errorf("pre-scope v7 snapshot aborted: source inventory changed while snapshotting")
	}
	for index := range expected.files {
		left, right := expected.files[index], current.files[index]
		if left.Path != right.Path || left.sourcePath != right.sourcePath ||
			left.Size != right.Size || left.SHA256 != right.SHA256 {
			return fmt.Errorf("pre-scope v7 snapshot aborted: source changed while snapshotting: %s", left.sourcePath)
		}
	}
	return nil
}

func renderPreScopeV7Audit(
	createdAt time.Time,
	plan V7WorkspaceConversionPlan,
	inventory *preScopeV7Inventory,
) []byte {
	var totalBytes int64
	kindCounts := make(map[string]int)
	for _, source := range inventory.files {
		totalBytes += source.Size
		kindCounts[source.Kind]++
	}
	actionCounts := make(map[string]int)
	droppedScopes := 0
	for _, entry := range plan.Audit {
		actionCounts[entry.Action]++
		if entry.Field == "scope_definition" {
			droppedScopes++
		}
	}
	parked := 0
	tagged := 0
	for _, project := range plan.Catalog.Projects {
		if len(project.Tags) > 0 {
			tagged++
		}
		for _, tag := range project.Tags {
			if tag == models.ParkedProjectTag {
				parked++
				break
			}
		}
	}

	var out strings.Builder
	out.WriteString("# Pre-scope schema-7 migration audit\n\n")
	fmt.Fprintf(&out, "- Created at: `%s`\n", createdAt.UTC().Format(time.RFC3339))
	out.WriteString("- Schema-7 candidate validation: complete\n")
	fmt.Fprintf(&out, "- Snapshotted files: %d\n", len(inventory.files))
	fmt.Fprintf(&out, "- Snapshotted source bytes: %d\n", totalBytes)
	fmt.Fprintf(&out, "- Project task documents: %d\n", kindCounts["project document"])
	fmt.Fprintf(&out, "- Notes: %d\n", kindCounts["note"])
	fmt.Fprintf(&out, "- Attachments: %d\n", kindCounts["attachment"])
	fmt.Fprintf(&out, "- Cataloged projects: %d\n", len(plan.Catalog.Projects))
	fmt.Fprintf(&out, "- Projects carrying at least one tag: %d\n", tagged)
	fmt.Fprintf(&out, "- Projects parked by the reserved `%s` tag: %d\n", models.ParkedProjectTag, parked)
	fmt.Fprintf(&out, "- Retired grouping definitions discarded: %d\n", droppedScopes)
	fmt.Fprintf(&out, "- Retired names renamed to avoid the reserved tag: %d\n", actionCounts["renamed"])
	fmt.Fprintf(&out, "- Retired names rewritten to canonical tag bytes: %d\n", actionCounts["sanitized"])
	fmt.Fprintf(&out, "- Retired names that folded onto one tag: %d\n", actionCounts["deduped"]+actionCounts["merged"])

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

	out.WriteString("\n## Schema-7 project tags\n\n")
	out.WriteString("| Project | Name | Archived | Tags |\n")
	out.WriteString("| --- | --- | --- | --- |\n")
	for _, project := range plan.Catalog.Projects {
		fmt.Fprintf(&out, "| `%s` | %s | %t | %s |\n",
			escapePreSimplificationV5AuditCell(project.ID),
			escapePreSimplificationV5AuditCell(project.Name),
			project.Archived,
			escapePreSimplificationV5AuditCell(strings.Join(project.Tags, " ")),
		)
	}

	out.WriteString("\n## Conversion decisions\n\n")
	if len(plan.Audit) == 0 {
		out.WriteString("None.\n")
		return []byte(out.String())
	}
	out.WriteString("| Project | Field | Action | From | To | Reason |\n")
	out.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range plan.Audit {
		fmt.Fprintf(&out, "| `%s` | `%s` | %s | %s | %s | %s |\n",
			escapePreSimplificationV5AuditCell(entry.ProjectID),
			escapePreSimplificationV5AuditCell(entry.Field),
			escapePreSimplificationV5AuditCell(entry.Action),
			escapePreSimplificationV5AuditCell(entry.From),
			escapePreSimplificationV5AuditCell(entry.To),
			escapePreSimplificationV5AuditCell(entry.Reason),
		)
	}
	return []byte(out.String())
}

func buildPreScopeV7Archive(sources []preScopeV7Source, audit []byte) ([]byte, error) {
	return buildWorkspaceSnapshotArchive(sources, audit, preScopeV7AuditEntry)
}

func buildWorkspaceSnapshotArchive(sources []preScopeV7Source, audit []byte, auditEntry string) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create pre-scope v7 snapshot compressor: %w", err)
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
			return nil, fmt.Errorf("write pre-scope v7 archive entry %s: %w", source.Path, err)
		}
	}
	if err := writeEntry(auditEntry, audit); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("write pre-scope v7 audit entry: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("finalize pre-scope v7 snapshot tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("finalize pre-scope v7 snapshot gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

func publicPreScopeV7Files(sources []preScopeV7Source) []PreScopeV7SnapshotFile {
	files := make([]PreScopeV7SnapshotFile, len(sources))
	for index := range sources {
		files[index] = sources[index].PreScopeV7SnapshotFile
	}
	return files
}
