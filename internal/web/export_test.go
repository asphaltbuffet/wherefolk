package web

import (
	"context"
	"net/http"
	"strings"

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

// TreeNodesForTest exposes tree rendering to the external test package.
//
// It routes through openSet rather than calling treeNodes with the bare map,
// because openSet is what folds the selection's ancestors into the open set.
// Calling treeNodes directly would test a layer in isolation that production
// never uses that way, and would invite pushing openSet's rule down into
// treeNodes so the isolated call looked right.
func (s *Server) TreeNodesForTest(selected rolo.HouseholdID, open map[rolo.HouseholdID]bool) []treeNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(open))
	for id, isOpen := range open {
		if isOpen {
			ids = append(ids, string(id))
		}
	}

	return s.treeNodes(selected, s.openSet(selected, strings.Join(ids, ",")))
}

// OpenSetForTest exposes openSet to the external test package.
func (s *Server) OpenSetForTest(selected rolo.HouseholdID, raw string) map[rolo.HouseholdID]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.openSet(selected, raw)
}

// ToggleURLForTest exposes toggleURL to the external test package.
func ToggleURLForTest(selected rolo.HouseholdID, openList string, id rolo.HouseholdID, isOpen bool) string {
	return toggleURL(selected, openList, id, isOpen)
}

// HouseholdViewForTest is householdView, exported for the external test package.
type HouseholdViewForTest = householdView

// HouseholdViewForTest exposes householdView to the external test package.
func (s *Server) HouseholdViewForTest(id rolo.HouseholdID) (householdView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.householdView(id)
}
