// Package web serves the Wherefolk UI over HTTP.
//
// The server binds loopback only. That is a security property, not a setting:
// ADR-0001 removed authentication on the grounds that the tailnet authenticates
// and the binary cannot be reached from beyond the host. See ADR-0007.
package web

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// Host is the only interface the server ever binds. It is deliberately not
// configurable; only the port is. See ADR-0007.
const Host = "127.0.0.1"

// Server holds the loaded Directory and renders it. The document is authoritative
// in memory: the server is the single writer, which is what lets item 6's undo
// have a coherent place to live.
type Server struct {
	mu   sync.RWMutex
	doc  *store.Document
	tree *rolo.Tree
}

// New builds a Server over an already-loaded document. It takes a document
// rather than a path so the web layer has no filesystem dependency; main owns
// loading and treats failure as fatal.
func New(doc *store.Document) (*Server, error) {
	if doc == nil {
		return nil, fmt.Errorf("web: document is nil")
	}

	tree, err := doc.Tree()
	if err != nil {
		return nil, fmt.Errorf("web: build tree: %w", err)
	}

	return &Server{doc: doc, tree: tree}, nil
}

// Handler returns the server's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Item 4 replaces the "/" registration with the two-pane editing UI; /status
	// keeps its own route so the Operator's bookmark never moves.
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /{$}", s.handleStatus)

	return mux
}
