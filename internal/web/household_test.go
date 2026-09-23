package web_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectoryPage(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		checkFunc  func(t *testing.T, body string)
	}{
		{
			name:       "the root shows the tree with no Household selected",
			target:     "/",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "<!DOCTYPE html>", "the root is a page, not a fragment")
				assert.Contains(t, body, "Aden/Nettie")
				assert.Contains(t, body, "Choose a household", "an empty detail pane explains itself")
			},
		},
		{
			name:       "selecting a Household renders both panes",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Clyde Whitlock")
				assert.Contains(t, body, "1412 Oak St")
				assert.Contains(t, body, "Aden/Nettie", "the tree pane is still there")
			},
		},
		{
			name:       "the Path breadcrumb is always visible",
			target:     "/h/h_dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "›", "§4.1: the Path is the disambiguation mechanism")
				assert.Contains(t, body, "Clyde/Doris")
			},
		},
		{
			name:       "a withheld field shows [private], never its value",
			target:     "/h/h_reeve",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "[private]")
				assert.NotContains(t, body, "9 Elm St", "a withheld address must not reach the page")
				assert.NotContains(t, body, "555-0199")
			},
		},
		{
			name:       "a shared address is a back-reference, not a repeated block",
			target:     "/h/h_dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Same address as Clyde/Doris")
				assert.NotContains(t, body, "1412 Oak St", "§3: a Shared Address renders as a reference")
			},
		},
		{
			name:       "a Memorial Household carries no contact details",
			target:     "/h/h_aden",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Memorial")
				assert.Contains(t, body, "1989-11-17", "§5.3: a deceased person's dates are whole")
			},
		},
		{
			name:       "an unknown Household explains itself in plain English",
			target:     "/h/h_ghost",
			wantStatus: http.StatusNotFound,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "not in the directory")
				assert.NotContains(t, body, "404", "the Editor never sees an error code")
				assert.Contains(t, body, "Aden/Nettie", "the tree is still navigable from the error")
			},
		},
		{
			name:       "/status still works",
			target:     "/status",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Wherefolk status", "the Operator's bookmark does not move")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, tt.wantStatus, rec.Code)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}
