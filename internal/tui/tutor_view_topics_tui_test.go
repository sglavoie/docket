package tui

import (
	"strings"
	"testing"
)

func TestTutorBulkOperationsTopicDocumentsSelectedTaskFieldActions(t *testing.T) {
	topic := findTutorTUITopic(t, "Bulk Operations")

	required := []string{
		"selected tasks instead of only the cursor task",
		"!1-!5  Set priority",
		"Set, clear, or pick due dates",
		"Set, clear, or pick snooze dates",
		"Set or clear stage",
		"Move to section",
		"Replace tags",
		"cross-project views",
		"cross-project section moves are not supported",
		"Kanban multi-select applies supported task-field mutations to the",
		"selected cards",
		"Bulk delete is not supported in Kanban",
		"cursor task",
		"Kanban views",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Bulk Operations topic missing %q:\n%s", want, topic.Content)
		}
	}

	stale := "Multi-select is available in the outliner, project tree, and\nall-tasks views."
	if strings.Contains(topic.Content, stale) {
		t.Fatalf("Bulk Operations topic still contains stale view scope text")
	}
}

func findTutorTUITopic(t *testing.T, title string) TutorTopic {
	t.Helper()

	for _, topic := range tutorTUITopics() {
		if topic.Title == title {
			return topic
		}
	}
	t.Fatalf("tutor TUI topic %q not found", title)
	return TutorTopic{}
}
