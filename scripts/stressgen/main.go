// Command stressgen builds a throwaway Docket workspace of arbitrary size, so the
// Outliner can be exercised and timed under load without touching the workspace
// a person actually uses.
//
// Everything it writes lives beneath one directory:
//
//	<dir>/workspace   the workspace root (.docket/ and projects/)
//	<dir>/home        the machine-local home: runtime state, logs, pointer
//	<dir>/.docket-stress  the marker that lets -force recreate it
//
// Tasks are committed through the engine's own transactions, the path every
// frontend takes, so the result is a workspace the canonical writer accepts
// rather than hand-rolled Markdown. Generation is deterministic for a seed
// except for minted identities and timestamps.
//
//	go run ./scripts/stressgen -dir /tmp/docket-stress -tasks 20000
//	HOME=/tmp/docket-stress/home DOCKET_DATA_DIR=/tmp/docket-stress/workspace ./docket
//	go run ./scripts/stressgen -dir /tmp/docket-stress -measure
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/models"
)

const (
	// markerName identifies a directory this command created, and is the only
	// thing -force trusts before removing one.
	markerName = ".docket-stress"

	// deviceID is the transaction owner the generator commits as.
	deviceID = "docket-stress"
)

type options struct {
	dir       string
	projects  int
	tasks     int
	notes     int
	batch     int
	seed      int64
	childFrac float64
	maxDepth  int
	archived  float64
	parked    float64
	force     bool
	measure   bool
}

func main() {
	var opts options
	flag.StringVar(&opts.dir, "dir", "", "directory to create the stress workspace in (required)")
	flag.IntVar(&opts.projects, "projects", 50, "number of projects")
	flag.IntVar(&opts.tasks, "tasks", 10000, "total number of tasks across all projects")
	flag.IntVar(&opts.notes, "notes", 200, "number of notes, split between projects and global")
	flag.IntVar(&opts.batch, "batch", 500, "tasks committed per transaction")
	flag.Int64Var(&opts.seed, "seed", 1, "random seed for the generated content")
	flag.Float64Var(&opts.childFrac, "children", 0.3, "fraction of tasks nested under another task")
	flag.IntVar(&opts.maxDepth, "depth", 4, "maximum nesting depth (1-6)")
	flag.Float64Var(&opts.archived, "archived", 0.05, "fraction of projects archived")
	flag.Float64Var(&opts.parked, "parked", 0.05, "fraction of projects parked")
	flag.BoolVar(&opts.force, "force", false, "replace an existing stress workspace in -dir")
	flag.BoolVar(&opts.measure, "measure", false, "only time opening an existing stress workspace")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "stressgen:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.dir == "" {
		return errors.New("-dir is required")
	}
	root, err := filepath.Abs(opts.dir)
	if err != nil {
		return err
	}
	if opts.measure {
		return measure(root)
	}
	if err := validate(opts); err != nil {
		return err
	}
	if err := prepareDir(root, opts.force); err != nil {
		return err
	}

	start := time.Now()
	rt, err := open(root, true)
	if err != nil {
		return err
	}
	g := &generator{opts: opts, rt: rt, rng: rand.New(rand.NewSource(opts.seed)), now: time.Now()}
	if err := g.createProjects(); err != nil {
		return err
	}
	if err := g.createTasks(); err != nil {
		return err
	}
	if err := g.createNotes(); err != nil {
		return err
	}
	if err := g.retireProjects(); err != nil {
		return err
	}
	fmt.Printf("generated %d tasks in %d projects and %d notes in %s\n",
		g.taskCount, len(g.projects), opts.notes, time.Since(start).Round(time.Millisecond))

	if err := measure(root); err != nil {
		return err
	}
	fmt.Printf("\nlaunch it, isolated from your own workspace:\n  HOME=%s DOCKET_DATA_DIR=%s docket\n",
		shellQuote(homeDir(root)), shellQuote(workspaceDir(root)))
	return nil
}

func validate(opts options) error {
	switch {
	case opts.projects < 1:
		return errors.New("-projects must be at least 1")
	case opts.tasks < 0 || opts.notes < 0:
		return errors.New("-tasks and -notes may not be negative")
	case opts.batch < 1:
		return errors.New("-batch must be at least 1")
	case opts.childFrac < 0 || opts.childFrac >= 1:
		return errors.New("-children must be in [0, 1)")
	case opts.maxDepth < 1 || opts.maxDepth > 6:
		return errors.New("-depth must be between 1 and 6")
	case opts.archived < 0 || opts.parked < 0 || opts.archived+opts.parked > 1:
		return errors.New("-archived and -parked must be non-negative and sum to at most 1")
	}
	return nil
}

func workspaceDir(root string) string { return filepath.Join(root, "workspace") }
func homeDir(root string) string      { return filepath.Join(root, "home") }

// prepareDir refuses to write into anything but an empty or missing directory,
// or, with force, one this command created before.
func prepareDir(root string, force bool) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return createDir(root)
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return createDir(root)
	}
	if _, err := os.Stat(filepath.Join(root, markerName)); err != nil {
		return fmt.Errorf("%s is not empty and holds no %s marker; refusing to write there", root, markerName)
	}
	if !force {
		return fmt.Errorf("%s already holds a stress workspace; pass -force to replace it", root)
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return createDir(root)
}

func createDir(root string) error {
	for _, dir := range []string{root, workspaceDir(root), homeDir(root)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(root, markerName), []byte("throwaway workspace generated by scripts/stressgen\n"), 0o644)
}

func open(root string, create bool) (*engine.Runtime, error) {
	return engine.Open(engine.Config{
		DataDir:         workspaceDir(root),
		Machine:         engine.ExplicitMachine(homeDir(root), deviceID),
		CreateIfMissing: create,
	})
}

// measure times a cold open of the workspace the way a launch reads it, and
// reports what it holds.
func measure(root string) error {
	if _, err := os.Stat(filepath.Join(root, markerName)); err != nil {
		return fmt.Errorf("%s holds no stress workspace", root)
	}
	start := time.Now()
	rt, err := engine.Open(engine.Config{
		DataDir:         workspaceDir(root),
		Machine:         engine.ExplicitMachine(homeDir(root), deviceID),
		RefuseMigration: true,
	})
	if err != nil {
		return err
	}
	opened := time.Since(start)

	// The first snapshot is what parses every project's tasks.md; the second
	// is what a later read costs once they are loaded.
	start = time.Now()
	snap, err := rt.Snapshot()
	if err != nil {
		return err
	}
	cold := time.Since(start)
	start = time.Now()
	if _, err := rt.Snapshot(); err != nil {
		return err
	}
	warm := time.Since(start)

	tasks := 0
	for _, p := range snap.Projects() {
		tasks += len(p.Tasks)
	}
	size, files := diskUsage(workspaceDir(root))
	taskBytes, _ := diskUsage(filepath.Join(workspaceDir(root), "projects"))
	fmt.Printf("\nworkspace %s\n", workspaceDir(root))
	fmt.Printf("  holds:          %d tasks in %d projects\n", tasks, len(snap.Projects()))
	fmt.Printf("  on disk:        %s in %d files (%s under projects/)\n", humanBytes(size), files, humanBytes(taskBytes))
	fmt.Printf("  open:           %s (recover, load domains)\n", opened.Round(time.Microsecond))
	fmt.Printf("  cold snapshot:  %s (parse every project)\n", cold.Round(time.Microsecond))
	fmt.Printf("  warm snapshot:  %s\n", warm.Round(time.Microsecond))
	return nil
}

func diskUsage(dir string) (int64, int) {
	var size int64
	files := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, infoErr := d.Info(); infoErr == nil {
			size += info.Size()
			files++
		}
		return nil
	})
	return size, files
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func shellQuote(s string) string {
	if !strings.ContainsAny(s, " '\"$`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// generator holds the identities later phases refer back to.
type generator struct {
	opts options
	rt   *engine.Runtime
	rng  *rand.Rand
	now  time.Time

	projects []string
	// open holds nonterminal tasks by depth, the candidates for a parent and
	// for a dependency target.
	open      []app.TaskLocator
	depth     map[string]int
	taskCount int
}

func (g *generator) createProjects() error {
	start := time.Now()
	for i := 0; i < g.opts.projects; i++ {
		name := fmt.Sprintf("%s %s %03d", pick(g.rng, projectAdjectives), pick(g.rng, projectNouns), i+1)
		change, err := g.rt.CreateProject(name)
		if err != nil {
			return fmt.Errorf("create project %q: %w", name, err)
		}
		id := change.Project.ID
		g.projects = append(g.projects, id)
		if g.rng.Float64() < 0.4 {
			edit := engine.ProjectEdit{
				Name:        name,
				Description: sentence(g.rng, 8, 30),
				Tags:        pickN(g.rng, projectTags, 1+g.rng.Intn(2)),
			}
			if _, err := g.rt.EditProject(id, edit); err != nil {
				return fmt.Errorf("describe project %q: %w", name, err)
			}
		}
	}
	fmt.Printf("projects: %d in %s\n", len(g.projects), time.Since(start).Round(time.Millisecond))
	return nil
}

// createTasks fills every project in two passes: roots first, then children
// whose parents an earlier batch already committed, since a capture resolves
// its parent against the snapshot it starts from.
func (g *generator) createTasks() error {
	start := time.Now()
	g.depth = make(map[string]int)
	shares := g.taskShares()
	children := int(float64(g.opts.tasks) * g.opts.childFrac)
	roots := g.opts.tasks - children

	rootsPerProject := make([]int, len(g.projects))
	assigned := 0
	for i := range g.projects {
		rootsPerProject[i] = int(float64(roots) * shares[i])
		assigned += rootsPerProject[i]
	}
	for i := 0; assigned < roots; i++ {
		rootsPerProject[i%len(g.projects)]++
		assigned++
	}

	var pending []app.CaptureRequest
	for i, projectID := range g.projects {
		for n := 0; n < rootsPerProject[i]; n++ {
			pending = append(pending, g.request(projectID, nil))
			if len(pending) == g.opts.batch {
				if err := g.commit(pending, 1); err != nil {
					return err
				}
				pending = pending[:0]
			}
		}
	}
	if err := g.commit(pending, 1); err != nil {
		return err
	}

	// Each child batch draws parents only from tasks already committed, so a
	// later batch can nest under an earlier batch's children.
	for remaining := children; remaining > 0; {
		size := min(remaining, g.opts.batch)
		batch := make([]app.CaptureRequest, 0, size)
		for len(batch) < size {
			parent, ok := g.parentCandidate()
			if !ok {
				break
			}
			parentID := parent.TaskID
			batch = append(batch, g.request(parent.ProjectID, &parentID))
		}
		if len(batch) == 0 {
			break
		}
		if err := g.commit(batch, 0); err != nil {
			return err
		}
		remaining -= len(batch)
	}
	fmt.Printf("\rtasks: %d in %s          \n", g.taskCount, time.Since(start).Round(time.Millisecond))
	return nil
}

// taskShares skews the distribution so a few projects are large, as real
// workspaces are.
func (g *generator) taskShares() []float64 {
	weights := make([]float64, len(g.projects))
	total := 0.0
	for i := range weights {
		weights[i] = 1 + g.rng.ExpFloat64()*4
		total += weights[i]
	}
	for i := range weights {
		weights[i] /= total
	}
	return weights
}

// parentCandidate picks an open task still below the depth cap.
func (g *generator) parentCandidate() (app.TaskLocator, bool) {
	for tries := 0; tries < 64 && len(g.open) > 0; tries++ {
		candidate := g.open[g.rng.Intn(len(g.open))]
		if g.depth[candidate.TaskID] < g.opts.maxDepth {
			return candidate, true
		}
	}
	return app.TaskLocator{}, false
}

// commit captures one batch as one transaction and records what it minted. A
// root batch knows its depth; a child batch derives it from the parent.
func (g *generator) commit(batch []app.CaptureRequest, rootDepth int) error {
	if len(batch) == 0 {
		return nil
	}
	results, err := g.rt.Service().CaptureMany(batch)
	if err != nil {
		return fmt.Errorf("capture %d tasks: %w", len(batch), err)
	}
	for i, result := range results {
		req := batch[i]
		depth := rootDepth
		if req.ParentID != nil {
			depth = g.depth[*req.ParentID] + 1
		}
		g.depth[result.TaskID] = depth
		if !terminalDestination(req.Destination) {
			g.open = append(g.open, app.TaskLocator{ProjectID: result.ProjectID, TaskID: result.TaskID})
		}
	}
	g.taskCount += len(results)
	fmt.Printf("\rtasks: %d/%d", g.taskCount, g.opts.tasks)
	return nil
}

func terminalDestination(d app.CaptureDestination) bool {
	return d == "done" || d == "canceled"
}

// request draws one task's fields. A child stays nonterminal-safe because only
// open tasks are ever chosen as parents.
func (g *generator) request(projectID string, parentID *string) app.CaptureRequest {
	r := g.rng
	req := app.CaptureRequest{
		ProjectID:   projectID,
		Title:       title(r),
		ParentID:    parentID,
		Destination: weighted(r, destinations),
	}
	if r.Float64() < 0.6 {
		req.Priority = 1 + r.Intn(3)
	}
	if r.Float64() < 0.55 {
		req.Tags = pickN(r, taskTags, 1+r.Intn(3))
	}
	if r.Float64() < 0.3 {
		req.Context = pick(r, contexts)
	}
	if r.Float64() < 0.4 {
		req.Description = paragraphs(r)
	}
	if r.Float64() < 0.25 {
		due := g.day(r.Intn(90) - 30)
		req.Due = &due
	}
	switch req.Destination {
	case "backlog":
		if r.Float64() < 0.08 {
			wake := g.now.Add(time.Duration(1+r.Intn(24*21)) * time.Hour)
			req.DeferSet, req.Defer = true, &wake
		}
	case "waiting", "doing":
		if r.Float64() < 0.5 {
			next := g.day(r.Intn(14) - 4)
			req.CheckIn = models.CheckInSchedule{Next: &next, RepeatDays: []int{0, 0, 3, 7}[r.Intn(4)]}
		}
	}
	if !terminalDestination(req.Destination) {
		req.Pinned = r.Float64() < 0.005
		req.Review = r.Float64() < 0.02
		if r.Float64() < 0.05 {
			req.ResumeNote = sentence(r, 4, 14)
		}
		if r.Float64() < 0.03 && len(g.open) > 0 {
			req.Dependencies = []app.TaskLocator{g.open[r.Intn(len(g.open))]}
		}
	}
	if r.Float64() < 0.06 {
		for n := 1 + r.Intn(3); n > 0; n-- {
			req.Comments = append(req.Comments, sentence(r, 5, 25))
		}
	}
	return req
}

// day is local midnight offset days from today: a calendar date.
func (g *generator) day(offset int) time.Time {
	y, m, d := g.now.AddDate(0, 0, offset).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func (g *generator) createNotes() error {
	if g.opts.notes == 0 {
		return nil
	}
	start := time.Now()
	ops := make([]app.NoteMutationRequest, 0, g.opts.batch)
	flush := func() error {
		if len(ops) == 0 {
			return nil
		}
		if _, err := g.rt.Service().ApplyNoteBatch(app.NoteBatchRequest{Operations: ops, UndoAction: "Create Notes"}); err != nil {
			return fmt.Errorf("create %d notes: %w", len(ops), err)
		}
		ops = ops[:0]
		return nil
	}
	for i := 0; i < g.opts.notes; i++ {
		location := app.NoteLocation{Scope: app.NoteScopeGlobal}
		if g.rng.Float64() < 0.8 {
			location = app.ProjectNoteLocation(g.projects[g.rng.Intn(len(g.projects))])
		}
		note := models.NewNote(fmt.Sprintf("%s %d", title(g.rng), i+1))
		note.Body = noteBody(g.rng)
		ops = append(ops, app.NoteMutationRequest{Kind: app.NoteCreate, Location: location, Note: note})
		if len(ops) == g.opts.batch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	fmt.Printf("notes: %d in %s\n", g.opts.notes, time.Since(start).Round(time.Millisecond))
	return nil
}

// retireProjects parks and archives a share of the projects last, so their
// tasks were filed like any other project's.
func (g *generator) retireProjects() error {
	archive := int(float64(len(g.projects)) * g.opts.archived)
	park := int(float64(len(g.projects)) * g.opts.parked)
	order := g.rng.Perm(len(g.projects))
	for _, i := range order[:archive] {
		if _, err := g.rt.ArchiveProject(g.projects[i]); err != nil {
			return fmt.Errorf("archive project: %w", err)
		}
	}
	for _, i := range order[archive : archive+park] {
		if _, err := g.rt.SetProjectTags(g.projects[i], []string{"parked"}); err != nil {
			return fmt.Errorf("park project: %w", err)
		}
	}
	if archive+park > 0 {
		fmt.Printf("projects: %d archived, %d parked\n", archive, park)
	}
	return nil
}

type weightedDestination struct {
	value  app.CaptureDestination
	weight int
}

var destinations = []weightedDestination{
	{"inbox", 8}, {"backlog", 34}, {"doing", 4}, {"waiting", 4}, {"paused", 3},
	{"someday", 10}, {"done", 30}, {"canceled", 5}, {"today", 1}, {"next", 1},
}

func weighted(r *rand.Rand, options []weightedDestination) app.CaptureDestination {
	total := 0
	for _, o := range options {
		total += o.weight
	}
	n := r.Intn(total)
	for _, o := range options {
		if n < o.weight {
			return o.value
		}
		n -= o.weight
	}
	return options[0].value
}

func pick(r *rand.Rand, words []string) string { return words[r.Intn(len(words))] }

func pickN(r *rand.Rand, words []string, n int) []string {
	out := make([]string, 0, n)
	for _, i := range r.Perm(len(words))[:min(n, len(words))] {
		out = append(out, words[i])
	}
	return out
}

func title(r *rand.Rand) string {
	return fmt.Sprintf("%s %s %s", pick(r, verbs), pick(r, adjectives), pick(r, nouns))
}

func sentence(r *rand.Rand, minWords, maxWords int) string {
	n := minWords + r.Intn(maxWords-minWords+1)
	words := make([]string, n)
	for i := range words {
		words[i] = pick(r, lorem)
	}
	s := strings.Join(words, " ")
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

func paragraphs(r *rand.Rand) string {
	n := 1 + r.Intn(3)
	parts := make([]string, n)
	for i := range parts {
		parts[i] = sentence(r, 10, 60)
	}
	return strings.Join(parts, "\n\n")
}

func noteBody(r *rand.Rand) string {
	var b strings.Builder
	for n := 1 + r.Intn(5); n > 0; n-- {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", title(r), paragraphs(r))
	}
	return b.String()
}

var (
	projectAdjectives = []string{"Core", "Quiet", "Rapid", "Legacy", "Shared", "Mobile", "Cloud", "Internal", "Public", "Studio"}
	projectNouns      = []string{"Platform", "Garden", "Ledger", "Pipeline", "Atlas", "Harbor", "Beacon", "Forge", "Archive", "Compass"}
	projectTags       = []string{"work", "home", "client", "research", "ops", "q3", "q4", "long-term"}
	taskTags          = []string{
		"bug", "question", "epic", "story", "feature", "idea", "infra", "docs", "ui", "api", "perf",
		"security", "design", "release", "meeting", "followup", "blocked-ext", "quick", "deep", "errand",
		"finance", "health", "reading", "writing", "review-me", "backend", "frontend", "mobile", "data", "test",
	}
	contexts   = []string{"office", "home", "phone", "computer", "errands", "laptop", "deep-work", "calls", "online", "anywhere"}
	verbs      = []string{"Fix", "Write", "Review", "Plan", "Refactor", "Call", "Draft", "Ship", "Investigate", "Update", "Migrate", "Clean", "Book", "Measure", "Document"}
	adjectives = []string{"the flaky", "quarterly", "new", "old", "shared", "urgent", "pending", "remaining", "broken", "draft", "release", "customer", "weekly", "nested", "cached"}
	nouns      = []string{"handoff notes", "search index", "billing report", "onboarding flow", "release checklist", "budget", "dentist", "API client", "sync engine", "dashboard", "test suite", "roadmap", "invoice", "backup", "migration"}
	lorem      = strings.Fields("lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua release handoff review deadline sync outline project task context budget follow up schedule")
)
