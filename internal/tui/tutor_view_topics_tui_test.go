package tui

import (
	"strings"
	"testing"
)

func findTutorTUITopic(t *testing.T, title string) TutorTopic {
	t.Helper()
	for _, topic := range tutorTUITopics() {
		if topic.Title == title {
			return topic
		}
	}
	t.Fatalf("TUI tutor topic %q not found", title)
	return TutorTopic{}
}

func findTutorTopicByTitle(t *testing.T, title string) TutorTopic {
	t.Helper()
	for _, topic := range tutorTopics() {
		if topic.Title == title {
			return topic
		}
	}
	t.Fatalf("tutor topic %q not found", title)
	return TutorTopic{}
}

func TestTutorDocumentsConsolidatedNavigation(t *testing.T) {
	for _, title := range []string{"The pipeline", "The Task Pipeline", "Pipeline Views"} {
		topic := findTutorTopicByTitle(t, title)
		for _, want := range []string{"Outliner", "Today", "Tab", "Ctrl+Esc"} {
			if !strings.Contains(topic.Content, want) {
				t.Errorf("%s missing %q", title, want)
			}
		}
	}
}

func TestTutorDocumentsFinderAndTaskComposer(t *testing.T) {
	finder := findTutorTUITopic(t, "Finder")
	for _, want := range []string{"Press :", "search", "OperationRegistry"} {
		if !strings.Contains(finder.Content, want) {
			t.Errorf("Finder topic missing %q", want)
		}
	}
	composer := findTutorTUITopic(t, "Task Composer Grammar")
	for _, want := range []string{"Press a", "Task Composer", "@date", "#tag"} {
		if !strings.Contains(composer.Content, want) {
			t.Errorf("Task Composer Grammar topic missing %q", want)
		}
	}
}

func TestTutorDocumentsFinalDailyLoopContract(t *testing.T) {
	keys := findTutorTUITopic(t, "Everyday Keys")
	for _, want := range []string{
		"a       Capture",
		"e       Edit",
		"s       Start",
		"d       Complete",
		"b       Move",
		"t       Toggle",
		"n       Toggle",
		"Space   Open",
	} {
		if !strings.Contains(keys.Content, want) {
			t.Errorf("Everyday Keys topic missing %q", want)
		}
	}

	pipeline := findTutorTUITopic(t, "Pipeline Views")
	for _, want := range []string{
		"Tab cycles All, Inbox, Next, and Backlog",
		"Press c",
		"N opens Notes",
		"R opens Weekly Review",
		"T opens Tags",
		"! opens Notifications",
	} {
		if !strings.Contains(pipeline.Content, want) {
			t.Errorf("Pipeline Views topic missing %q", want)
		}
	}

	actions := findTutorTUITopic(t, "Task Actions and Multi-Select")
	for _, want := range []string{"Press Space", "Press v", "Space to select", "Press Enter", "Destructive actions require confirmation"} {
		if !strings.Contains(actions.Content, want) {
			t.Errorf("Task Actions and Multi-Select topic missing %q", want)
		}
	}
}

func TestTutorDocumentsCanonicalStateGrammar(t *testing.T) {
	state := findTutorTopicByTitle(t, "Workflow States")
	for _, want := range []string{"inbox", "ready", "next", "in_progress", "waiting", "paused", "review", "someday", "done", "canceled"} {
		if !strings.Contains(state.Content, want) {
			t.Errorf("Workflow States topic missing %q", want)
		}
	}

	composer := findTutorTUITopic(t, "Task Composer Grammar")
	for _, want := range []string{"$state", "state:ready", "omission defaults to Inbox"} {
		if !strings.Contains(composer.Content, want) {
			t.Errorf("Task Composer Grammar topic missing %q", want)
		}
	}
}

func TestTutorContainsNoRetiredNavigationSurfaces(t *testing.T) {
	retired := []string{
		"Command Palette",
		"command palette",
		"Inbox Processor",
		"inbox processor",
		"Pipeline Hub",
		"Keybinding Explorer",
		"Global Search",
		"Ctrl+N",
		"Ctrl+X",
		"smart focus",
		"perspective",
		"prefix-chord",
		";;",
		";i",
		";q",
		";w",
		`ctrl+\`,
		"Sections (GTD)",
		"GTD section",
		"Today section",
		"workflow sections",
		"/sections",
		"--section",
		"--status",
		"status transitions",
		"status changes to",
		"Press C in the Outliner",
		"Press C to",
		"Press q",
	}
	for _, topic := range tutorTopics() {
		for _, term := range retired {
			if strings.Contains(topic.Content, term) {
				t.Errorf("topic %q contains retired navigation %q", topic.Title, term)
			}
		}
	}
}
