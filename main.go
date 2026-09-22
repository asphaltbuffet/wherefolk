// Command wherefolk serves the family directory over HTTP.
//
// The binary takes no arguments: it is a service, not a toolbox, so no argument
// can make it do anything other than serve (§2.2). Configuration comes from the
// environment; see internal/config.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
)

func main() {
	if err := run(os.Getenv, os.Stderr); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

// run performs every fallible startup step and returns rather than exiting, so
// that tests can exercise the failure paths. It returns only on error or when
// the server stops.
func run(getenv func(string) string, logOut io.Writer) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(logOut, nil)))

	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	// A document that will not load is fatal: a container that boots into an
	// error page passes its own health check and hides the fault (§2.4).
	doc, err := store.Load(cfg.DocumentPath())
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	srv, err := web.New(doc, web.Meta{DocumentPath: cfg.DocumentPath()})
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	addr := net.JoinHostPort(web.Host, strconv.Itoa(cfg.Port))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	slog.Info("serving", "addr", ln.Addr().String(), "document", cfg.DocumentPath(), "households", len(doc.Households))

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	return httpSrv.Serve(ln)
}
