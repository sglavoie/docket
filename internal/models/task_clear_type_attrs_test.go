package models

import "testing"

func TestTask_ClearTypeAttrs(t *testing.T) {
	task := &Task{
		BugAttrs:   &BugAttrs{Severity: BugSeverityMajor},
		IdeaAttrs:  &IdeaAttrs{Category: IdeaCategoryFeature},
		TaskAttrs:  &TaskAttrs{Effort: EffortSmall},
		StoryAttrs: &StoryAttrs{StoryPoints: 5},
		EpicAttrs:  &EpicAttrs{Scope: EpicScopeDefined},
	}

	task.ClearTypeAttrs()

	if task.BugAttrs != nil {
		t.Error("BugAttrs should be nil after ClearTypeAttrs()")
	}
	if task.IdeaAttrs != nil {
		t.Error("IdeaAttrs should be nil after ClearTypeAttrs()")
	}
	if task.TaskAttrs != nil {
		t.Error("TaskAttrs should be nil after ClearTypeAttrs()")
	}
	if task.StoryAttrs != nil {
		t.Error("StoryAttrs should be nil after ClearTypeAttrs()")
	}
	if task.EpicAttrs != nil {
		t.Error("EpicAttrs should be nil after ClearTypeAttrs()")
	}
}