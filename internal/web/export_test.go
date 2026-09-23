package web

import (
	"context"
	"net/http"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// RenderFragmentForTest exposes renderFragment to the external test package.
// It exists only in test builds.
func (s *Server) RenderFragmentForTest(
	ctx context.Context,
	w http.ResponseWriter,
	page, fragment string,
	data any,
) error {
	return s.renderFragment(ctx, w, page, fragment, data)
}

// TreeNodeForTest is treeNode, exported for the external test package.
type TreeNodeForTest = treeNode

// TreeNodesForTest exposes treeNodes to the external test package.
func (s *Server) TreeNodesForTest(selected rolo.HouseholdID, open map[rolo.HouseholdID]bool) []treeNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.treeNodes(selected, open)
}

// OpenSetForTest exposes openSet to the external test package.
func (s *Server) OpenSetForTest(selected rolo.HouseholdID, raw string) map[rolo.HouseholdID]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.openSet(selected, raw)
}
