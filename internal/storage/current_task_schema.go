package storage

import "github.com/sglavoie/docket/internal/storage/format"

// This file is the current-version authority for live task Markdown. Live reads
// and writes select their codec here and nowhere else, so advancing the live
// task schema is an edit to this file plus a new codec beside the frozen ones,
// rather than a sweep over the call sites that merely happen to name a version.
//
// The historical codecs keep their own exported version names: a migration has
// to say which schema it reads, and a frozen reader that silently followed the
// current version would stop being frozen. Only the live path is
// version-neutral, and the aliases below are what let it be.

// CurrentTaskSchemaVersion is the schema live task Markdown is parsed and
// written at. ProjectDocument.SchemaVersion carries this value, and
// WriteCurrent refuses any other.
const CurrentTaskSchemaVersion = SchemaVersionV20

// currentTaskProjectDocument is one project's task file at the current schema.
// It is an alias rather than a distinct type so the codec keeps its exported
// version-specific name for migrations while the live path spells only
// "current"; a distinct type would need a conversion at every boundary and buy
// nothing, since the two are the same bytes by construction.
type currentTaskProjectDocument = V20ProjectDocument

// currentTaskRecord is one task record at the current schema. Schema 20
// advances the record grammar to carry the stored hidden flag.
type currentTaskRecord = format.V19TaskRecord

// currentProjectCommentRecord is one stored project comment at the current
// schema.
type currentProjectCommentRecord = format.ProjectCommentRecord

// parseCurrentTaskProjectDocument strictly parses live task Markdown.
func parseCurrentTaskProjectDocument(content string) (currentTaskProjectDocument, error) {
	return ParseV20ProjectDocument(content)
}

// scanCurrentTaskProjectDocument parses live task Markdown into records without
// the document-wide validation; only the repair reader may use it.
func scanCurrentTaskProjectDocument(content string) (currentTaskProjectDocument, error) {
	return scanV20ProjectDocument(content)
}

// writeValidatedCurrentTaskProjectDocument renders a live document that was
// validated as it was built, as currentTaskProjectDocumentFromModel's are.
func writeValidatedCurrentTaskProjectDocument(document currentTaskProjectDocument) (string, error) {
	return writeValidatedV20ProjectDocument(document)
}

// validateCurrentTaskProjectDocument validates a live document without
// rendering it.
func validateCurrentTaskProjectDocument(document currentTaskProjectDocument) error {
	return ValidateV20ProjectDocument(document)
}

// validateCurrentTaskRecord validates one live task record.
func validateCurrentTaskRecord(record currentTaskRecord) error {
	return format.ValidateV19TaskRecord(record)
}
