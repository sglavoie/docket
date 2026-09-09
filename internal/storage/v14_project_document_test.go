package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

func v14TestDependencies() []models.TaskReference {
	return []models.TaskReference{
		{ProjectID: "project-beta", TaskID: "task-9"},
		{ProjectID: "project-alpha", TaskID: "task-1"},
	}
}

// TestLiveProjectDocumentRoundTripsDependencies proves the live boundary — not
// just the codec — carries an ordered dependency list from model to bytes and
// back.
func TestLiveProjectDocumentRoundTripsDependencies(t *testing.T) {
	doc := mergeTestDoc("Deps", mergeTestTask("task-1", "Depends on two"))
	doc.Tasks[0].Dependencies = v14TestDependencies()

	content, err := doc.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	if !strings.Contains(content, schemaMarkerV17) {
		t.Fatalf("live document is not schema 17:\n%s", content)
	}
	if !strings.Contains(content, "depends: project-beta:task-9;project-alpha:task-1") {
		t.Fatalf("live document omitted the dependency list:\n%s", content)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if parsed.SchemaVersion != SchemaVersionV17 {
		t.Fatalf("parsed schema = %d, want %d", parsed.SchemaVersion, SchemaVersionV17)
	}
	if !models.TaskReferencesEqual(parsed.Tasks[0].Dependencies, doc.Tasks[0].Dependencies) {
		t.Fatalf("dependencies = %+v, want %+v", parsed.Tasks[0].Dependencies, doc.Tasks[0].Dependencies)
	}
}

// TestEmptyDependenciesLeaveLiveBytesUnchanged is what makes the schema-15
// migration a no-op for every existing task: adding the field must not move a
// single byte for a task that has no dependencies.
func TestEmptyDependenciesLeaveLiveBytesUnchanged(t *testing.T) {
	doc := mergeTestDoc("Deps", mergeTestTask("task-1", "No dependencies"))
	content, err := doc.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	if strings.Contains(content, "depends") {
		t.Fatalf("an absent dependency list still wrote a key:\n%s", content)
	}
	v13, err := v13ProjectDocumentFromV14Model(t, doc)
	if err != nil {
		t.Fatalf("render schema 13: %v", err)
	}
	if strings.Replace(content, schemaMarkerV17, schemaMarkerV13, 1) != v13 {
		t.Fatalf("live bytes differ from schema 13 beyond the marker\nlive: %s\nv13: %s", content, v13)
	}
}

func v13ProjectDocumentFromV14Model(t *testing.T, doc *ProjectDocument) (string, error) {
	t.Helper()
	lifted, err := currentTaskProjectDocumentFromModel(&models.Project{Name: doc.ProjectName, Tasks: doc.Tasks})
	if err != nil {
		return "", err
	}
	return WriteV13ProjectDocument(v13ProjectDocumentFromV14(v14ProjectDocumentFromV15(v15ProjectDocumentFromV16(v16ProjectDocumentFromV17(lifted)))))
}

func TestV14ProjectDocumentLiftFromV13AddsNoDependencies(t *testing.T) {
	v13 := V13ProjectDocument{ProjectName: "Deps", Tasks: []format.V13TaskRecord{v13ProjectTestRecord()}}
	lifted := V14ProjectDocumentFromV13(v13)
	if len(lifted.Tasks) != 1 || len(lifted.Tasks[0].Dependencies) != 0 {
		t.Fatalf("lift = %+v, want one task with no dependencies", lifted.Tasks)
	}
	published, err := WriteV14ProjectDocument(lifted)
	if err != nil {
		t.Fatalf("WriteV14ProjectDocument: %v", err)
	}
	if strings.Contains(published, "depends") {
		t.Fatalf("lifted document wrote a dependency key:\n%s", published)
	}
	if !strings.Contains(published, schemaMarkerV14) {
		t.Fatalf("lifted document is not schema 14:\n%s", published)
	}
}

func v13ProjectTestRecord() format.V13TaskRecord {
	task := mergeTestTask("task-1", "Lifted")
	record, err := currentTaskRecordFromModel(task)
	if err != nil {
		panic(err)
	}
	return record.V13TaskRecord
}

// TestV14ProjectDocumentRejectsForeignSchemaMarker keeps the live reader strict:
// a schema-13 document is not silently upgraded on read, which is what forces
// every workspace through the audited migration boundary.
func TestV14ProjectDocumentRejectsForeignSchemaMarker(t *testing.T) {
	doc := mergeTestDoc("Deps", mergeTestTask("task-1", "Strict"))
	content, err := doc.WriteCurrent()
	if err != nil {
		t.Fatal(err)
	}
	downgraded := strings.Replace(content, schemaMarkerV17, schemaMarkerV14, 1)
	if _, err := ParseProjectDocument(downgraded); err == nil ||
		!strings.Contains(err.Error(), "unsupported v17 task schema 14") {
		t.Fatalf("ParseProjectDocument error = %v, want an unsupported-schema refusal", err)
	}
}

// TestDependenciesSurviveRevisionAndCanonicalization covers the two derived
// artifacts a dependency edit must reach: the content hash a stale writer is
// compared against, and the canonical document the commit path stores.
func TestDependenciesSurviveRevisionAndCanonicalization(t *testing.T) {
	project := &models.Project{Name: "Deps", Tasks: []*models.Task{mergeTestTask("task-1", "Depends")}}
	bare := ToProjectDocument(project)
	bareRevision := bare.Revision

	project.Tasks[0].Dependencies = v14TestDependencies()
	withDeps := ToProjectDocument(project)
	if withDeps.Revision == bareRevision {
		t.Fatal("adding a dependency did not change the document revision")
	}

	canonical, err := CanonicalProjectDocument(project, withDeps)
	if err != nil {
		t.Fatalf("CanonicalProjectDocument: %v", err)
	}
	if !models.TaskReferencesEqual(canonical.Tasks[0].Dependencies, project.Tasks[0].Dependencies) {
		t.Fatalf("canonical dependencies = %+v, want %+v", canonical.Tasks[0].Dependencies, project.Tasks[0].Dependencies)
	}
	if canonical.Revision != withDeps.Revision {
		t.Fatalf("canonical revision = %q, want %q", canonical.Revision, withDeps.Revision)
	}
}

// TestLiveWriterRefusesAnUnpersistableDependency keeps an unrepresentable
// locator out of the file rather than writing bytes the reader would reject.
func TestLiveWriterRefusesAnUnpersistableDependency(t *testing.T) {
	doc := mergeTestDoc("Deps", mergeTestTask("task-1", "Bad locator"))
	doc.Tasks[0].Dependencies = []models.TaskReference{{ProjectID: "project alpha", TaskID: "task-1"}}
	if _, err := doc.WriteCurrent(); err == nil {
		t.Fatal("WriteCurrent accepted a dependency carrying a reserved character")
	}
}
