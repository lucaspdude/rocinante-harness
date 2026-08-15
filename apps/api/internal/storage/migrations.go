package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

// migrationsFS embeds the migrations directory so the binary is
// self-contained (no external file dependency).
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// ApplyMigrations runs the embedded migrations in lexicographic
// order. The schema_version table records the highest version that
// has been applied; re-running is a no-op.
func ApplyMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version(version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("schema_version: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version := extractVersion(name)
		if version == 0 {
			continue
		}
		already, err := isVersionApplied(db, version)
		if err != nil {
			return err
		}
		if already {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		stmt := string(body)
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_version(version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("record version %d: %w", version, err)
		}
	}
	return nil
}

func isVersionApplied(db *sql.DB, version int) (bool, error) {
	row := db.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = ?`, version)
	var c int
	if err := row.Scan(&c); err != nil {
		return false, fmt.Errorf("query schema_version: %w", err)
	}
	return c > 0, nil
}

func extractVersion(name string) int {
	base := strings.SplitN(name, "_", 2)[0]
	var n int
	for _, ch := range base {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
