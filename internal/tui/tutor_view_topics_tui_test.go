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
		for _, want := range []string{"Ctrl+G", "Home", "Planner", "Task Workspace", "Review", "Manage"} {
			if !strings.Contains(topic.Content, want) {
				t.Errorf("%s missing %q", title, want)
			}
		}
	}
}

func TestTutorDocumentsPaletteAndTaskComposer(t *testing.T) {
	palette := findTutorTUITopic(t, "Command Palette")
	for _, want := range []string{"Press :", "search", "triage inbox", "undo last triage action"} {
		if !strings.Contains(palette.Content, want) {
			t.Errorf("Command Palette topic missing %q", want)
		}
	}
	composer := findTutorTUITopic(t, "Quick Capture Grammar")
	for _, want := range []string{"Ctrl+N", "Task Composer", "@date", "#tag"} {
		if !strings.Contains(composer.Content, want) {
			t.Errorf("Quick Capture Grammar topic missing %q", want)
		}
	}
}

func TestTutorContainsNoRetiredNavigationSurfaces(t *testing.T) {
	retired := []string{"Pipeline Hub", "Keybinding Explorer", "Global Search", ";;", ";i", ";q", ";w", `ctrl+\`}
	for _, topic := range tutorTopics() {
		for _, term := range retired {
			if strings.Contains(topic.Content, term) {
				t.Errorf("topic %q contains retired navigation %q", topic.Title, term)
			}
		}
	}
}
