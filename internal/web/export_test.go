package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// ExportDateForTest exposes exportDate to the external test package.
func ExportDateForTest(now time.Time) time.Time { return exportDate(now) }

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

// JoinIDsOrderedForTest exposes joinIDsOrdered to the external test package.
func (s *Server) JoinIDsOrderedForTest(open map[rolo.HouseholdID]bool) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.joinIDsOrdered(open)
}

// SubmissionForTest is submission, exported for the external test package.
type SubmissionForTest = submission

// PersonSubmissionForTest is personSubmission, exported for the external test
// package.
type PersonSubmissionForTest = personSubmission

// FieldErrorForTest is fieldError, exported for the external test package.
type FieldErrorForTest = fieldError

// ParseSubmissionForTest exposes parseSubmission to the external test package.
func ParseSubmissionForTest(form url.Values, h rolo.Household) (submission, []fieldError) {
	return parseSubmission(form, h)
}

// ChangeForTest is change, exported for the external test package.
type ChangeForTest = change

// CloneDocumentForTest exposes cloneDocument to the external test package.
func CloneDocumentForTest(doc *store.Document) *store.Document { return cloneDocument(doc) }

// ApplySubmissionForTest exposes applySubmission to the external test package.
func (s *Server) ApplySubmissionForTest(
	doc *store.Document,
	id rolo.HouseholdID,
	sub submission,
) ([]change, error) {
	return s.applySubmission(doc, id, sub)
}

// HouseholdFormViewForTest is householdFormView, exported for the external test
// package.
type HouseholdFormViewForTest = householdFormView

// FormViewFromHouseholdForTest exposes formViewFromHousehold to the external
// test package.
func FormViewFromHouseholdForTest(
	id rolo.HouseholdID,
	h rolo.Household,
	findings []rolo.Finding,
) householdFormView {
	return formViewFromHousehold(id, h, findings)
}

// FormViewFromSubmissionForTest exposes formViewFromSubmission to the external
// test package.
func FormViewFromSubmissionForTest(
	id rolo.HouseholdID,
	sub submission,
	errs []fieldError,
) householdFormView {
	return formViewFromSubmission(id, sub, errs)
}
