package compat

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// coverageProbes answers each coverage key a manifest may declare. A probe
// inspects only what the live readers produced, so a key holds only if the
// feature survived parsing, not merely if its bytes are present.
var coverageProbes = map[string]func(*fixtureWorld) bool{
	"relationship.parent_child": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.ParentID != nil && task.Parent != nil })
	},
	"relationship.nested_depth_3": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.Level >= 2 })
	},
	"relationship.dependency_cross_project": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			for _, reference := range task.Dependencies {
				if reference.ProjectID != w.owner[task.ID].ID {
					return true
				}
			}
			return false
		})
	},
	"relationship.dependency_reciprocal": func(w *fixtureWorld) bool {
		edges := map[[2]string]bool{}
		for _, task := range w.tasks {
			for _, reference := range task.Dependencies {
				edges[[2]string{w.owner[task.ID].ID, reference.ProjectID}] = true
			}
		}
		for edge := range edges {
			if edge[0] != edge[1] && edges[[2]string{edge[1], edge[0]}] {
				return true
			}
		}
		return false
	},
	"relationship.checklist_origin": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.ChecklistOrigin != nil && task.ParentID != nil && task.ChecklistOrigin.TaskID == *task.ParentID
		})
	},
	"planning.today": func(w *fixtureWorld) bool { return len(w.runtime.Planning().GetToday()) > 0 },
	"planning.next":  func(w *fixtureWorld) bool { return len(w.runtime.NextOrder()) > 0 },
	"planning.parked_next_hidden": func(w *fixtureWorld) bool {
		return len(w.runtime.Planning().GetNext()) > len(w.runtime.NextOrder())
	},
	"checkin.repeating": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.NextCheckIn != nil && task.CheckInRepeatDays > 0 })
	},
	"checkin.one_off": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.NextCheckIn != nil && task.CheckInRepeatDays == 0 })
	},
	"checkin.waiting": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.WorkflowState == models.StateWaiting && task.NextCheckIn != nil
		})
	},
	"checkin.reviewed_terminal": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.WorkflowState.IsTerminal() && task.LastReviewedAt != nil && task.NextCheckIn == nil
		})
	},
	"lifecycle.all_states": func(w *fixtureWorld) bool {
		seen := map[models.WorkflowState]bool{}
		for _, task := range w.tasks {
			seen[task.WorkflowState] = true
		}
		for _, state := range []models.WorkflowState{
			models.StateInbox, models.StateBacklog, models.StateDoing, models.StateWaiting,
			models.StatePaused, models.StateSomeday, models.StateDone, models.StateCanceled,
		} {
			if !seen[state] {
				return false
			}
		}
		return true
	},
	"schedule.due_date": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.DueDate != nil })
	},
	"schedule.due_time": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.DueDate != nil && (task.DueDate.Hour() != 0 || task.DueDate.Minute() != 0)
		})
	},
	"schedule.deferred": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.DeferredUntil != nil && task.DeferCount > 0 })
	},
	"rich.description": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return strings.Contains(task.Description, "\n") })
	},
	"rich.resume_note": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.ResumeNote != "" })
	},
	"rich.task_comments": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return len(task.Comments) > 1 })
	},
	"rich.project_comments": func(w *fixtureWorld) bool {
		for _, project := range w.projects {
			for _, comment := range project.Comments {
				if strings.Contains(comment.Text, "\n") {
					return true
				}
			}
		}
		return false
	},
	"rich.checklist": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			done, open := false, false
			for _, item := range task.Checklist {
				done = done || item.Done
				open = open || !item.Done
			}
			return done && open
		})
	},
	"rich.attachment_file": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			for _, attachment := range task.Attachments {
				if !attachment.IsURL && !attachment.IsCodeRef &&
					storage.AttachmentExists(w.owner[task.ID], task, attachment.Filename) &&
					storage.GetAttachmentSize(w.owner[task.ID], task, &attachment) > 0 {
					return true
				}
			}
			return false
		})
	},
	"rich.attachment_url": func(w *fixtureWorld) bool {
		return w.anyAttachment(func(attachment models.Attachment) bool { return attachment.IsURL })
	},
	"rich.attachment_code_ref": func(w *fixtureWorld) bool {
		return w.anyAttachment(func(attachment models.Attachment) bool {
			return attachment.IsCodeRef && attachment.LineNumber > 0
		})
	},
	"rich.project_note": func(w *fixtureWorld) bool {
		for _, notes := range w.notes {
			for _, note := range notes {
				if note.ParseError == "" && note.Body != "" {
					return true
				}
			}
		}
		return false
	},
	"rich.global_note": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.globalNotes, func(note *models.Note) bool { return note.ParseError == "" && note.Body != "" })
	},
	"rich.tags_and_context": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return len(task.Tags) > 1 && task.Context != "" })
	},
	"rich.pinned": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.Pinned && task.PinnedAt != nil })
	},
	"rich.review_flag": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.Review })
	},
	"lens.scoped": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.lenses, func(lens config.NamedLens) bool {
			return lens.Scope != "" && w.projectByID(lens.Scope) != nil && lens.Query != ""
		})
	},
	"lens.unscoped": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.lenses, func(lens config.NamedLens) bool { return lens.Scope == "" })
	},
	"project.archived": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.projects, func(project *models.Project) bool {
			return project.Archived && project.ArchivedAt != nil && len(project.Tasks) > 0
		})
	},
	"project.archived_check_in": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return w.owner[task.ID].Archived && task.NextCheckIn != nil })
	},
	"project.parked": func(w *fixtureWorld) bool {
		for _, project := range w.projects {
			if !models.ProjectTagsParked(project.Tags) {
				continue
			}
			hidden := !slices.ContainsFunc(w.runtime.TaskWorkspaceProjects(), func(p *models.Project) bool { return p.ID == project.ID })
			available := slices.ContainsFunc(w.runtime.AvailableProjects(), func(p *models.Project) bool { return p.ID == project.ID })
			if hidden && available {
				return true
			}
		}
		return false
	},
	"project.system_unfiled": func(w *fixtureWorld) bool {
		unfiled := w.runtime.SystemUnfiledProject()
		return unfiled != nil && w.projectByID(unfiled.ID) != nil && len(w.projectByID(unfiled.ID).Tasks) > 0
	},
	"project.tags": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.projects, func(project *models.Project) bool {
			return len(project.Tags) > 1 && !models.ProjectTagsParked(project.Tags)
		})
	},
}

func (w *fixtureWorld) anyTask(predicate func(*models.Task) bool) bool {
	for _, task := range w.tasks {
		if predicate(task) {
			return true
		}
	}
	return false
}

func (w *fixtureWorld) anyAttachment(predicate func(models.Attachment) bool) bool {
	return w.anyTask(func(task *models.Task) bool { return slices.ContainsFunc(task.Attachments, predicate) })
}

func (w *fixtureWorld) projectByID(id string) *models.Project {
	for _, project := range w.projects {
		if project.ID == id {
			return project
		}
	}
	return nil
}

// TestCompatFixtureCoverageSurvivesLiveParsing runs the probe for every key each
// manifest declares, one subtest per key so a lost feature is named.
func TestCompatFixtureCoverageSurvivesLiveParsing(t *testing.T) {
	for _, manifest := range loadFixtureManifests(t) {
		world := openFixture(t, manifest.Workspace)
		for _, key := range manifest.Coverage {
			t.Run(manifest.Name+"/"+key, func(t *testing.T) {
				probe, ok := coverageProbes[key]
				if !ok {
					t.Fatalf("no probe for %q", key)
				}
				if !probe(world) {
					t.Fatalf("%s declares %q but the live readers do not exhibit it", manifest.Name, key)
				}
			})
		}
	}
}
