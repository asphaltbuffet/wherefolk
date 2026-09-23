package web

import (
	"context"
	"net/http"
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
