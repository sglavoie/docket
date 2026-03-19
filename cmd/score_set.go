package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var scoreSetCmd = &cobra.Command{
	Use:   "set <task-ref> <dim=value>...",
	Short: "Set manual scoring dimensions on a task",
	Long: `Set one or more scoring dimension values on a task.

Each dimension is specified as key=value where value is an integer.
The dimension key must match a dimension ID in the project's active scoring profile.

Examples:
  tt score set 42 impact=4 confidence=3
  tt score set abc-123 reach=8 effort=2`,
	RunE: runScoreSet,
}

func init() {
	scoreCmd.AddCommand(scoreSetCmd)
}

func runScoreSet(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and score")
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	// Validate profile assignment
	profile := cfg.GetProjectScoringProfile(project.ID)
	if profile == nil {
		return fmt.Errorf("no scoring profile assigned to project %q. Assign one with: tt score assign <profile-id>", project.Name)
	}

	// Parse dimension assignments
	if task.ScoreDimensions == nil {
		task.ScoreDimensions = make(map[string]int)
	}
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid dimension assignment %q: expected key=value", arg)
		}
		dimID := parts[0]
		val, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid value for %q: %w", dimID, err)
		}

		dim := profile.GetDimension(dimID)
		if dim == nil {
			return fmt.Errorf("dimension %q not found in profile %q", dimID, profile.ID)
		}

		if val < dim.Min || val > dim.Max {
			return fmt.Errorf("value %d for %q outside range [%d, %d]", val, dimID, dim.Min, dim.Max)
		}

		task.ScoreDimensions[dimID] = val
	}

	task.UpdatedAt = time.Now()
	if err := SaveProject(project, nil); err != nil {
		return err
	}
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Updated score dimensions for task #%d %q\n", task.ShortID, task.Title)
	return nil
}
