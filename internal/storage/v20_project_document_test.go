package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage/format"
)

// TestV20ProjectDocumentLiftFromV19IsTheMarkerAlone is what makes the
// schema-23 catalog migration a marker-only rewrite: nothing before schema 20
// could hide a task, so every lifted task is unhidden, and the lifted document
// renders exactly the bytes its schema-19 source did apart from the marker.
func TestV20ProjectDocumentLiftFromV19IsTheMarkerAlone(t *testing.T) {
	source := V19ProjectDocument{
		ProjectName: "Hidden",
		Comments:    []format.ProjectCommentRecord{v17TestComment(v17TestCommentIDA, "kept", 0)},
		Tasks:       v19ProjectTestDocument(t).Tasks,
	}
	lifted := V20ProjectDocumentFromV19(source)
	for i, task := range lifted.Tasks {
		if task.Hidden {
			t.Fatalf("lifted task %d was hidden", i)
		}
	}
	published, err := WriteV20ProjectDocument(lifted)
	if err != nil {
		t.Fatalf("WriteV20ProjectDocument: %v", err)
	}
	original, err := WriteV19ProjectDocument(source)
	if err != nil {
		t.Fatalf("WriteV19ProjectDocument: %v", err)
	}
	if strings.Replace(published, schemaMarkerV20, schemaMarkerV19, 1) != original {
		t.Fatalf("lift changed bytes beyond the marker\nv19:\n%s\nv20:\n%s", original, published)
	}
}

// TestV20ProjectDocumentRoundTripsTheHiddenFlag is the document half: a hidden
// task survives parse and rewrite byte for byte, and the frozen schema-19
// reader refuses the schema-20 marker.
func TestV20ProjectDocumentRoundTripsTheHiddenFlag(t *testing.T) {
	document := V20ProjectDocumentFromV19(v19ProjectTestDocument(t))
	document.Tasks[0].Hidden = true
	published, err := WriteV20ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV20ProjectDocument: %v", err)
	}
	if !strings.Contains(published, "hidden: true -->") {
		t.Fatalf("hidden flag not written:\n%s", published)
	}
	parsed, err := ParseV20ProjectDocument(published)
	if err != nil {
		t.Fatalf("ParseV20ProjectDocument: %v", err)
	}
	if !parsed.Tasks[0].Hidden {
		t.Fatal("hidden flag did not survive the round trip")
	}
	rewritten, err := WriteV20ProjectDocument(parsed)
	if err != nil || rewritten != published {
		t.Fatalf("rewrite changed bytes (%v)\nfirst:\n%s\nsecond:\n%s", err, published, rewritten)
	}
	if _, err := ParseV19ProjectDocument(published); err == nil {
		t.Fatal("the schema-19 reader accepted a schema-20 document")
	}
}

// TestLiveTaskCarriesHiddenThroughTheConversionSeam proves the model's flag
// reaches the live record and comes back, and that an unhidden task writes no
// key at all.
func TestLiveTaskCarriesHiddenThroughTheConversionSeam(t *testing.T) {
	task := mergeTestTask("task-1", "Out of sight")
	task.Hidden = true
	record, err := currentTaskRecordFromModel(task)
	if err != nil {
		t.Fatal(err)
	}
	if !record.Hidden {
		t.Fatal("the record dropped the hidden flag")
	}
	back, err := taskFromCurrentTaskRecord(record, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Hidden {
		t.Fatal("the live task dropped the hidden flag")
	}
	task.Hidden = false
	record, err = currentTaskRecordFromModel(task)
	if err != nil {
		t.Fatal(err)
	}
	body, err := format.WriteV19TaskBody(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "hidden") {
		t.Fatalf("an unhidden task wrote the key:\n%s", body)
	}
}
