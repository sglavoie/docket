package storage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
)

// Task markdown schema versions. Schema v1 is the implicit legacy format that
// carries the per-task status/section/stage/active lifecycle fields and has no
// version marker. Schema v2 adds an explicit project schema marker and persists
// a single canonical `state` field instead, while preserving unknown header
// blocks losslessly for forward compatibility.
const (
	SchemaVersionLegacy = 1
	SchemaVersionV2     = 2
)

// schemaMarkerRegex matches the v2 project schema marker, e.g. "<!-- tt-schema: 2 -->".
var schemaMarkerRegex = regexp.MustCompile(`^<!--\s*tt-schema:\s*(\d+)\s*-->\s*$`)

// knownHeaderCommentRegexes are the recognized project-level HTML comment lines
// in the header region (before the first task). Any other HTML comment found in
// the header is treated as an opaque block and preserved verbatim.
var knownHeaderCommentRegexes = []*regexp.Regexp{
	schemaMarkerRegex,
	sectionsRegex,
}

// OpaqueBlock is a verbatim slice of header content that the structured parser
// does not recognize. Preserving it lets schema-v2 documents round-trip unknown
// metadata losslessly rather than silently dropping it on the next save.
type OpaqueBlock struct {
	// Content is the exact source line (without its trailing newline).
	Content string
}

// ProjectDocument is the storage-boundary representation of a project's
// tasks.md: canonical tasks and project metadata plus any opaque header blocks
// the structured parser could not interpret, together with the detected schema
// version and a canonical content revision. Retired layout metadata remains
// private to the decoder.
type ProjectDocument struct {
	ProjectName   string
	Tasks         []*models.Task
	SchemaVersion int
	OpaqueBlocks  []OpaqueBlock
	// Revision is the canonical content hash of this document. Two documents
	// with identical logical content share a revision regardless of source
	// formatting.
	Revision            string
	Warnings            []ParseWarning
	legacySections      []string
	legacySectionColors map[string]string
}

// ParseProjectDocument parses the live schema-v5 tasks.md format. Older task
// schemas are accepted only by the migration-specific decoder and are rejected
// at this runtime boundary.
func ParseProjectDocument(content string) (*ProjectDocument, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	schemaVersion, _, err := scanDocumentHeader(content)
	if err != nil {
		return nil, err
	}
	if schemaVersion != SchemaVersionV5 {
		return nil, fmt.Errorf(
			"task schema version %d requires the snapshot-first workspace migration to schema %d",
			schemaVersion,
			SchemaVersionV5,
		)
	}
	return parseLegacyAwareProjectDocument(content)
}

// parseLegacyAwareProjectDocument is retained for legacy codec tests. Runtime
// loading and conflict handling call ParseProjectDocument and therefore accept
// schema 5 only; migration reads v4 through format.DecodeLegacyV4TaskDocument.
func parseLegacyAwareProjectDocument(content string) (*ProjectDocument, error) {
	// Mirror the parser's BOM handling so header detection lines up with the
	// structured parse.
	content = strings.TrimPrefix(content, "\uFEFF")

	schemaVersion, opaque, err := scanDocumentHeader(content)
	if err != nil {
		return nil, err
	}
	if schemaVersion == SchemaVersionV5 {
		v5, parseErr := ParseV5ProjectDocument(content)
		if parseErr != nil {
			return nil, parseErr
		}
		tasks, conversionErr := ModelsFromV5ProjectDocument(v5, time.Local)
		if conversionErr != nil {
			return nil, conversionErr
		}
		doc := &ProjectDocument{
			ProjectName:   v5.ProjectName,
			Tasks:         tasks,
			SchemaVersion: SchemaVersionV5,
		}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if schemaVersion > SchemaVersionV2 {
		return nil, fmt.Errorf(
			"task schema version %d is not supported; restore the pre-simplification v5 snapshot to use an older tt release",
			schemaVersion,
		)
	}

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		return nil, err
	}

	doc := &ProjectDocument{
		ProjectName:         result.ProjectName,
		Tasks:               result.Tasks,
		SchemaVersion:       schemaVersion,
		OpaqueBlocks:        opaque,
		Warnings:            result.Warnings,
		legacySections:      result.legacySections,
		legacySectionColors: result.legacySectionColors,
	}
	doc.Revision = doc.ComputeRevision()
	return doc, nil
}

// scanDocumentHeader reads the header region (up to the first task heading) and
// returns the detected schema version and any unrecognized HTML comment lines as
// opaque blocks. Absence of the schema marker means legacy v1.
func scanDocumentHeader(content string) (int, []OpaqueBlock, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), scannerMaxBufSize)

	schemaVersion := SchemaVersionLegacy
	var opaque []OpaqueBlock

	for scanner.Scan() {
		line := scanner.Text()
		// The header ends at the first task heading; everything below belongs
		// to the structured task parser.
		if headingRegex.MatchString(line) {
			break
		}

		if m := schemaMarkerRegex.FindStringSubmatch(line); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil {
				schemaVersion = v
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "<!--") {
			// Non-comment header lines (project title, prose, blanks) are
			// reproduced by the structured writer or intentionally dropped;
			// only unknown HTML comments need explicit preservation.
			continue
		}
		if isKnownHeaderComment(line) {
			continue
		}
		opaque = append(opaque, OpaqueBlock{Content: line})
	}

	if err := scanner.Err(); err != nil {
		return 0, nil, fmt.Errorf("%w: %w", tterrors.ErrParseError, err)
	}
	return schemaVersion, opaque, nil
}

func isKnownHeaderComment(line string) bool {
	for _, re := range knownHeaderCommentRegexes {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// ComputeRevision returns the SHA-256 (hex) of the document's canonical
// rendering.
func (d *ProjectDocument) ComputeRevision() string {
	sum := sha256.Sum256([]byte(d.canonicalContent()))
	return hex.EncodeToString(sum[:])
}

// canonicalContent renders the document in its selected canonical schema.
func (d *ProjectDocument) canonicalContent() string {
	if d.SchemaVersion == SchemaVersionV5 {
		if content, err := d.writeV5(); err == nil {
			return content
		}
	}
	return d.render()
}

// WriteV2 renders the document as schema-v2 markdown: the schema marker,
// canonical project metadata, preserved opaque blocks, and tasks serialized
// with the canonical `state` field instead of legacy lifecycle fields.
func (d *ProjectDocument) WriteV2() string {
	return d.render()
}

// WriteCurrent renders the strict schema selected by the document. Runtime
// documents are schema 5; WriteV2 remains available only to migration fixtures
// and legacy codec tests.
func (d *ProjectDocument) WriteCurrent() (string, error) {
	if d.SchemaVersion == SchemaVersionV5 {
		return d.writeV5()
	}
	return d.render(), nil
}

func (d *ProjectDocument) writeV5() (string, error) {
	project := &models.Project{Name: d.ProjectName, Tasks: d.Tasks}
	v5, err := V5ProjectDocumentFromModel(project)
	if err != nil {
		return "", err
	}
	return WriteV5ProjectDocument(v5)
}

// render serializes the document as schema v2 unconditionally: it writes the
// schema marker, preserves opaque header blocks, and serializes tasks with the
// canonical `state` writer. The legacy v1 rendering has been retired — v2 is the
// sole write format.
func (d *ProjectDocument) render() string {
	header := func(b *strings.Builder) {
		fmt.Fprintf(b, "<!-- tt-schema: %d -->\n", SchemaVersionV2)
		for _, block := range d.OpaqueBlocks {
			b.WriteString(block.Content)
			b.WriteString("\n")
		}
	}
	return writeTasksMarkdownDocument(d.ProjectName, d.Tasks, header, format.WriteMetadataV2)
}

// ToProjectDocument builds a live schema-v5 document from an already-loaded
// project. Opaque legacy header blocks are intentionally excluded.
func ToProjectDocument(project *models.Project, opaque []OpaqueBlock) *ProjectDocument {
	doc := &ProjectDocument{
		ProjectName:   project.Name,
		Tasks:         project.Tasks,
		SchemaVersion: SchemaVersionV5,
	}
	doc.Revision = doc.ComputeRevision()
	return doc
}

// ProjectDocumentFromBase overlays a project's current structured state onto the
// exact document it was loaded from. The base remains immutable: only its opaque
// header blocks are copied into the new local document. Keeping the whole base on
// ChangeSet (rather than only []OpaqueBlock) also gives the commit boundary the
// field-level merge base it needs when an expected revision becomes stale.
func ProjectDocumentFromBase(project *models.Project, base *ProjectDocument) *ProjectDocument {
	doc := ToProjectDocument(project, nil)
	doc.Revision = doc.ComputeRevision()
	return doc
}

// CanonicalProjectDocument returns the fixed-point schema-v5 document that will
// be observed after a write and immediate reload. Rendering and parsing once
// intentionally applies every persisted normalization (timestamp precision,
// set de-duplication, hierarchy ordering) before the commit result is computed,
// so the reported revision can never describe a richer in-memory value than the
// bytes actually staged by the journal.
func CanonicalProjectDocument(project *models.Project, base *ProjectDocument) (*ProjectDocument, error) {
	doc := ProjectDocumentFromBase(project, base)
	doc.SchemaVersion = SchemaVersionV5
	doc.OpaqueBlocks = nil
	content, err := doc.WriteCurrent()
	if err != nil {
		return nil, fmt.Errorf("rendering canonical project document: %w", err)
	}
	canonical, err := ParseProjectDocument(content)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing project document: %w", err)
	}
	return canonical, nil
}

// LoadProjectDocument reads a project's tasks.md into a ProjectDocument, the
// lossless base retained for later merging. It reads through the filesystem
// directly (the transactional repository, sessions 12+, will own richer
// loading); a missing file yields an empty v5 document.
func LoadProjectDocument(project *models.Project) (*ProjectDocument, error) {
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		doc := &ProjectDocument{
			ProjectName:   project.Name,
			Tasks:         []*models.Task{},
			SchemaVersion: SchemaVersionV5,
		}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}

// SaveProjectDocumentV2 is retained only for legacy codec tests and migration
// fixtures. Live persistence writes schema 5.
func SaveProjectDocumentV2(project *models.Project, opaque []OpaqueBlock) error {
	if err := auditProjectBeforeSave(project); err != nil {
		return err
	}
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	doc := ToProjectDocument(project, opaque)
	if err := writeFileAtomic(tasksFilePath(project.Path), []byte(doc.WriteV2()), FilePermissions); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}
	return nil
}
