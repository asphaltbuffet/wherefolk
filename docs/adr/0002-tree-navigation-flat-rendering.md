# Navigate as a tree, store flat, render flat

Households are navigated as a hierarchy because a Path like `Aden/Nettie › Clyde/Doris ›
Dave/Diane` disambiguates relatives who share a given name far better than a search box does, but
the rendered Directory is a flat sequence of Household blocks — a Household never nests inside its
parent's block. Storage is therefore flat records with stable IDs and parent references, with the
tree derived for navigation and used only to order and group the output.

## Consequences

The existing recursive `Family` struct in `pkg/rolo`, where `Children []Family` is walked directly
to produce output, does not survive this decision. Stable IDs are required even though the user
never sees them, because a Path is a display identity that changes whenever the tree is
restructured.
