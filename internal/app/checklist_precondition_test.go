package app_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestChecklistMutationsRefuseStaleLists(t *testing.T) {
	actions := map[string]func(*app.Service, app.TaskLocator, []models.ChecklistItem) error{
		"edit": func(s *app.Service, target app.TaskLocator, frozen []models.ChecklistItem) error {
			_, err := s.EditChecklistItem(target, 1, "replacement", frozen)
			return err
		},
		"remove": func(s *app.Service, target app.TaskLocator, frozen []models.ChecklistItem) error {
			_, err := s.RemoveChecklistItem(target, 1, frozen)
			return err
		},
		"toggle": func(s *app.Service, target app.TaskLocator, frozen []models.ChecklistItem) error {
			_, err := s.ToggleChecklistItem(target, 1, frozen)
			return err
		},
		"swap": func(s *app.Service, target app.TaskLocator, frozen []models.ChecklistItem) error {
			_, err := s.ReorderChecklistItem(target, 1, 0, frozen)
			return err
		},
		"move": func(s *app.Service, target app.TaskLocator, frozen []models.ChecklistItem) error {
			_, err := s.MoveChecklistItem(target, 1, 0, frozen)
			return err
		},
	}
	changes := map[string]func(*models.Task){
		"shifted":          func(task *models.Task) { task.Checklist = task.Checklist[1:] },
		"edited":           func(task *models.Task) { task.Checklist[1].Text = "external edit" },
		"ticked duplicate": func(task *models.Task) { task.Checklist[0].Done = true },
		"reordered duplicates": func(task *models.Task) {
			task.Checklist[1], task.Checklist[2] = task.Checklist[2], task.Checklist[1]
		},
		"removed": func(task *models.Task) { task.Checklist = nil },
	}
	for action, mutate := range actions {
		for change, perturb := range changes {
			t.Run(action+"/"+change, func(t *testing.T) {
				task := newTask("t1", 1, "Checklist owner")
				task.Checklist = []models.ChecklistItem{{Text: "duplicate"}, {Text: "duplicate"}, {Text: "third", Done: true}}
				frozen := slices.Clone(task.Checklist)
				perturb(task)
				repo := newFakeRepo(newProject("p1", "Alpha", task))
				before := models.CloneTaskDeep(task)
				if err := mutate(newTestService(repo), repo.loc("t1"), frozen); !errors.Is(err, app.ErrInvalidInput) {
					t.Fatalf("err = %v, want stale-list refusal", err)
				}
				if len(repo.commits) != 0 || !reflect.DeepEqual(before, models.CloneTaskDeep(task)) {
					t.Fatal("refused mutation changed content, clocks, or transaction history")
				}
			})
		}
	}
}
