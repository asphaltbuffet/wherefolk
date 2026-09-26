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
	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
)

func main() {
	err := run(os.Getenv, os.Stderr)
	if err != nil {
		// run owns the configured logger, and a failure may predate it, so the
		// last word is written plainly to stderr rather than through slog.
		fmt.Fprintln(os.Stderr, "startup failed:", err)
		os.Exit(1)
	}
}

// run performs every fallible startup step and returns rather than exiting, so
// that tests can exercise the failure paths. It returns only on error or when
// the server stops.
func run(getenv func(string) string, logOut io.Writer) error {
	// Configuration is read before the logger is built, because the logger's
	// level comes from it. A failure here therefore has nowhere structured to go
	// and is returned for main to report.
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if cfg.FullPassphrase.Reveal() == "" {
		// Not fatal (see config.Config.FullPassphrase), but the Operator must
		// hear about it before the Editor does.
		logger.Warn("WHEREFOLK_FULL_PASSPHRASE is not set; the Full tier is unavailable")
	}

	// A document that will not load is fatal: a container that boots into an
	// error page passes its own health check and hides the fault (§2.4).
	docPath := cfg.DocumentPath()

	doc, err := store.Load(docPath)
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// Typst is a host dependency, not vendored (ADR-0004). Verifying it here
	// means a missing or unreadable renderer is an Operator-facing startup
	// failure in the logs, rather than an opaque error the Editor meets
	// halfway through an export (§5.1a).
	_, typstVersion, err := verifyRenderer(cfg.TemplateDir)
	if err != nil {
		return err
	}

	srv, err := web.New(doc, cfg, logger,
		web.Meta{
			DocumentPath: docPath,
			TypstVersion: typstVersion,
			TemplatePath: cfg.TemplateDir,
		},
		func(d *store.Document) error { return store.Save(docPath, d) },
		store.NewHouseholdID,
		store.NewPersonID,
	)
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	addr := net.JoinHostPort(web.Host, strconv.Itoa(cfg.Port))

	// ListenConfig rather than net.Listen so the bind can be cancelled. serve
	// installs the signal handler after this returns, so the context here is
	// only a placeholder — but it is the seam a startup timeout would use.
	var lc net.ListenConfig

	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	logger.Info("serving",
		"addr", ln.Addr().String(),
		"document", docPath,
		"typst", typstVersion,
		"households", len(doc.Households))

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serve(ctx, stop, httpSrv, ln, logger)
}

// shutdownTimeout bounds how long a stopping server waits for in-flight
// requests. It sits well inside Docker's default ten-second SIGTERM grace
// period, so the process exits on its own terms rather than being SIGKILLed.
const (
	shutdownTimeout = 5 * time.Second

	// readHeaderTimeout bounds how long a client may take to send its headers,
	// so a stalled connection cannot hold a handler open indefinitely.
	readHeaderTimeout = 10 * time.Second
)

// serve runs the server until it fails or ctx is cancelled, and calls stop once
// shutdown begins so a second signal is no longer intercepted.
//
// Without this, SIGTERM from `docker stop` would be ignored and the container
// SIGKILLed once the grace period expired. Every route is a read today, so the
// cost would only be dropped responses — but item 5 adds the write path, and
// the atomic temp/fsync/rename in internal/store protects a write that has
// begun, not one that never got to run.
// The caller owns the signal registration rather than serve creating it, so
// that it is demonstrably installed before anything can signal the process. A
// registration made here would race with a signal raised immediately after
// serve is called — which is exactly what a test does.
func serve(ctx context.Context, stop func(), httpSrv *http.Server, ln net.Listener, logger *slog.Logger) error {
	errc := make(chan error, 1)

	go func() { errc <- httpSrv.Serve(ln) }()

	select {
	case err := <-errc:
		// Serve only returns on a real failure here: the shutdown path below
		// is the only thing that closes the server, and it returns instead.
		return fmt.Errorf("serve: %w", err)

	case <-ctx.Done():
		logger.InfoContext(ctx, "shutting down", "timeout", shutdownTimeout)

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

// verifyRenderer resolves the Typst renderer and reports its version.
func verifyRenderer(templateDir string) (render.Renderer, string, error) {
	renderer, err := render.NewRenderer(templateDir)
	if err != nil {
		return render.Renderer{}, "", fmt.Errorf("renderer: %w", err)
	}

	version, err := renderer.Typst.Version(context.Background())
	if err != nil {
		return render.Renderer{}, "", fmt.Errorf("renderer: %w", err)
	}

	return renderer, version, nil
}
