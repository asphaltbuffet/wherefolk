# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [0.2.0] - 2026-11-15

### Added

- A one-line household summary under the heading (adults, dependents, anniversary), and memorial cards show the lifespan, e.g. "In memory, 1963–1981" [#42]
- Atkinson Hyperlegible Next and Besley fonts, self-hosted with their OFL licences so no page loads an asset from off the tailnet [#42]
- A solid focus ring on fields, and scroll padding so the Save bar never covers the focused field [#42]

### Changed

- The editor is restyled as index cards on a sage-linen desk: each person is a card with Name, Dates and Contact groups, "Hide" is now "Keep private", and "Save changes" sits in a bar pinned to the bottom [#42]
- The export, delete and Recently deleted pages use the same cards, and each export tier is a whole-tile choice [#42]
- "Give them a household of their own" and "Remove from the directory" move into a "More for {Given}" disclosure that opens itself when either is ticked [#42]
- Findings appear on the card of the person they concern; Household-level ones, and any about someone without a card, appear on the household card [#42]
- Built with Go 1.27.3 and pdfcpu 0.16.4; PDF encryption uses a stateless configuration and stops when the request is cancelled [#37, #41]
- The Tailscale sidecar image is pinned to v1.104.4 [#38]

### Fixed

- A refused save (422) keeps the own-household and remove choices ticked; before, they were silently cleared [#42]

[0.2.0]: https://github.com/asphaltbuffet/wherefolk/compare/v0.1.0...v0.2.0


## [0.1.0] - 2026-10-04

### Added

- Build, lint, release, and CI tooling — golangci-lint, goreleaser, changie, mise tasks, and GitHub Actions workflows
- internal/buildmeta reports the running build, injected via ldflags and surfaced on /status
- Container image published to ghcr.io for linux/amd64 and linux/arm64
- WHEREFOLK_LOG_LEVEL sets the log threshold (debug, info, warn, error; default info)
- deploy/ holds a compose file with a Tailscale sidecar, a Serve config that keeps Funnel off, and an Operator runbook for the tailnet setup

### Changed

- Codebase is clean against the strict golangci-lint config; the logger is now injected rather than taken from slog's package default

### Removed

- internal/tui, an unreferenced leftover from the pre-service CLI; this drops lipgloss and 12 transitive dependencies

### Fixed

- The container image creates its data directory owned by the service user, so a new named volume is writable

[0.1.0]: https://github.com/asphaltbuffet/wherefolk/compare/v0.0.0...v0.1.0

