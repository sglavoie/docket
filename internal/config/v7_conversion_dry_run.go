package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/sglavoie/docket/internal/models"
)

const (
	// V7ConfigSchemaVersion is the project-tag catalog payload version. Schema 7
	// keeps every schema-6 project field except the retired per-project scope
	// pair, which becomes an ordinary project tag set.
	V7ConfigSchemaVersion = 7

	// v7ActiveScopeReference is the migration-only copy of the reserved scope
	// reference that meant "not parked". Schema 7 has no scope table at all, so
	// the value cannot be read from live code and is restated here.
	v7ActiveScopeReference = "active"

	// v7ParkedAliasIdentity separates a retired scope name that literally reads
	// "parked" on a project that was *not* parked from the reserved behavioral
	// tag. Sanitization maps every control character away, so no real name can
	// collide with this key.
	v7ParkedAliasIdentity = "\x00parked-alias"
)

// v7SourceProject is the schema-6 project shape exactly as it rests on disk,
// including the two retired scope fields. It is deliberately not the frozen
// v5-to-v6 output DTO: that one never learned to emit either field.
type v7SourceProject struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Context     string     `json:"context"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	ColorSlot   int        `json:"color_slot,omitempty"`
	BoundaryID  string     `json:"boundary_id"`
}

// v7SourceScope is one retired availability grouping definition.
type v7SourceScope struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// v7SourceCatalog decodes a complete schema-6 workspace catalog. A catalog
// written by the v5-to-v6 migration carries neither scope field, while one
// saved by a later release carries both, so every retired field is optional.
type v7SourceCatalog struct {
	SchemaVersion          int               `json:"schema_version"`
	WorkspaceID            string            `json:"workspace_id,omitempty"`
	Projects               []v7SourceProject `json:"projects"`
	SystemUnfiledProjectID string            `json:"system_unfiled_project_id"`
	Boundaries             []v7SourceScope   `json:"boundaries,omitempty"`
}

// V7CatalogMigrationAuditEntry records one deterministic decision taken while
// folding the retired scope pair into project tags.
type V7CatalogMigrationAuditEntry struct {
	ProjectID string `json:"project_id,omitempty"`
	Action    string `json:"action"`
	Field     string `json:"field"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Reason    string `json:"reason"`
}

// V7WorkspaceDryRunOptions identifies the schema-6 workspace whose catalog is
// converted. The migration reads and rewrites only the workspace catalog.
type V7WorkspaceDryRunOptions struct {
	DataDir string
}

// V7WorkspaceArtifactCandidate carries the exact schema-7 catalog bytes for the
// single in-place rewrite the migration publishes.
type V7WorkspaceArtifactCandidate struct {
	Path string
	Data []byte
}

// V7WorkspaceConversionPlan is a filesystem-free publication candidate after
// construction: the source catalog has been read, validated, and converted, and
// the target bytes are fixed. Building the plan never writes live data.
type V7WorkspaceConversionPlan struct {
	Catalog  WorkspaceCatalog
	Audit    []V7CatalogMigrationAuditEntry
	Artifact V7WorkspaceArtifactCandidate
}

// DryRunV7WorkspaceConversion reads a complete schema-6 workspace catalog and
// builds a deterministic, fully validated schema-7 candidate without publishing
// any write or cleanup operation.
func DryRunV7WorkspaceConversion(options V7WorkspaceDryRunOptions) (V7WorkspaceConversionPlan, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return V7WorkspaceConversionPlan{}, err
	}
	path := WorkspaceCatalogPath(root)
	source, err := readV7SourceCatalog(path)
	if err != nil {
		return V7WorkspaceConversionPlan{}, err
	}
	catalog, audit, err := convertV7CatalogCandidate(source)
	if err != nil {
		return V7WorkspaceConversionPlan{}, err
	}
	data, err := encodeDomainEnvelope(domainCatalog, catalog.WorkspaceID, &catalog)
	if err != nil {
		return V7WorkspaceConversionPlan{}, fmt.Errorf("encode schema-7 workspace catalog: %w", err)
	}
	plan := V7WorkspaceConversionPlan{
		Catalog:  catalog,
		Audit:    audit,
		Artifact: V7WorkspaceArtifactCandidate{Path: path, Data: data},
	}
	if err := ValidateV7WorkspaceConversionPlan(plan); err != nil {
		return V7WorkspaceConversionPlan{}, err
	}
	return plan, nil
}

// ValidateV7WorkspaceConversionPlan re-checks a candidate independently of how
// it was built. The round trip through the live decoder is the load guarantee:
// a frozen output DTO that emits a retired field would publish bytes the strict
// runtime decoder refuses, and this rejects that before any snapshot exists.
func ValidateV7WorkspaceConversionPlan(plan V7WorkspaceConversionPlan) error {
	if plan.Catalog.SchemaVersion != V7ConfigSchemaVersion {
		return fmt.Errorf("v7 catalog schema is %d, want %d", plan.Catalog.SchemaVersion, V7ConfigSchemaVersion)
	}
	if err := plan.Catalog.Validate(); err != nil {
		return fmt.Errorf("v7 catalog candidate: %w", err)
	}
	if filepath.Base(plan.Artifact.Path) != filepath.Base(WorkspaceCatalogPath(".")) {
		return fmt.Errorf("v7 migration artifact %s is not the workspace catalog", plan.Artifact.Path)
	}
	published, err := decodeV7PublishedCatalog(plan.Artifact.Data)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(published, plan.Catalog) {
		return fmt.Errorf("v7 catalog artifact does not decode back into the validated candidate")
	}
	return nil
}

func readV7SourceCatalog(path string) (v7SourceCatalog, error) {
	data, err := readV5DryRunSource(path)
	if err != nil {
		return v7SourceCatalog{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return v7SourceCatalog{}, fmt.Errorf("decode v6 workspace envelope %s: %w", path, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return v7SourceCatalog{}, fmt.Errorf("decode v6 workspace envelope %s: %w", path, err)
	}
	if envelope.SchemaVersion != CurrentDomainSchemaVersion || envelope.Kind != domainCatalog.name() {
		return v7SourceCatalog{}, fmt.Errorf("v6 workspace envelope %s has invalid schema or kind", path)
	}
	if err := validateWorkspaceID(envelope.WorkspaceID); err != nil {
		return v7SourceCatalog{}, fmt.Errorf("v6 workspace envelope %s: %w", path, err)
	}
	payload := json.NewDecoder(bytes.NewReader(envelope.Data))
	payload.DisallowUnknownFields()
	var catalog v7SourceCatalog
	if err := payload.Decode(&catalog); err != nil {
		return v7SourceCatalog{}, fmt.Errorf("decode v6 workspace payload %s: %w", path, err)
	}
	if err := requireJSONEOF(payload); err != nil {
		return v7SourceCatalog{}, fmt.Errorf("decode v6 workspace payload %s: %w", path, err)
	}
	if catalog.SchemaVersion != V6ConfigSchemaVersion {
		return v7SourceCatalog{}, fmt.Errorf(
			"v7 migration source catalog schema is %d, want %d", catalog.SchemaVersion, V6ConfigSchemaVersion,
		)
	}
	if catalog.WorkspaceID != envelope.WorkspaceID {
		return v7SourceCatalog{}, fmt.Errorf(
			"v6 workspace payload identity %q does not match envelope %q", catalog.WorkspaceID, envelope.WorkspaceID,
		)
	}
	return catalog, nil
}

func decodeV7PublishedCatalog(data []byte) (WorkspaceCatalog, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return WorkspaceCatalog{}, fmt.Errorf("decode published v7 workspace envelope: %w", err)
	}
	payload := json.NewDecoder(bytes.NewReader(envelope.Data))
	payload.DisallowUnknownFields()
	var catalog WorkspaceCatalog
	if err := payload.Decode(&catalog); err != nil {
		return WorkspaceCatalog{}, fmt.Errorf("decode published v7 workspace payload: %w", err)
	}
	return catalog, nil
}

//nolint:gocyclo // The seven audited scope edge cases belong to one readable pass.
func convertV7CatalogCandidate(
	source v7SourceCatalog,
) (WorkspaceCatalog, []V7CatalogMigrationAuditEntry, error) {
	converter := &v7CatalogConverter{namer: newV7TagNamer()}
	scopeNames, err := converter.indexScopes(source.Boundaries)
	if err != nil {
		return WorkspaceCatalog{}, nil, err
	}

	target := WorkspaceCatalog{
		SchemaVersion:          V7ConfigSchemaVersion,
		WorkspaceID:            source.WorkspaceID,
		Projects:               make([]CatalogProject, 0, len(source.Projects)),
		SystemUnfiledProjectID: source.SystemUnfiledProjectID,
	}
	for _, project := range source.Projects {
		scope := strings.TrimSpace(project.BoundaryID)
		if scope == "" {
			scope = v7ActiveScopeReference
		}
		parked := scope != v7ActiveScopeReference
		if parked && project.ID == source.SystemUnfiledProjectID {
			converter.record(project.ID, "dropped", "scope_reference", scope, "", ""+
				"the protected Unfiled project can never be parked, so its retired scope reference is discarded")
			parked = false
		}

		var tags []string
		if tag, ok := converter.tagFromName(project.ID, "context", project.Context, parked); ok {
			tags = append(tags, tag)
		}
		if parked {
			name, known := scopeNames[scope]
			if !known {
				return WorkspaceCatalog{}, nil, fmt.Errorf(
					"project %q references undefined retired scope %q", project.ID, scope,
				)
			}
			if tag, ok := converter.tagFromName(project.ID, "scope_name", name, parked); ok {
				tags = append(tags, tag)
			}
			tags = append(tags, models.ParkedProjectTag)
		}

		normalized, err := models.NormalizeProjectTags(tags)
		if err != nil {
			return WorkspaceCatalog{}, nil, fmt.Errorf("project %q produced invalid tags: %w", project.ID, err)
		}
		if len(normalized) < len(tags) {
			converter.record(project.ID, "deduped", "tags", strings.Join(tags, " "), strings.Join(normalized, " "),
				"the retired context and scope name folded to the same tag")
		}
		if parked && project.Archived {
			converter.record(project.ID, "retained", "tags", "", models.ParkedProjectTag,
				"an archived parked project keeps the tag so restoring it returns it parked")
		}
		target.Projects = append(target.Projects, CatalogProject{
			ID:          project.ID,
			Slug:        project.Slug,
			Name:        project.Name,
			Description: project.Description,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
			Archived:    project.Archived,
			ArchivedAt:  cloneTimePointer(project.ArchivedAt),
			Pinned:      project.Pinned,
			PinnedAt:    cloneTimePointer(project.PinnedAt),
			ColorSlot:   project.ColorSlot,
			Tags:        normalized,
		})
	}
	if err := target.Validate(); err != nil {
		return WorkspaceCatalog{}, nil, fmt.Errorf("schema-7 catalog candidate: %w", err)
	}
	return target, converter.audit, nil
}

type v7CatalogConverter struct {
	namer *v7TagNamer
	audit []V7CatalogMigrationAuditEntry
}

func (c *v7CatalogConverter) record(projectID, action, field, from, to, reason string) {
	c.audit = append(c.audit, V7CatalogMigrationAuditEntry{
		ProjectID: projectID, Action: action, Field: field, From: from, To: to, Reason: reason,
	})
}

// indexScopes drops the whole retired scope table and returns the display names
// the parked projects still need. Every dropped definition is audited, so the
// archive explains where a name went.
func (c *v7CatalogConverter) indexScopes(scopes []v7SourceScope) (map[string]string, error) {
	names := make(map[string]string, len(scopes))
	for _, scope := range scopes {
		id := strings.TrimSpace(scope.ID)
		if id == "" {
			return nil, fmt.Errorf("retired scope definition %q has an empty reference", scope.Name)
		}
		if _, duplicate := names[id]; duplicate {
			return nil, fmt.Errorf("retired scope reference %q is defined twice", id)
		}
		names[id] = scope.Name
		reason := "schema 7 has no scope table; membership became the reserved parked tag"
		if id == v7ActiveScopeReference {
			reason = "the reserved unparked scope has no schema-7 meaning and carries no tag"
		}
		c.record("", "dropped", "scope_definition", scope.Name, "", reason)
	}
	return names, nil
}

// tagFromName converts one retired scope name into a canonical project tag,
// reporting false when the name contributes nothing. The parked flag decides
// the only ambiguous case: a name reading "parked" merges into the reserved tag
// for a project that was parked and must be renamed for one that was not.
func (c *v7CatalogConverter) tagFromName(projectID, field, raw string, parked bool) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		c.record(projectID, "dropped", field, raw, "",
			"an empty retired name contributes no tag and the empty tag set is legal")
		return "", false
	}
	identity := sanitizeV7ScopeName(trimmed, 0)
	if identity == "" {
		c.record(projectID, "dropped", field, raw, "",
			"the retired name held no characters a tag can carry")
		return "", false
	}
	folded := strings.ToLower(trimmed)
	alias := !parked && folded == models.ParkedProjectTag
	tag := c.namer.assign(identity, alias)
	switch {
	case alias:
		c.record(projectID, "renamed", field, raw, tag,
			"a retired name reading parked would have silently parked an unparked project")
	case parked && folded == models.ParkedProjectTag:
		c.record(projectID, "merged", field, raw, tag,
			"the retired name already matches the reserved parked tag this parked project receives")
	case tag != folded:
		c.record(projectID, "sanitized", field, raw, tag,
			"the retired name was not usable as a tag and was rewritten deterministically")
	case folded == "personal":
		c.record(projectID, "retained", field, raw, tag,
			"the retired default scope is preserved as an ordinary descriptive tag rather than stripped")
	}
	return tag, true
}

// v7TagNamer allocates one canonical tag per distinct retired name. Two names
// that sanitize identically share a tag, which is what collapses a context and
// a scope name that already read the same; two that differ but truncate to the
// same bytes are kept apart with a numeric suffix.
type v7TagNamer struct {
	byIdentity map[string]string
	assigned   map[string]bool
}

func newV7TagNamer() *v7TagNamer {
	return &v7TagNamer{
		byIdentity: map[string]string{models.ParkedProjectTag: models.ParkedProjectTag},
		assigned:   map[string]bool{models.ParkedProjectTag: true},
	}
}

// assign takes an already sanitized identity. Tag validity is not re-asserted
// here: NormalizeProjectTags validates the assembled set and is the one seam
// every tag reaches the catalog through.
func (n *v7TagNamer) assign(identity string, alias bool) string {
	key := identity
	if alias {
		key = v7ParkedAliasIdentity
	}
	if existing, known := n.byIdentity[key]; known {
		return existing
	}
	tag := truncateV7Tag(identity, models.MaxTagLength)
	for suffix := 2; n.assigned[tag]; suffix++ {
		mark := "-" + strconv.Itoa(suffix)
		tag = truncateV7Tag(identity, models.MaxTagLength-len(mark)) + mark
	}
	n.byIdentity[key] = tag
	n.assigned[tag] = true
	return tag
}

// sanitizeV7ScopeName rewrites a retired name into canonical tag bytes. A
// budget of zero keeps the untruncated identity used to decide whether two
// names were the same to begin with.
func sanitizeV7ScopeName(raw string, budget int) string {
	// Every character a tag cannot carry becomes a space so that runs of them
	// collapse into a single separator instead of a hyphen per rejected rune.
	spaced := strings.Map(func(r rune) rune {
		switch {
		case r == ',' || r == ';' || r < 0x20 || r == 0x7f:
			return ' '
		case unicode.IsSpace(r):
			return ' '
		default:
			return r
		}
	}, raw)
	folded := strings.ToLower(strings.Join(strings.Fields(spaced), "-"))
	for {
		next := models.NormalizeTag(folded)
		if next == folded {
			break
		}
		folded = next
	}
	return truncateV7Tag(folded, budget)
}

func truncateV7Tag(value string, budget int) string {
	if budget <= 0 || len(value) <= budget {
		return value
	}
	for len(value) > budget {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}
