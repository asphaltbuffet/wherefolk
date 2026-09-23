# Keep navigation state in the URL

The two-pane interface (§4.1) needs to know which Household is selected and which Branches are
expanded. §2.1 fixes the frontend as server-rendered templates with htmx and permits hand-written
JavaScript only for tree expand/collapse, and ADR-0001 removed identity entirely, so there is no
session to hang state on and no accounts to key it by. Navigation state therefore lives in the URL:
`/h/{id}` names the selection, `?open=` carries the expanded set as comma-separated Household IDs,
and `?close=` removes one from it. Nothing is stored in a cookie, a session, or client-side state.

The server always expands the selection's own ancestors, whatever the query string asks for. A
`?close=` that names an ancestor of the selection is therefore applied *before* that chain is
re-added, so it cannot hide the Household the detail pane is showing — §4.3 makes search a shortcut
*through* the tree, and a selected Household missing from the tree would be precisely the detached
view that requirement exists to prevent. Because such a toggle can never act, those nodes render a
plain marker rather than a control.

## Consequences

Every view is deep-linkable and the browser's back button retraces navigation exactly, which matters
for an Editor who treats a wrong click as a mistake to undo rather than a step to repeat. Expansion
survives a reload, a bookmark, and a restart of the service. An htmx swap does not touch the address
bar by itself, so the toggles carry `hx-push-url`; without it the tree would change while the URL
stood still and the back button would retrace nothing.

Each toggle costs a round-trip over the tailnet. §2.1 licensed hand-written JavaScript to make
expand/collapse feel instant; that licence is deferred rather than spent, and can be taken up later
without changing the server, because the server-rendered state remains the fallback and the toggles
are ordinary links.

Household IDs reach the query string, so the toggle URL is built with `url.Values` rather than
concatenated in the template. `html/template` percent-encodes into `href`, which it recognises as a
URL attribute, but not into `hx-get`, which it treats as ordinary text; an ID containing `&` would
otherwise terminate the parameter and silently drop a node from the open set. IDs are not guaranteed
to be URL-safe — `store.Load` accepts a hand-repaired document exactly as written, and validation in
this project observes rather than rejects (§4.5).

A deeply expanded tree makes a long URL. At a few hundred Households this stays well inside every
browser's limit, and the open set is emitted in the tree's own depth-first order so the same
expansion always produces the same URL rather than filling history with entries that differ only by
map iteration order.
