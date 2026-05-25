package tui

import (
	"reflect"
	"sort"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestSearchSortDeterminism verifies that score-based sort tiebreaks by ID/filename
// produce a canonical order regardless of input permutation.
func TestSearchSortDeterminism(t *testing.T) {
	t.Run("task_jumper", func(t *testing.T) {
		type scoredTask struct {
			task  *models.Task
			score int
		}
		mkScored := func(id string, score int) scoredTask {
			return scoredTask{task: &models.Task{ID: id}, score: score}
		}

		// Three tasks share the same score.
		input1 := []scoredTask{mkScored("aaaa", 10), mkScored("bbbb", 10), mkScored("cccc", 10)}
		input2 := []scoredTask{mkScored("cccc", 10), mkScored("aaaa", 10), mkScored("bbbb", 10)}

		less := func(s []scoredTask) func(i, j int) bool {
			return func(i, j int) bool {
				if s[i].score != s[j].score {
					return s[i].score > s[j].score
				}
				return s[i].task.ID < s[j].task.ID
			}
		}
		sort.Slice(input1, less(input1))
		sort.Slice(input2, less(input2))

		ids1 := make([]string, len(input1))
		ids2 := make([]string, len(input2))
		for i, s := range input1 {
			ids1[i] = s.task.ID
		}
		for i, s := range input2 {
			ids2[i] = s.task.ID
		}
		if !reflect.DeepEqual(ids1, ids2) {
			t.Errorf("task jumper sort non-deterministic:\n run1=%v\n run2=%v", ids1, ids2)
		}
	})

	t.Run("notes_search", func(t *testing.T) {
		mkResult := func(filename string, score int) notesSearchResult {
			return notesSearchResult{note: &models.Note{Filename: filename}, score: score}
		}
		input1 := []notesSearchResult{
			mkResult("2026-01-03-c.md", 5),
			mkResult("2026-01-01-a.md", 5),
			mkResult("2026-01-02-b.md", 5),
		}
		input2 := []notesSearchResult{
			mkResult("2026-01-02-b.md", 5),
			mkResult("2026-01-03-c.md", 5),
			mkResult("2026-01-01-a.md", 5),
		}
		less := func(s []notesSearchResult) func(i, j int) bool {
			return func(i, j int) bool {
				if s[i].score != s[j].score {
					return s[i].score > s[j].score
				}
				return s[i].note.Filename < s[j].note.Filename
			}
		}
		sort.Slice(input1, less(input1))
		sort.Slice(input2, less(input2))

		names1 := make([]string, len(input1))
		names2 := make([]string, len(input2))
		for i, r := range input1 {
			names1[i] = r.note.Filename
		}
		for i, r := range input2 {
			names2[i] = r.note.Filename
		}
		if !reflect.DeepEqual(names1, names2) {
			t.Errorf("notes search sort non-deterministic:\n run1=%v\n run2=%v", names1, names2)
		}
	})

	t.Run("global_search_tasks_vs_notes", func(t *testing.T) {
		// Equal-score task and note: task should come first.
		type scoredResult struct {
			result models.SearchResult
			score  int
		}
		task := scoredResult{result: models.SearchResult{Task: &models.Task{ID: "t1"}}, score: 10}
		note := scoredResult{result: models.SearchResult{Note: &models.Note{Filename: "note.md"}}, score: 10}

		s1 := []scoredResult{note, task}
		s2 := []scoredResult{task, note}

		less := func(s []scoredResult) func(i, j int) bool {
			return func(i, j int) bool {
				if s[i].score != s[j].score {
					return s[i].score > s[j].score
				}
				ri, rj := s[i].result, s[j].result
				if ri.Task != nil && rj.Task != nil {
					return ri.Task.ID < rj.Task.ID
				}
				if ri.Note != nil && rj.Note != nil {
					return ri.Note.Filename < rj.Note.Filename
				}
				return ri.Task != nil
			}
		}
		sort.Slice(s1, less(s1))
		sort.Slice(s2, less(s2))

		if s1[0].result.Task == nil || s2[0].result.Task == nil {
			t.Errorf("expected task before note in global search; got task-first=%v/%v",
				s1[0].result.Task != nil, s2[0].result.Task != nil)
		}
		if s1[0].result.Task.ID != s2[0].result.Task.ID {
			t.Errorf("global search sort non-deterministic for same score")
		}
	})
}
