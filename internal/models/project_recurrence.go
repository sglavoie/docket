package models

import (
	"crypto/rand"
	"fmt"
	"maps"
	"sync/atomic"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// GenerateRecurringInstances checks for completed recurring tasks and creates
// their next instances. Returns the number of instances generated.
// New instances are placed in the Inbox section with status todo.
func (p *Project) GenerateRecurringInstances() int {
	generated := 0
	now := time.Now()

	// Collect tasks that need new instances (don't modify slice while iterating)
	var toGenerate []*Task
	for _, task := range p.Tasks {
		if task.Recurrence != nil && task.Status == StatusDone {
			// Only generate if not already processed (check by looking for existing todo instance)
			toGenerate = append(toGenerate, task)
		}
	}

	for _, task := range toGenerate {
		// Update streak based on completion timeliness
		if task.Recurrence != nil {
			onTime := true
			if task.DueDate != nil && task.CompletedAt != nil {
				completedDay := util.StartOfDay(*task.CompletedAt)
				dueDay := util.StartOfDay(*task.DueDate)
				onTime = !completedDay.After(dueDay)
			}
			// No DueDate counts as on-time
			if onTime {
				task.Recurrence.Streak++
				if task.Recurrence.Streak > task.Recurrence.MaxStreak {
					task.Recurrence.MaxStreak = task.Recurrence.Streak
				}
			} else {
				task.Recurrence.Streak = 0
			}
		}

		// Calculate next due date
		nextDue := task.NextDueDate()

		// Create new instance — carry forward recurrence pattern with streak data
		newRecurrence := &RecurrencePattern{
			Interval:  task.Recurrence.Interval,
			Unit:      task.Recurrence.Unit,
			Weekdays:  append([]int{}, task.Recurrence.Weekdays...),
			Skipped:   task.Recurrence.Skipped,
			Streak:    task.Recurrence.Streak,
			MaxStreak: task.Recurrence.MaxStreak,
		}

		// Copy score dimensions from the completed task
		scoreDims := make(map[string]int, len(task.ScoreDimensions))
		maps.Copy(scoreDims, task.ScoreDimensions)

		// Copy checklist items (reset all to unchecked for the new instance)
		var checklist []ChecklistItem
		if len(task.Checklist) > 0 {
			checklist = make([]ChecklistItem, len(task.Checklist))
			for i, item := range task.Checklist {
				checklist[i] = ChecklistItem{Text: item.Text, Done: false}
			}
		}

		newTask := &Task{
			ID:              generateUUID(),
			ShortID:         p.GetNextShortID(),
			Type:            task.Type,
			Title:           task.Title,
			Description:     task.Description, // Carry forward description for context
			Status:          StatusTodo,
			Priority:        task.Priority,
			Section:         SectionInbox, // New instances go to Inbox for triage
			Tags:            append([]string{}, task.Tags...),
			DependsOn:       []string{},
			RelatedTo:       []string{},
			ScoreDimensions: scoreDims,
			Children:        []*Task{},
			DueDate:         nextDue,
			Recurrence:      newRecurrence,
			Checklist:       checklist,
			Stage:           task.Stage,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		// Clear recurrence from completed task (pattern moves to new instance)
		task.Recurrence = nil
		task.UpdatedAt = now

		// Add new task at the beginning of the list
		p.Tasks = append([]*Task{newTask}, p.Tasks...)
		generated++
	}

	// Ensure Inbox section exists if we generated anything
	if generated > 0 {
		p.EnsureSection(SectionInbox)
	}

	return generated
}

// uuidCounter is an atomic counter to ensure uniqueness when generateUUID
// is called multiple times within the same nanosecond (e.g., in a loop).
var uuidCounter atomic.Uint64

// generateUUID creates a new unique ID string using timestamp, random bytes,
// and an atomic counter. This avoids collisions when called rapidly in a loop
// (e.g., generating multiple recurring task instances).
func generateUUID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	seq := uuidCounter.Add(1)
	return fmt.Sprintf("%s-%x-%d", time.Now().Format("20060102150405.000000000"), b, seq)
}
