// Package web serves the Wherefolk UI over HTTP.
//
// The server binds loopback only. That is a security property, not a setting:
// ADR-0001 removed authentication on the grounds that the tailnet authenticates
// and the binary cannot be reached from beyond the host. See ADR-0007.
package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/asphaltbuffet/wherefolk/internal/config"
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
	mu  sync.RWMutex
	doc *store.Document
	// tree is DERIVED from doc, not independent state. Any future write path
	// must rebuild it inside the same write lock that mutates doc, or the two
	// silently disagree and navigation renders a tree that no longer exists.
	tree *rolo.Tree
	meta Meta
	cfg  config.Config
	log  *slog.Logger
}

// Meta carries Operator-facing facts about the running service that the
// Document itself does not contain. It holds resolved values, never paths to
// resolve: internal/web must stay free of filesystem concerns.
type Meta struct {
	DocumentPath string
}

// New builds a Server over an already-loaded document. It takes a document
// rather than a path so the web layer has no filesystem dependency; main owns
// loading and treats failure as fatal.
//
// The logger is injected rather than taken from slog's package default so that
// nothing here depends on process-global state: main builds it at the level cfg
// carries and owns where the output goes.
func New(doc *store.Document, cfg config.Config, logger *slog.Logger, meta Meta) (*Server, error) {
	if doc == nil {
		return nil, errors.New("web: document is nil")
	}

	if logger == nil {
		return nil, errors.New("web: logger is nil")
	}

	tree, err := doc.Tree()
	if err != nil {
		return nil, fmt.Errorf("web: build tree: %w", err)
	}

	return &Server{doc: doc, tree: tree, meta: meta, cfg: cfg, log: logger}, nil
}

// Handler returns the server's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// /status is the Operator's diagnostics page and keeps its own route so the
	// bookmark never moves; everything else is the Editor's two-pane interface.
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /{$}", s.handleDirectory)
	mux.HandleFunc("GET /h/{id}", s.handleDirectory)
	mux.HandleFunc("GET /tree", s.handleTree)
	mux.HandleFunc("GET /search", s.handleSearch)

	// Vendored assets, served from the embedded FS so the binary stays a single
	// file with no runtime dependency on a directory beside it. The embed root
	// already carries the "static/" prefix the URLs use, so the FS is served
	// as-is rather than sub-rooted.
	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	return mux
}
