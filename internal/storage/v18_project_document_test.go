package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func TestV18ChecklistOriginRoundTripAndStrictness(t *testing.T) {
	project := currentSchemaFixtureProject()
	content, err := WriteProjectMarkdown(project)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatal(err)
	}
	want := &models.ChecklistOrigin{ProjectID: "project-alpha", TaskID: "task-parent", Index: 3}
	if got := parsed.Tasks[1].ChecklistOrigin; got == nil || *got != *want {
		t.Fatalf("origin = %+v", got)
	}
	for _, malformed := range []string{
		strings.Replace(content, `"index":3`, `"index":-1`, 1),
		strings.Replace(content, `"index":3`, `"index":3,"unknown":1`, 1),
		strings.Replace(content, `"index":3`, `"index":3, "extra":1`, 1),
	} {
		if _, err := ParseProjectDocument(malformed); err == nil {
			t.Fatal("malformed origin accepted")
		}
	}
}
