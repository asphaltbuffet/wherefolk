package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrTypstMissing reports that the typst binary could not be found.
//
// Typst is a host dependency rather than vendored (ADR-0004), so its absence is
// an Operator-facing deployment fault, not something the Editor should ever meet
// as an opaque error mid-export (§5.1a).
var ErrTypstMissing = errors.New("typst not found on PATH")

// pdfName and svgPattern are the output names inside the scratch directory.
//
// The SVG name carries Typst's {p} page-number placeholder: SVG export writes
// one file per page, and Typst refuses an output name without a placeholder
// once a document runs past a single page. A Directory always will.
const (
	pdfName    = "out.pdf"
	svgPattern = "out-{p}.svg"
	svgGlob    = "out-*.svg"
)

// sourceName is the basename of the generated markup inside the scratch
// directory. The generated markup imports "directory.typ" as a sibling, so the
// two must land in the same directory.
const sourceName = "directory-source.typ"

// Typst invokes the typst binary as a subprocess.
//
// Go never performs layout: it generates markup and shells out, so that one
// renderer produces both the in-app preview and the printed export and the
// preview cannot drift from what prints (§5.1a).
type Typst struct {
	// Bin is the resolved path to the typst executable.
	Bin string
}

// NewTypst resolves typst on PATH.
func NewTypst() (Typst, error) {
	bin, err := exec.LookPath("typst")
	if err != nil {
		return Typst{}, fmt.Errorf("%w: %w", ErrTypstMissing, err)
	}

	return Typst{Bin: bin}, nil
}

// Version returns the binary's self-reported version, e.g.
// "typst 0.14.2 (b33de9de)".
//
// The application verifies presence and version at startup rather than at
// export time, so a missing or wrong binary fails with an Operator-facing
// message while the Operator is still watching the logs (ADR-0004).
func (t Typst) Version(ctx context.Context) (string, error) {
	var out bytes.Buffer

	//nolint:gosec // Bin is resolved via exec.LookPath, not attacker input.
	cmd := exec.CommandContext(ctx, t.Bin, "--version")
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("typst --version: %w: %s", err, strings.TrimSpace(out.String()))
	}

	return strings.TrimSpace(out.String()), nil
}

// CompilePDF renders markup as a single PDF, for export.
func (t Typst) CompilePDF(ctx context.Context, markup, templateDir string) ([]byte, error) {
	var out []byte

	err := t.compile(ctx, markup, templateDir, "pdf", pdfName, func(dir string) error {
		b, err := os.ReadFile(filepath.Join(dir, pdfName))
		if err != nil {
			return fmt.Errorf("typst: read output: %w", err)
		}

		out = b

		return nil
	})

	return out, err
}

// CompileSVG renders markup as one SVG per page, for the in-app preview.
//
// Typst writes a file per page, so the result is a slice. Pages come back in
// document order: the filenames are numeric, and a plain directory listing
// sorts "out-10.svg" before "out-2.svg", so they are collected by index rather
// than by name.
func (t Typst) CompileSVG(ctx context.Context, markup, templateDir string) ([][]byte, error) {
	var pages [][]byte

	err := t.compile(ctx, markup, templateDir, "svg", svgPattern, func(dir string) error {
		matches, err := filepath.Glob(filepath.Join(dir, svgGlob))
		if err != nil {
			return fmt.Errorf("typst: find svg pages: %w", err)
		}

		if len(matches) == 0 {
			return errors.New("typst: no svg pages were written")
		}

		pages = make([][]byte, len(matches))

		// Read by page number rather than by the glob's lexical order.
		for i := range matches {
			name := strings.Replace(svgPattern, "{p}", strconv.Itoa(i+1), 1)

			b, readErr := os.ReadFile(filepath.Join(dir, name))
			if readErr != nil {
				return fmt.Errorf("typst: read svg page %d: %w", i+1, readErr)
			}

			pages[i] = b
		}

		return nil
	})

	return pages, err
}

// compile runs typst in a scratch directory and hands that directory to collect.
//
// The markup is written alongside a copy of the template, because the generated
// source imports "directory.typ" as a sibling; the scratch directory is also the
// --root, so nothing outside it can be imported. templateDir is the Operator's
// edited template, not a stale embedded copy (ADR-0004).
func (t Typst) compile(
	ctx context.Context,
	markup, templateDir, format, outName string,
	collect func(dir string) error,
) error {
	dir, err := os.MkdirTemp("", "wherefolk-render-")
	if err != nil {
		return fmt.Errorf("typst: scratch directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	err = stageTemplate(dir, templateDir)
	if err != nil {
		return err
	}

	src := filepath.Join(dir, sourceName)

	err = os.WriteFile(src, []byte(markup), 0o600)
	if err != nil {
		return fmt.Errorf("typst: write markup: %w", err)
	}

	var stderr bytes.Buffer

	//nolint:gosec // Bin is resolved via exec.LookPath, not attacker input.
	cmd := exec.CommandContext(ctx, t.Bin,
		"compile", "--root", dir, "--format", format, src, filepath.Join(dir, outName))
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("typst compile: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return collect(dir)
}

// stageTemplate copies every .typ file from templateDir into dir, so the
// generated source can import the template as a sibling.
func stageTemplate(dir, templateDir string) error {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return fmt.Errorf("typst: read template directory %s: %w", templateDir, err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".typ" {
			continue
		}

		b, readErr := os.ReadFile(filepath.Join(templateDir, e.Name()))
		if readErr != nil {
			return fmt.Errorf("typst: read template %s: %w", e.Name(), readErr)
		}

		if writeErr := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o600); writeErr != nil {
			return fmt.Errorf("typst: stage template %s: %w", e.Name(), writeErr)
		}
	}

	return nil
}
