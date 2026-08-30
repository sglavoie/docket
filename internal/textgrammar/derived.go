package textgrammar

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// DerivedPredicate is one parsed ? value: the computed property being asked
// about, plus the day threshold only ?completed carries. Days is zero when the
// token asked the unqualified question, which for completion is "has a
// completion instant at all" rather than "completed zero days ago".
type DerivedPredicate struct {
	Name models.DerivedProperty
	Days int
}

// ParseDerived resolves one ? or -? value. It is deliberately stricter than the
// tokenizer: validDerivedTokenValue admits any qualifier so a wrong day count
// reaches a diagnostic by name, and this is where that qualifier is judged.
func ParseDerived(value string) (DerivedPredicate, error) {
	name, qualifier, qualified := strings.Cut(strings.ToLower(strings.TrimSpace(value)), ":")
	property := models.DerivedProperty(name)
	if !slices.Contains(models.DerivedProperties, property) {
		return DerivedPredicate{}, InvalidDerivedError(value)
	}
	if !qualified {
		return DerivedPredicate{Name: property}, nil
	}
	if property != models.DerivedCompleted {
		return DerivedPredicate{}, fmt.Errorf("?%s takes no value", name)
	}
	days, ok := parseDerivedAge(qualifier)
	if !ok {
		return DerivedPredicate{}, fmt.Errorf(
			"invalid completion age: ?%s:%s (use a day count such as ?%s:90%s)",
			name, qualifier, name, derivedAgeSuffix,
		)
	}
	return DerivedPredicate{Name: property, Days: days}, nil
}

// parseDerivedAge reads the "90d" shape. The unit is required and is days alone:
// the qualifier exists to ask "done long enough ago to stop caring", and no
// finer unit answers that question.
func parseDerivedAge(qualifier string) (int, bool) {
	body, found := strings.CutSuffix(qualifier, derivedAgeSuffix)
	if !found {
		return 0, false
	}
	days, err := strconv.Atoi(body)
	if err != nil || days <= 0 {
		return 0, false
	}
	return days, true
}

// InvalidDerivedError names the closed set a ? token must come from.
func InvalidDerivedError(raw string) error {
	return fmt.Errorf(
		"invalid derived property: ?%s (use %s)",
		raw,
		joinOptions(prefixed(derivedNames, "?")),
	)
}

// validDerivedTokenValue decides what the tokenizer consumes. It admits the
// closed set of names with any qualifier at all, which is priority's bargain
// rather than pin's: ? leads no ordinary prose word, so consuming ?completed:90
// to say the unit is missing beats leaving it to be searched as literal text.
func validDerivedTokenValue(value string) bool {
	name, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(value)), ":")
	return slices.Contains(models.DerivedProperties, models.DerivedProperty(name))
}
