[← Architecture index](../architecture.md)

## Canonical query

`models.Query` is the one narrowing predicate a view holds. It is an ordered
clause list plus three axes that are deliberately not clauses. It lives in
`internal/models` because that is the only package every carrier of a query
already imports: `internal/textgrammar` compiles a typed string into it,
`internal/config` persists that string beside a lens, and `internal/tui` applies
it and renders it back. No second query representation may be introduced beside
it.

### Clause vocabulary

A clause is a field, a polarity, an operator, and a value. The operator carries
the reading the field would otherwise have to guess: `is` matches exactly,
`prefix` matches an attribute the value starts, `contains` matches case
insensitively anywhere, `at_least` is a threshold along the field's own scale,
`none` matches an unset attribute, and `present` matches a boolean predicate.
`none` and `present` never carry a value. Exact and threshold priority are
therefore two clauses rather than one ambiguous one, and the same split gives
tag its exact and prefix readings.

The fields are free text, dependency, priority, tag, context, due, defer, state,
pinned, stale, deferred, completed, parent, child, leaf, project tag, parked,
and archived. There is no root field: root is the negation of child. `0` is the
reserved unset token across priority, context, due, and the project tag, which
is the same convention the capture grammar already spells `!0`, `##0`, and `@0`
with.

A predicate belongs in this vocabulary only when the triple `(task, project,
now)` decides it. That rule is what keeps `Admits` total and side-effect free,
and it is why four otherwise plausible predicates are absent: whether a
dependency target is still open, whether a task belongs to the Today or Next
planning collection, whether a project note matches free text, and the Review
flag. Each needs resolution the triple cannot supply. Admitting one means giving
the evaluator a resolution context, never widening `Admits`.

Free text matches over the task-side corpus: title, description, context, tags,
checklist items, comments, attachment filenames and paths, and the owning
project's name and slug. Notes are outside a task and so are outside admission,
even though the Outliner's flat ranked list offers them as its lowest tier.

### The join law

The law is one rule with one consequence, applied uniformly:

- across fields, AND;
- within a field, OR when the underlying task attribute is single-valued;
- within a field, AND when the attribute is multi-valued;
- negative clauses, always AND-of-nots, at any cardinality and on any field.

Field cardinality alone decides the within-field join, so two contexts union
while two tags intersect. ANDing two values of a single-valued attribute would
make the query unsatisfiable rather than narrow, and ORing two values of a
multi-valued one would widen exactly the query a user wrote to narrow. A field
carrying only negatives admits a row the field cannot match at all: nothing is
being asked for, only excluded.

### Canonical order, equality, and fingerprint

Clause order is not meaning. `Canonical` deduplicates, re-derives each clause's
reload obligation from its field, and sorts by field in the declared field
order, then positives before negatives, then by operator rank, then by value.
`String` renders that canonical list in the sigil vocabulary joined by single
spaces, and negation is always the leading `-`, on every field, so the canonical
form has exactly one spelling for "not this".

`Equal` compares canonical forms, and `Fingerprint` digests everything `Equal`
compares. Two queries share a fingerprint exactly when they are equal, so a
memoization guard keyed on the fingerprint can never be short-circuited by a
change it cannot see.

`ParseQuery` reads a canonical string back and accepts exactly what `String`
emits, nothing more. It is not the user-facing grammar: lenient input,
diagnostics, and completion stay in `internal/textgrammar`. What lives beside
the type is only the closure that makes the durable string trustworthy — a
persisted query parses back to the query it was rendered from, with no second
implementation of the vocabulary between them.

### The three non-clause axes

Mode, Scope, and Sort are axes rather than clauses because none of them can be
answered by looking at one task: Mode chooses which lanes a projection builds,
Scope chooses which projects load, and Sort chooses the order rows appear in.
They persist beside the canonical string rather than inside it, which matches
the shape a named lens already stores. An empty axis means the query does not
own it; an empty Scope in particular never means "force global".

The zero query narrows nothing. The base workspace population — non-archived and
not parked — is decided by the caller that loads projects, never by an implied
clause. The parked, archived, and project-tag clauses are the one place a clause
value is not purely a narrowing: they carry a reload obligation because the
population they name has to be loaded before any task of it can be evaluated.
Their `include` value narrows nothing and exists only to carry that obligation,
while `only` both loads and isolates. `RequiresReload` reads the canonical form,
so a hand-built clause that understates its own obligation is corrected before
it is believed.
