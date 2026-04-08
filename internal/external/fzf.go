package external

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

const defaultFzfTimeoutSeconds = 30

// fzfTimeoutSeconds is the timeout for fzf invocations. Configurable via SetFzfTimeout.
var fzfTimeoutSeconds = defaultFzfTimeoutSeconds

// SetFzfTimeout sets the fzf invocation timeout in seconds.
// Values <= 0 are ignored and the default (30s) is kept.
func SetFzfTimeout(seconds int) {
	if seconds > 0 {
		fzfTimeoutSeconds = seconds
	}
}

// FzfAvailable checks if fzf is installed and available.
func FzfAvailable() bool {
	_, err := CachedFzfPath()
	return err == nil
}

// FzfSelectProject uses fzf to select a project from the list.
func FzfSelectProject(projects []*models.Project, prompt string) (*models.Project, error) {
	if _, err := CachedFzfPath(); err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to select from")
	}

	// Build input for fzf.
	// Format: slug<TAB>display-name (slug)<TAB>context<TAB>N tasks
	// The slug column is hidden via --with-nth=2.. and used only for parsing.
	var lines []string
	projectMap := make(map[string]*models.Project)

	for _, p := range projects {
		line := fmt.Sprintf("%s\t%s (%s)\t%s\t%d tasks", p.Slug, p.Name, p.Slug, p.Context, p.CountTasks())
		lines = append(lines, line)
		projectMap[p.Slug] = p
	}

	input := strings.Join(lines, "\n")

	// Run fzf
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(fzfTimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fzf",
		"--prompt", prompt+" ",
		"--height", "40%",
		"--reverse",
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--preview-window", "hidden",
	)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 {
				return nil, tterrors.ErrCanceled
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("fzf timed out after %ds — try again or press Ctrl+C to cancel", fzfTimeoutSeconds)
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	}

	selected := strings.TrimSpace(stdout.String())
	if selected == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse the selection - slug is the first tab-delimited field.
	parts := strings.SplitN(selected, "\t", 2)
	if len(parts) > 0 {
		if project, ok := projectMap[parts[0]]; ok {
			return project, nil
		}
	}

	return nil, fmt.Errorf("project not found")
}

// FzfSelectTask uses fzf to select a task from the list.
func FzfSelectTask(tasks []*models.Task, prompt string, previewCmd string) (*models.Task, error) {
	if _, err := CachedFzfPath(); err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to select from")
	}

	// Build input for fzf
	var lines []string
	taskMap := make(map[int]*models.Task)

	for _, t := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			t.ShortID,
			t.Type.Icon(),
			t.Status.Icon(),
			t.Title)
		lines = append(lines, line)
		taskMap[t.ShortID] = t
	}

	input := strings.Join(lines, "\n")

	// Build fzf command with optional preview
	args := []string{
		"--prompt", prompt + " ",
		"--height", "50%",
		"--reverse",
		"--delimiter", "\t",
		"--with-nth", "1,2,3,4",
	}

	if previewCmd != "" {
		args = append(args,
			"--preview", previewCmd+" {1}",
			"--preview-window", "right:50%:wrap",
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(fzfTimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fzf", args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 {
				return nil, tterrors.ErrCanceled
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("fzf timed out after %ds — try again or press Ctrl+C to cancel", fzfTimeoutSeconds)
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	}

	selected := strings.TrimSpace(stdout.String())
	if selected == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse the selection - get task ID from first field
	parts := strings.Split(selected, "\t")
	if len(parts) > 0 {
		// Extract short ID from "#N" format
		idStr := strings.TrimPrefix(parts[0], "#")
		var shortID int
		if _, err := fmt.Sscanf(idStr, "%d", &shortID); err == nil {
			if task, ok := taskMap[shortID]; ok {
				return task, nil
			}
		}
	}

	return nil, fmt.Errorf("task not found")
}

// FzfMultiSelectProjects uses fzf to select multiple projects from the list.
func FzfMultiSelectProjects(projects []*models.Project, prompt string, preselectedIDs map[string]bool) ([]*models.Project, error) {
	if _, err := CachedFzfPath(); err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to select from")
	}

	// Build input for fzf.
	// Format: slug<TAB>display-name (slug)<TAB>context<TAB>N tasks
	// The slug column is hidden via --with-nth=2.. and used only for parsing.
	var lines []string
	projectMap := make(map[string]*models.Project)

	for _, p := range projects {
		marker := " "
		if preselectedIDs[p.ID] {
			marker = "*"
		}
		line := fmt.Sprintf("%s\t%s %s (%s)\t%s\t%d tasks", p.Slug, marker, p.Name, p.Slug, p.Context, p.CountTasks())
		lines = append(lines, line)
		projectMap[p.Slug] = p
	}

	input := strings.Join(lines, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(fzfTimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fzf",
		"--prompt", prompt+" ",
		"--height", "50%",
		"--reverse",
		"--multi",
		"--delimiter", "\t",
		"--with-nth", "2..",
	)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 {
				return nil, tterrors.ErrCanceled
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("fzf timed out after %ds — try again or press Ctrl+C to cancel", fzfTimeoutSeconds)
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse selections - slug is the first tab-delimited field.
	var selectedProjects []*models.Project
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) > 0 {
			if project, ok := projectMap[parts[0]]; ok {
				selectedProjects = append(selectedProjects, project)
			}
		}
	}

	if len(selectedProjects) == 0 {
		return nil, fmt.Errorf("no projects selected")
	}

	return selectedProjects, nil
}

// FzfMultiSelectTasks uses fzf to select multiple tasks from the list.
func FzfMultiSelectTasks(tasks []*models.Task, prompt string) ([]*models.Task, error) {
	if _, err := CachedFzfPath(); err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to select from")
	}

	// Build input for fzf
	var lines []string
	taskMap := make(map[int]*models.Task)

	for _, t := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			t.ShortID,
			t.Type.Icon(),
			t.Status.Icon(),
			t.Title)
		lines = append(lines, line)
		taskMap[t.ShortID] = t
	}

	input := strings.Join(lines, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(fzfTimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fzf",
		"--prompt", prompt+" ",
		"--height", "50%",
		"--reverse",
		"--multi",
		"--delimiter", "\t",
		"--with-nth", "1,2,3,4",
	)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 {
				return nil, tterrors.ErrCanceled
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("fzf timed out after %ds — try again or press Ctrl+C to cancel", fzfTimeoutSeconds)
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse selections
	var selectedTasks []*models.Task
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) > 0 {
			idStr := strings.TrimPrefix(parts[0], "#")
			var shortID int
			if _, err := fmt.Sscanf(idStr, "%d", &shortID); err == nil {
				if task, ok := taskMap[shortID]; ok {
					selectedTasks = append(selectedTasks, task)
				}
			}
		}
	}

	if len(selectedTasks) == 0 {
		return nil, fmt.Errorf("no tasks selected")
	}

	return selectedTasks, nil
}
