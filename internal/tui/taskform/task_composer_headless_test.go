package taskform

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestParseCaptureTextReadsLinesAsTheComposerDoes(t *testing.T) {
	project := &models.Project{ID: "p1", Slug: "home", Name: "Home"}
	preset := TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            project,
		AllProjects:        []*models.Project{project},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}

	reqs, errs := ParseCaptureText(preset, nil, "Buy milk #errand\n\nCall bank !1")
	if len(errs) != 0 {
		t.Fatalf("errors = %+v", errs)
	}
	if len(reqs) != 2 || reqs[0].Title != "Buy milk" || reqs[1].Priority != 1 || reqs[1].SourceLine != 3 {
		t.Fatalf("requests = %+v", reqs)
	}
	if reqs[0].ProjectID != "p1" || reqs[0].Destination != app.CaptureDestinationInbox {
		t.Fatalf("defaults not applied: %+v", reqs[0])
	}

	if _, errs := ParseCaptureText(preset, nil, "   "); len(errs) != 1 || errs[0].Message != "no tasks to create" {
		t.Fatalf("blank text errors = %+v", errs)
	}
}
