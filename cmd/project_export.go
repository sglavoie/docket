package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var projectExportFile string

var projectExportCmd = &cobra.Command{
	Use:   "export [slug-or-id]",
	Short: "Export a project to clean Markdown",
	Long: `Export a project to a human-readable Markdown file in the exports/ folder of your tt data directory.

If no project is specified, exports the current project.
Use --file to write to an arbitrary path instead.

EXAMPLES

  tt project export
  tt project export my-project
  tt project export my-project --file /tmp/my-project.md`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectExport,
}

func init() {
	projectCmd.AddCommand(projectExportCmd)
	projectExportCmd.Flags().StringVar(&projectExportFile, "file", "", "write to this path instead of the default exports/<slug>.md")
}

func runProjectExport(cmd *cobra.Command, args []string) error {
	project, workspace, err := LoadRuntimeProjectOrCurrent(args)
	if err != nil {
		return err
	}

	dest := projectExportFile
	if dest == "" {
		dest = workspace.Manager.ExportPath(project.Slug)
		if err = os.MkdirAll(workspace.Manager.ExportsDir(), 0700); err != nil {
			return fmt.Errorf("create exports dir: %w", err)
		}
	}
	if guardErr := config.RefusePreSimplificationV5SnapshotMutation(workspace.Runtime.DataDir(), dest); guardErr != nil {
		return fmt.Errorf("refusing project export destination: %w", guardErr)
	}

	// Archived projects join the label population deliberately. An export is a
	// document rather than a live view, so naming a reference that points into
	// archived work is strictly better than emitting its bare locator; nothing
	// here loads an archived project for any other purpose.
	related := append(workspace.Runtime.AvailableProjects(), workspace.Runtime.ArchivedProjects()...)
	destPath, err := ExportProjectMarkdown(project, dest, related...)
	if err != nil {
		return err
	}

	fmt.Printf("Exported %q to %s\n", project.Name, destPath)
	return nil
}

// ExportProjectMarkdown writes the project to destPath (which must be non-empty;
// callers resolve default export locations through config.Manager.ExportPath)
// and returns the final absolute path.
// LoadProject is called on p to ensure tasks are populated before exporting.
// related supplies display labels for outgoing task references that leave p.
func ExportProjectMarkdown(p *models.Project, destPath string, related ...*models.Project) (string, error) {
	return storage.ExportProjectMarkdown(p, destPath, related...)
}
