package capturetext

import "github.com/sglavoie/tt/internal/models"

// LineError is one refusal reported against one input line. Both the multiline
// composer and the outline buffer validate every nonblank line before they
// commit anything, so a refusal has to name the line it came from rather than
// stop at the first problem.
type LineError struct {
	Line    int
	Column  int // zero-based rune column; zero when no token owns the refusal
	Message string
	// UnknownProject is the %project value no available project holds, set only
	// when that is the line's refusal, so a caller can tell the refusal apart
	// without reading Message.
	UnknownProject string
}

// CreatableProject reports the slug this refusal could create: the line was
// refused for naming a project nobody holds, and the value is already a
// canonical slug. A value the slug rules would rewrite, such as %Work, is
// not one the user has spelled as a slug, so it stays an ordinary refusal.
func (e LineError) CreatableProject() (string, bool) {
	slug := e.UnknownProject
	if slug == "" || models.Slugify(slug) != slug || models.ValidateProjectSlug(slug) != nil {
		return "", false
	}
	return slug, true
}
