package main

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger discards output, so a test's logging cannot reach the test binary's
// own stderr or leak into another test.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunStartupFailures(t *testing.T) {
	// A directory holding no document, to exercise the missing-store path.
	empty := t.TempDir()

	// A directory holding a document the binary is too old to read.
	tooNew := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tooNew, "directory.json"),
		[]byte(`{"schema": 99, "households": []}`),
		0o600,
	))

	// A document that parses but does not describe a valid tree.
	badTree := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(badTree, "directory.json"),
		[]byte(
			`{"schema":1,"households":[{"id":"h_orphan","parent":"h_missing","adults":[{"id":"p_x","given":"X","surname":"Y","birth":"","death":"","phone":"","email":""}]}]}`,
		),
		0o600,
	))

	// A document the process is not allowed to read.
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	unreadable := t.TempDir()
	unreadablePath := filepath.Join(unreadable, "directory.json")
	require.NoError(t, os.WriteFile(unreadablePath, []byte(`{"schema":1,"households":[]}`), 0o600))
	require.NoError(t, os.Chmod(unreadablePath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(unreadablePath, 0o600) })

	tests := []struct {
		name    string
		vars    map[string]string
		wantErr string
	}{
		{
			name:    "invalid port",
			vars:    map[string]string{"WHEREFOLK_PORT": "http", "WHEREFOLK_DATA": empty},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name:    "missing document",
			vars:    map[string]string{"WHEREFOLK_DATA": empty},
			wantErr: "read document",
		},
		{
			name:    "document newer than the binary",
			vars:    map[string]string{"WHEREFOLK_DATA": tooNew},
			wantErr: "newer",
		},
		{
			name:    "document that does not describe a valid tree",
			vars:    map[string]string{"WHEREFOLK_DATA": badTree},
			wantErr: "validate document",
		},
		{
			name:    "document that cannot be read",
			vars:    map[string]string{"WHEREFOLK_DATA": unreadable},
			wantErr: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer

			err := run(func(key string) string { return tt.vars[key] }, &logs)

			require.Error(t, err, "startup misconfiguration must be fatal")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestServeShutsDownOnSignal checks that a stop signal ends serve cleanly
// rather than being ignored until the supervisor loses patience and SIGKILLs.
//
// The signal is real and process-wide, so these rows run sequentially and this
// test must not be made parallel: [signal.NotifyContext] hooks the whole process,
// and a concurrent test raising its own signal would be delivered here too.
func TestServeShutsDownOnSignal(t *testing.T) {
	tests := []struct {
		name   string
		signal syscall.Signal
	}{
		{name: "SIGTERM from docker stop", signal: syscall.SIGTERM},
		{name: "SIGINT from a terminal", signal: syscall.SIGINT},
	}

	// serve deregisters its own handler while shutting down, so between one row
	// finishing and the next installing its handler the process has none — and a
	// signal landing in that window would kill the test binary by default
	// disposition. This registration spans every row and keeps a handler
	// installed throughout; it never reads the channel, it only holds the
	// disposition off.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, os.Interrupt, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(guard) })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			httpSrv := &http.Server{
				Handler:           http.NotFoundHandler(),
				ReadHeaderTimeout: readHeaderTimeout,
			}

			done := make(chan error, 1)
			go func() { done <- serve(httpSrv, ln, testLogger()) }()

			// Wait until the server is actually serving before signalling it.
			// This does not prove NotifyContext has run — the listener was
			// already accepting before serve was called — so the guard above is
			// what makes an early signal survivable.
			require.Eventually(t, func() bool {
				// A distinct name, not the outer err: this closure runs on the
				// polling goroutine's schedule, so assigning to the outer
				// variable would race with the assertions below.
				conn, dialErr := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
				if dialErr != nil {
					return false
				}
				_ = conn.Close()

				return true
			}, 5*time.Second, 10*time.Millisecond, "server never accepted a connection")

			require.NoError(t, syscall.Kill(os.Getpid(), tt.signal))

			select {
			case serveErr := <-done:
				require.NoError(t, serveErr, "a stop signal is a clean exit, not a failure")
			case <-time.After(10 * time.Second):
				t.Fatal("serve ignored the signal and kept running")
			}
		})
	}
}

// TestServeReportsListenerFailure checks that a genuine Serve error is still
// reported, rather than being swallowed by the shutdown path.
func TestServeReportsListenerFailure(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{name: "listener closed underneath the server", wantErr: "serve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			require.NoError(t, ln.Close())

			err = serve(&http.Server{
				Handler:           http.NotFoundHandler(),
				ReadHeaderTimeout: readHeaderTimeout,
			}, ln, testLogger())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
