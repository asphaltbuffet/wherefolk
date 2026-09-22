// Command wherefolk serves the family directory over HTTP.
//
// The binary takes no arguments: it is a service, not a toolbox, so no argument
// can make it do anything other than serve (§2.2). Configuration comes from the
// environment; see internal/config.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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

	return serve(httpSrv, ln)
}

// shutdownTimeout bounds how long a stopping server waits for in-flight
// requests. It sits well inside Docker's default ten-second SIGTERM grace
// period, so the process exits on its own terms rather than being SIGKILLed.
const shutdownTimeout = 5 * time.Second

// serve runs the server until it fails or the process is asked to stop.
//
// Without this, SIGTERM from `docker stop` would be ignored and the container
// SIGKILLed once the grace period expired. Every route is a read today, so the
// cost would only be dropped responses — but item 5 adds the write path, and
// the atomic temp/fsync/rename in internal/store protects a write that has
// begun, not one that never got to run.
func serve(httpSrv *http.Server, ln net.Listener) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)

	go func() { errc <- httpSrv.Serve(ln) }()

	select {
	case err := <-errc:
		// Serve only returns on a real failure here: the shutdown path below
		// is the only thing that closes the server, and it returns instead.
		return fmt.Errorf("serve: %w", err)

	case <-ctx.Done():
		slog.Info("shutting down", "timeout", shutdownTimeout)

		// Stop intercepting signals, so a second SIGTERM from an impatient
		// operator kills the process rather than being swallowed.
		stop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}

		return nil
	}
}
