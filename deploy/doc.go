// Package deploy holds the Operator's deployment artefacts — the compose file,
// the Tailscale Serve config and the secrets template — and the tests that pin
// the properties ADR-0001, ADR-0006 and ADR-0007 depend on. It has no runtime
// code: it exists so that `go test ./...` checks the deployment as it checks
// everything else.
package deploy
