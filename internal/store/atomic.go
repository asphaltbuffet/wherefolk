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
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Remove the temp file on any failure after this point. Once the rename
	// succeeds the temp name no longer exists and the remove is a harmless
	// no-op.
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Flush to the platter before renaming. Without this the rename can land
	// while the contents are still in the page cache, so a crash leaves the
	// target pointing at an empty or partial file.
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}

	return nil
}
