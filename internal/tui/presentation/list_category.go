package presentation

// ListCategoryID is the stable identity of a list category.
//
// A list category is a list-surface concept, but the operation manifest names
// one on every entry so an action sheet can group the operations it lists. That
// puts the same bare identity on both sides of a layer boundary the list
// surface sits above, so it lives here for the reason PresentationID does: an
// identity two layers read is vocabulary below both rather than a type either
// one owns. The list surface still spells it ListCategoryID through an alias,
// so nothing there has to name this package to keep its own vocabulary.
type ListCategoryID string
