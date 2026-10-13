package taskform

import (
	"strings"
	"testing"
	"time"

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

func TestCaptureCompletionsRewriteTheTokenAtTheCaret(t *testing.T) {
	home := &models.Project{ID: "p1", Slug: "home", Name: "Home", Tasks: []*models.Task{
		{ID: "t1", ShortID: 1, Title: "Existing", Tags: []string{"urgent", "urgently"}},
	}}
	work := &models.Project{ID: "p2", Slug: "client-work", Name: "Client Work", Tasks: []*models.Task{
		{ID: "t2", ShortID: 1, Title: "Invoice", Tags: []string{"billing"}},
	}}
	preset := TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            home,
		AllProjects:        []*models.Project{home, work},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}

	set := CaptureCompletions(preset, nil, "Call bank %cli")
	if set.Token != "%cli" || set.Exact {
		t.Fatalf("set = %+v", set)
	}
	if len(set.Items) == 0 || set.Items[0] != (CaptureCompletion{Label: "%Client Work", Insert: "%client-work "}) {
		t.Fatalf("items = %+v", set.Items)
	}

	// Only the last line is the one being typed.
	if set := CaptureCompletions(preset, nil, "first %cli\nsecond !"); set.Token != "!" || len(set.Items) == 0 {
		t.Fatalf("multiline set = %+v", set)
	}
	// A tag the grammar accepts as typed is complete, so Enter would submit it.
	if set := CaptureCompletions(preset, nil, "Call bank #urgent"); set.Token != "#urgent" || !set.Exact || len(set.Items) != 2 {
		t.Fatalf("a literal tag was not exact: %+v", set)
	}
	// A global capture starts in one project but offers every project's tags.
	if set := CaptureCompletions(preset, nil, "Call bank #bil"); set.Token != "#bil" ||
		len(set.Items) != 1 || set.Items[0] != (CaptureCompletion{Label: "#billing", Insert: "#billing "}) {
		t.Fatalf("cross-project tag set = %+v", set)
	}
	if set := CaptureCompletions(preset, nil, "Call bank"); set.Token != "" || set.Items != nil {
		t.Fatalf("prose completed to %+v", set)
	}
}

func TestDateCompletionExplainsTheDateWithoutInsertingTheLabel(t *testing.T) {
	project := &models.Project{ID: "p1", Slug: "home", Name: "Home"}
	preset := TaskComposerPreset{Kind: ComposerProject, Project: project}
	set := CaptureCompletions(preset, nil, "Ship @fri")
	if len(set.Items) == 0 {
		t.Fatal("missing Friday completion")
	}
	for _, item := range set.Items {
		if !strings.HasPrefix(item.Label, strings.TrimSpace(item.Insert)+" · Fri ") {
			t.Fatalf("date suggestion does not explain Friday: %+v", item)
		}
		reqs, errs := ParseCaptureText(preset, nil, "Ship "+item.Insert)
		if len(errs) != 0 || len(reqs) != 1 || reqs[0].Title != "Ship" || reqs[0].Due == nil {
			t.Fatalf("completion changed capture semantics: %+v, %+v", reqs, errs)
		}
		if reqs[0].Due.Weekday() != time.Friday || !strings.Contains(item.Label, reqs[0].Due.Format("Mon Jan 2")) {
			t.Fatalf("suggestion and saved date disagree: %+v, %v", item, reqs[0].Due)
		}
	}
}
