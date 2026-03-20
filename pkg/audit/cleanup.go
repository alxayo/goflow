package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ApplyRetention deletes old run directories to keep only the most recent N.
// Directories are sorted by name (timestamp prefix ensures chronological order).
// If retention is 0 or negative, keeps all runs. Missing audit dir is a no-op.
func ApplyRetention(auditDir string, retention int) error {
	if retention <= 0 {
		return nil
	}

	entries, err := os.ReadDir(auditDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading audit directory: %w", err)
	}

	// Collect only directories, skip regular files.
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}

	if len(dirs) <= retention {
		return nil
	}

	sort.Strings(dirs)

	// Delete the oldest directories beyond the retention limit.
	toDelete := dirs[:len(dirs)-retention]
	for _, name := range toDelete {
		p := filepath.Join(auditDir, name)
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("deleting old audit run %q: %w", name, err)
		}
	}

	return nil
}
