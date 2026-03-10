package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
)

var templateUseCmd = &cobra.Command{
	Use:   "use <template-id> [title]",
	Short: "Create a task from a template",
	Long: `Create a new task using a template.

The template defines the task type, priority, tags, section, and checklist items.
If a title is provided, it will be used; otherwise, the template name is used.

Checklist items are created automatically based on the template definition.

Templates may contain {{variable}} placeholders. These are prompted interactively
or can be supplied via --var flags. Use {{name:default}} for optional variables
with a default value.

FLAGS
  --skip-optional  Don't create optional children
  --parent         Parent task ID for the new task
  --var            Set a template variable (name=value), repeatable

EXAMPLES

  # Create a bug report
  tt template use def-bug-report "Login fails on mobile"

  # Create with default title (template name)
  tt template use def-release-checklist

  # Skip optional children
  tt template use def-bug-report "Crash on startup" --skip-optional

  # Create under a parent task
  tt template use def-api-endpoint "GET /users" --parent 42

  # Supply template variables via flags
  tt template use onboarding --var client=Acme --var env=staging

  # Mix flags and interactive prompting (unprovided vars are prompted)
  tt template use onboarding --var client=Acme`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTemplateUse,
}

var (
	templateUseSkipOptional bool
	templateUseParent       string
	templateUseVars         []string
)

func init() {
	templateCmd.AddCommand(templateUseCmd)

	templateUseCmd.Flags().BoolVar(&templateUseSkipOptional, "skip-optional", false, "don't create optional children")
	templateUseCmd.Flags().StringVarP(&templateUseParent, "parent", "p", "", "parent task ID or short ID")
	templateUseCmd.Flags().StringArrayVar(&templateUseVars, "var", nil, "set template variable (name=value), repeatable")
}

func runTemplateUse(cmd *cobra.Command, args []string) error {
	templateID := args[0]
	title := ""
	if len(args) > 1 {
		title = strings.Join(args[1:], " ")
	}

	// Load config
	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Find template by ID or name
	t := cfg.GetTaskTemplate(templateID)
	if t == nil {
		// Try by name
		t = cfg.GetTaskTemplateByName(templateID)
	}
	if t == nil {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	// Resolve template variables if present
	vars := template.ExtractVariables(t)
	if len(vars) > 0 {
		values, resolveErr := resolveTemplateVars(vars, templateUseVars)
		if resolveErr != nil {
			return resolveErr
		}
		resolved := template.ApplyVariables(*t, values)
		t = &resolved
	}

	// Use template name as title if not provided
	if title == "" {
		title = t.Name
	}

	// Load current project
	project, _, projectConfigManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Create main task
	taskID := uuid.New().String()
	shortID := project.GetNextShortID()
	task, err := models.NewTask(taskID, shortID, t.Type, title)
	if err != nil {
		return err
	}

	// Apply template settings
	task.Priority = t.GetEffectivePriority()
	if t.Section != "" {
		project.EnsureSection(t.Section)
		task.Section = t.Section
	}
	for _, tag := range t.Tags {
		task.AddTag(tag)
	}
	if t.Description != "" {
		task.Description = t.Description
	}

	// Handle parent
	if templateUseParent != "" {
		parent, err := ValidateParentTask(project, templateUseParent, task.Type)
		if err != nil {
			return err
		}
		task.ParentID = &parent.ID
	}

	// Add task to project
	project.Tasks = append(project.Tasks, task)

	// Create checklist from template
	checklistCreated := 0
	for _, item := range t.Checklist {
		task.Checklist = append(task.Checklist, models.ChecklistItem{
			Text: item,
			Done: false,
		})
		checklistCreated++
	}

	// Create subtasks from Children (for Epic/Story templates)
	subtasksCreated := 0
	if t.Type.CanHaveChildren() && len(t.Children) > 0 {
		for _, child := range t.Children {
			if child.Optional && templateUseSkipOptional {
				continue
			}
			childID := uuid.New().String()
			childShortID := project.GetNextShortID()
			childType := child.GetChildType()
			childTask, childErr := models.NewTask(childID, childShortID, childType, child.Title)
			if childErr != nil {
				return fmt.Errorf("invalid child task %q: %w", child.Title, childErr)
			}
			childTask.ParentID = &task.ID
			childTask.Priority = task.Priority
			childTask.Section = task.Section
			for _, tag := range task.Tags {
				childTask.AddTag(tag)
			}
			project.Tasks = append(project.Tasks, childTask)
			subtasksCreated++
		}
	}

	// Rebuild tree and save
	project.BuildTaskTree()
	project.ReorderRootTasksByCompletion()

	// Increment use count
	cfg.IncrementTemplateUseCount(t.ID)
	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	// Update project timestamp
	project.UpdatedAt = time.Now()

	// Save project
	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: projectConfigManager}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	fmt.Printf("Created %s #%d: %s (from template %q)\n", task.Type.HeadingPrefix(), shortID, title, t.Name)
	if checklistCreated > 0 {
		fmt.Printf("  Added %d checklist item(s)\n", checklistCreated)
	}
	if subtasksCreated > 0 {
		fmt.Printf("  Created %d subtask(s)\n", subtasksCreated)
	}

	return nil
}

// parseVarFlags parses --var flag values ("name=value") into a map.
func parseVarFlags(flags []string) (map[string]string, error) {
	values := make(map[string]string, len(flags))
	for _, f := range flags {
		idx := strings.Index(f, "=")
		if idx < 1 {
			return nil, fmt.Errorf("invalid --var format %q, expected name=value", f)
		}
		values[f[:idx]] = f[idx+1:]
	}
	return values, nil
}

// resolveTemplateVars builds a complete variable values map by combining --var
// flags with interactive prompts for any unsatisfied variables. When stdin is
// not a terminal, unsatisfied variables use their defaults silently.
func resolveTemplateVars(vars []template.TemplateVariable, flags []string) (map[string]string, error) {
	values, err := parseVarFlags(flags)
	if err != nil {
		return nil, err
	}

	interactive := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	reader := bufio.NewReader(os.Stdin)

	for _, v := range vars {
		if _, ok := values[v.Name]; ok {
			continue
		}
		if !interactive {
			values[v.Name] = v.Default
			continue
		}
		prompt := fmt.Sprintf("Enter %s", v.Name)
		if v.Default != "" {
			prompt += fmt.Sprintf(" [%s]", v.Default)
		}
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			line = v.Default
		}
		values[v.Name] = line
	}

	return values, nil
}
