package external

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// fallbackSelectOne prints a numbered list and reads a selection from stdin.
// Returns the 0-based index of the selected item.
func fallbackSelectOne(items []string, prompt string) (int, error) {
	fmt.Println(prompt)
	for i, item := range items {
		fmt.Printf("  %d) %s\n", i+1, item)
	}
	fmt.Print("Enter number: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("failed to read input: %w", err)
	}

	line = strings.TrimSpace(line)
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(items) {
		return 0, fmt.Errorf("invalid selection: %s", line)
	}
	return n - 1, nil
}

// FallbackSelectProject presents a numbered list of projects for selection.
func FallbackSelectProject(projects []*models.Project, prompt string) (*models.Project, error) {
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to select from")
	}

	items := make([]string, len(projects))
	for i, p := range projects {
		items[i] = fmt.Sprintf("%s (%s, %d tasks)", p.Name, p.Context, p.CountTasks())
	}

	idx, err := fallbackSelectOne(items, prompt)
	if err != nil {
		return nil, err
	}
	return projects[idx], nil
}

// FallbackSelectTask presents a numbered list of tasks for selection.
func FallbackSelectTask(tasks []*models.Task, prompt string) (*models.Task, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to select from")
	}

	items := make([]string, len(tasks))
	for i, t := range tasks {
		items[i] = fmt.Sprintf("#%d %s %s %s", t.ShortID, t.Type.Icon(), t.Status.Icon(), t.Title)
	}

	idx, err := fallbackSelectOne(items, prompt)
	if err != nil {
		return nil, err
	}
	return tasks[idx], nil
}

// FallbackMultiSelectTasks presents a numbered list and reads comma-separated selections.
func FallbackMultiSelectTasks(tasks []*models.Task, prompt string) ([]*models.Task, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to select from")
	}

	fmt.Println(prompt)
	for i, t := range tasks {
		fmt.Printf("  %d) #%d %s %s %s\n", i+1, t.ShortID, t.Type.Icon(), t.Status.Icon(), t.Title)
	}
	fmt.Print("Enter numbers (comma-separated): ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	var selected []*models.Task
	for part := range strings.SplitSeq(strings.TrimSpace(line), ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(tasks) {
			return nil, fmt.Errorf("invalid selection: %s", part)
		}
		selected = append(selected, tasks[n-1])
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no tasks selected")
	}
	return selected, nil
}
