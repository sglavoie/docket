package route

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

// OverlayKind classifies transient presentations independently from primary
// route identity and history.
type OverlayKind uint8

const (
	OverlayComposer OverlayKind = iota
	OverlayForm
	OverlayPicker
	OverlayTaskTool
	OverlayReference
)

var overlayKindNames = [...]string{
	"Composer",
	"Form",
	"Picker",
	"TaskTool",
	"Reference",
}

func (kind OverlayKind) String() string {
	if int(kind) < len(overlayKindNames) {
		return overlayKindNames[kind]
	}
	return fmt.Sprintf("OverlayKind(%d)", kind)
}

// Overlay is a typed transient presentation. Key distinguishes nested
// instances of the same kind without leaking view implementation state into
// navigation.
type Overlay struct {
	kind            OverlayKind
	key             string
	reference       *Route
	presentation    presentation.PresentationID
	hasPresentation bool
}

func NewOverlay(kind OverlayKind, key string) (Overlay, error) {
	if kind > OverlayReference {
		return Overlay{}, fmt.Errorf("invalid overlay kind: %d", kind)
	}
	if kind == OverlayReference {
		return Overlay{}, fmt.Errorf("reference overlays require NewReferenceOverlay")
	}
	return Overlay{kind: kind, key: key}, nil
}

func NewReferenceOverlay(route Route) (Overlay, error) {
	if route.ID() != Reference {
		return Overlay{}, fmt.Errorf("reference overlay requires Reference route, got %s", route.ID())
	}
	return Overlay{kind: OverlayReference, reference: &route}, nil
}

func (o Overlay) Kind() OverlayKind { return o.kind }

func (o Overlay) Key() string { return o.key }

func (o Overlay) ReferenceRoute() (Route, bool) {
	if o.reference == nil {
		return Route{}, false
	}
	return *o.reference, true
}

func (o Overlay) Presentation() (presentation.PresentationID, bool) {
	return o.presentation, o.hasPresentation
}
