package cmd

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/scoring"
)

var scoreSetCmd = &cobra.Command{
	Use:   "set <task-ref> <dim=value>...",
	Short: "Set manual scoring dimensions on a task",
	Args:  cobra.MinimumNArgs(2),
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
	addOutputFlags(scoreSetCmd)
}

func runScoreSet(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("requires at least 2 args: task-ref and at least one dim=value")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, cfg, _, err := LoadCurrentProject()
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

	// Parse and validate ALL dimension assignments first; only mutate the task
	// once we know every assignment is valid. Otherwise an error on the Nth
	// argument would leave the first N-1 already written into the in-memory
	// map.
	pending := make(map[string]int, len(args)-1)
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

		if err := scoring.ValidateScoreDimension(dim, val); err != nil {
			return err
		}

		pending[dimID] = val
	}

	// The mutation persists through the application layer (atomic,
	// revision-checked, undoable via the workspace ledger); the profile
	// validation above stays here because scoring profiles live in config,
	// which the app layer does not read.
	svc := newAppService(cfg)
	if _, err := withTaskRetry(func() (app.TaskResult, error) {
		return svc.SetScoreDimensions(task.ID, pending)
	}); err != nil {
		return translateAppError(err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		// Merge the applied values onto the loaded copy so the JSON reflects the
		// committed dimension set.
		dims := make(map[string]int, len(task.ScoreDimensions)+len(pending))
		maps.Copy(dims, task.ScoreDimensions)
		maps.Copy(dims, pending)
		return printJSON(struct {
			TaskID     string         `json:"task_id"`
			ShortID    int            `json:"short_id"`
			Title      string         `json:"title"`
			Dimensions map[string]int `json:"dimensions"`
		}{TaskID: task.ID, ShortID: task.ShortID, Title: task.Title, Dimensions: dims})
	}
	fmt.Printf("Updated score dimensions for task #%d %q\n", task.ShortID, task.Title)
	return nil
}
