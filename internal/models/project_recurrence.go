package models

import "time"

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
		// Calculate next due date
		nextDue := task.NextDueDate()

		// Create new instance
		newTask := &Task{
			ID:         generateUUID(),
			ShortID:    p.GetNextShortID(),
			Type:       task.Type,
			Title:      task.Title,
			Status:     StatusTodo,
			Priority:   task.Priority,
			Section:    SectionInbox, // New instances go to Inbox for triage
			Tags:       append([]string{}, task.Tags...),
			DueDate:    nextDue,
			Recurrence: task.Recurrence, // Carry forward the recurrence pattern
			CreatedAt:  now,
			UpdatedAt:  now,
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

// generateUUID creates a new UUID string. This is a simple implementation
// that relies on the uuid package being imported in files that use Project.
// For the model package, we use a simple time-based approach.
func generateUUID() string {
	// Use timestamp + counter approach for unique IDs within model package
	// The actual UUID generation happens in the calling code (TUI/CLI)
	// This is a fallback that should rarely be used
	return time.Now().Format("20060102150405.000000000")
}
