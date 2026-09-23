package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path so that a reader sees either the previous
// contents or the complete new contents, never a partial write.
//
// The document is the only copy of the Directory, and the Editor's machine may
// lose power mid-save. Writing in place would risk leaving a truncated JSON
// file, so the new contents go to a temp file in the same directory, are
// flushed to disk, and are then renamed over the target — rename is atomic
// within a filesystem.
//
// The file is written with documentPerm: the document is the only thing this
// package writes, so the permission is a property of the store, not a choice
// the caller makes.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Remove the temp file on any failure after this point. Once the rename
	// succeeds the temp name no longer exists and the remove is a harmless
	// no-op.
	// Both errors are deliberately discarded: this runs on the failure path,
	// where the caller is already returning the error that matters, and after a
	// successful rename both calls are expected no-ops.
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	err = tmp.Chmod(documentPerm)
	if err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	_, err = tmp.Write(data)
	if err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Flush to the platter before renaming. Without this the rename can land
	// while the contents are still in the page cache, so a crash leaves the
	// target pointing at an empty or partial file.
	err = tmp.Sync()
	if err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	err = os.Rename(tmpName, path)
	if err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}

	return nil
}
