package storage

import "github.com/sglavoie/tt/internal/storage/format"

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
const CurrentTaskSchemaVersion = SchemaVersionV19

// currentTaskProjectDocument is one project's task file at the current schema.
// It is an alias rather than a distinct type so the codec keeps its exported
// version-specific name for migrations while the live path spells only
// "current"; a distinct type would need a conversion at every boundary and buy
// nothing, since the two are the same bytes by construction.
type currentTaskProjectDocument = V19ProjectDocument

// currentTaskRecord is one task record at the current schema. Schema 19
// advances the record grammar to carry the check-in schedule and the instant
// the task was last reviewed.
type currentTaskRecord = format.V18TaskRecord

// currentProjectCommentRecord is one stored project comment at the current
// schema.
type currentProjectCommentRecord = format.ProjectCommentRecord

// parseCurrentTaskProjectDocument strictly parses live task Markdown.
func parseCurrentTaskProjectDocument(content string) (currentTaskProjectDocument, error) {
	return ParseV19ProjectDocument(content)
}

// scanCurrentTaskProjectDocument parses live task Markdown into records without
// the document-wide validation; only the repair reader may use it.
func scanCurrentTaskProjectDocument(content string) (currentTaskProjectDocument, error) {
	return scanV19ProjectDocument(content)
}

// writeCurrentTaskProjectDocument renders canonical live task Markdown.
func writeCurrentTaskProjectDocument(document currentTaskProjectDocument) (string, error) {
	return WriteV19ProjectDocument(document)
}

// validateCurrentTaskProjectDocument validates a live document without
// rendering it.
func validateCurrentTaskProjectDocument(document currentTaskProjectDocument) error {
	return ValidateV19ProjectDocument(document)
}

// validateCurrentTaskRecord validates one live task record.
func validateCurrentTaskRecord(record currentTaskRecord) error {
	return format.ValidateV18TaskRecord(record)
}
