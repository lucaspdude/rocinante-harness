package projects

// One-shot migration from ompweb's ~/.omp/agent/projects.json.
// Spec: docs/mvp/phase-1-functionality/06-migration.md.
//
// Runs from cmd/api/main.go BEFORE http.ListenAndServe(). A
// marker file <share-dir>/.migration-completed prevents re-runs.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MigrationResult summarizes what happened during migration.
type MigrationResult struct {
	Skipped         bool // true if .migration-completed existed
	Added           int
	SkippedExisting int
	Error           error
}

// DefaultOmpwebProjectsPath returns the ompweb projects.json path
// for the current OS / user. macOS / linux: ~/.omp/agent/projects.json.
func DefaultOmpwebProjectsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".omp", "agent", "projects.json"), nil
}

// MarkerPath returns the migration-completed marker file path.
func MarkerPath(shareDir string) string {
	return filepath.Join(shareDir, ".migration-completed")
}

// MigrateFromOmpweb imports ompweb projects into the registry and
// writes the marker file. If the marker exists already, the
// function reports Skipped=true and does nothing.
func MigrateFromOmpweb(reg *Registry, shareDir string) (MigrationResult, error) {
	res := MigrationResult{}
	marker := MarkerPath(shareDir)
	if _, err := os.Stat(marker); err == nil {
		res.Skipped = true
		return res, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		res.Error = err
		return res, err
	}

	src, err := DefaultOmpwebProjectsPath()
	if err != nil {
		res.Error = err
		return res, err
	}
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		if err := writeMarker(marker); err != nil {
			res.Error = err
			return res, err
		}
		res.Skipped = true
		return res, nil
	}
	added, skipped, err := reg.MigrateFromOmpweb(src)
	res.Added = added
	res.SkippedExisting = skipped
	if err != nil {
		res.Error = err
		return res, err
	}
	if err := writeMarker(marker); err != nil {
		res.Error = err
		return res, err
	}
	return res, nil
}

func writeMarker(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("marker mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	_, _ = f.WriteString("migrated at " + time.Now().UTC().Format("2006-01-02T15:04:05Z") + "\n")
	return f.Close()
}
