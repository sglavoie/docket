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
	tagsRegex,
	pinnedTagsRegex,
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
	Tags          []string
	PinnedTags    []string
	Tasks         []*models.Task
	SchemaVersion int
	OpaqueBlocks  []OpaqueBlock
	// Revision is the canonical content hash of this document. Two documents
	// with identical logical content share a revision regardless of source
	// formatting, because it is computed from the canonical v2 rendering.
	Revision            string
	Warnings            []ParseWarning
	legacySections      []string
	legacySectionColors map[string]string
}

// ParseProjectDocument parses tasks.md content into a storage ProjectDocument.
// It detects the schema version, captures unknown header comment lines as opaque
// blocks, delegates structured task parsing to the shared parser (so both legacy
// v1 and v2 `state` documents load identically for the rest of the domain), and
// computes the canonical revision.
func ParseProjectDocument(content string) (*ProjectDocument, error) {
	// Mirror the parser's BOM handling so header detection lines up with the
	// structured parse.
	content = strings.TrimPrefix(content, "\uFEFF")

	schemaVersion, opaque, err := scanDocumentHeader(content)
	if err != nil {
		return nil, err
	}

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		return nil, err
	}

	doc := &ProjectDocument{
		ProjectName:         result.ProjectName,
		Tags:                result.Tags,
		PinnedTags:          result.PinnedTags,
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

// ComputeRevision returns the canonical content revision for the document: the
// SHA-256 (hex) of its canonical v2 rendering. The v2 rendering is used as the
// canonical form regardless of the document's own schema version so that a
// legacy project and its upgraded v2 counterpart hash identically when their
// logical content matches.
func (d *ProjectDocument) ComputeRevision() string {
	sum := sha256.Sum256([]byte(d.canonicalContent()))
	return hex.EncodeToString(sum[:])
}

// canonicalContent renders the document in canonical v2 form for hashing. It is
// deliberately independent of the on-disk schema version and of incidental
// source formatting.
func (d *ProjectDocument) canonicalContent() string {
	return d.render()
}

// WriteV2 renders the document as schema-v2 markdown: the schema marker,
// canonical project metadata, preserved opaque blocks, and tasks serialized
// with the canonical `state` field instead of legacy lifecycle fields.
func (d *ProjectDocument) WriteV2() string {
	return d.render()
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
	return writeTasksMarkdownDocument(d.ProjectName, d.Tasks, d.Tags, d.PinnedTags, header, format.WriteMetadataV2)
}

// ToProjectDocument builds a v2 ProjectDocument from an already-loaded project,
// preserving the supplied opaque blocks. New transactional callers should use
// ProjectDocumentFromBase so the source of those blocks is the exact document
// retained by the repository snapshot rather than a separately threaded slice.
func ToProjectDocument(project *models.Project, opaque []OpaqueBlock) *ProjectDocument {
	doc := &ProjectDocument{
		ProjectName:   project.Name,
		Tags:          project.Tags,
		PinnedTags:    project.PinnedTags,
		Tasks:         project.Tasks,
		SchemaVersion: SchemaVersionV2,
		OpaqueBlocks:  opaque,
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
	var opaque []OpaqueBlock
	if base != nil {
		opaque = append([]OpaqueBlock(nil), base.OpaqueBlocks...)
	}
	return ToProjectDocument(project, opaque)
}

// CanonicalProjectDocument returns the fixed-point schema-v2 document that will
// be observed after a write and immediate reload. Rendering and parsing once
// intentionally applies every persisted normalization (timestamp precision,
// set de-duplication, hierarchy ordering) before the commit result is computed,
// so the reported revision can never describe a richer in-memory value than the
// bytes actually staged by the journal.
func CanonicalProjectDocument(project *models.Project, base *ProjectDocument) (*ProjectDocument, error) {
	doc := ProjectDocumentFromBase(project, base)
	canonical, err := ParseProjectDocument(doc.WriteV2())
	if err != nil {
		return nil, fmt.Errorf("canonicalizing project document: %w", err)
	}
	return canonical, nil
}

// LoadProjectDocument reads a project's tasks.md into a ProjectDocument, the
// lossless base retained for later merging. It reads through the filesystem
// directly (the transactional repository, sessions 12+, will own richer
// loading); a missing file yields an empty v2 document.
func LoadProjectDocument(project *models.Project) (*ProjectDocument, error) {
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		doc := &ProjectDocument{
			ProjectName:   project.Name,
			Tasks:         []*models.Task{},
			SchemaVersion: SchemaVersionV2,
		}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}

// SaveProjectDocumentV2 writes a project to disk in schema-v2 format. This is
// the opt-in v2 write path: the default SaveProject remains v1 until the
// transactional repository (sessions 12+) makes v2 the canonical write format.
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
