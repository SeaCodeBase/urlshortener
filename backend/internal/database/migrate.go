package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// initMigration is the baseline schema. On databases created before the
// runner existed (or via docker-entrypoint-initdb.d) it is recorded as applied
// without being executed.
const initMigration = "001_init.sql"

// Migrate applies every *.sql file in dir that is not yet recorded in
// schema_migrations, in lexical order, and returns the versions it executed.
//
// DDL is not transactional in MariaDB, so files must be idempotent
// (CREATE TABLE IF NOT EXISTS, ADD COLUMN IF NOT EXISTS, ...): a file that
// fails half-way is simply re-run on the next start.
func Migrate(ctx context.Context, db *sqlx.DB, dir string) ([]string, error) {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(64) PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("list migrations in %s: %w", dir, err)
	}
	sort.Strings(files)

	var recorded []string
	if err := db.SelectContext(ctx, &recorded, `SELECT version FROM schema_migrations`); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	done := make(map[string]bool, len(recorded))
	for _, v := range recorded {
		done[v] = true
	}

	applied := []string{}
	for _, path := range files {
		version := filepath.Base(path)
		if done[version] {
			continue
		}

		if version == initMigration {
			exists, err := tableExists(ctx, db, "links")
			if err != nil {
				return applied, err
			}
			if exists {
				if err := record(ctx, db, version); err != nil {
					return applied, err
				}
				logger.Info(ctx, "migration recorded as already applied", zap.String("version", version))
				continue
			}
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return applied, fmt.Errorf("read migration %s: %w", version, err)
		}
		for i, stmt := range SplitStatements(string(raw)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return applied, fmt.Errorf("migration %s statement %d: %w", version, i+1, err)
			}
		}
		if err := record(ctx, db, version); err != nil {
			return applied, err
		}
		logger.Info(ctx, "migration applied", zap.String("version", version))
		applied = append(applied, version)
	}
	return applied, nil
}

// SplitStatements splits a migration file into statements. A statement ends
// at a line whose last non-blank character is ';'. Lines that start with
// "--" are dropped; inline comments are left for the server to ignore.
func SplitStatements(sql string) []string {
	var stmts []string
	var cur []string
	flush := func() {
		s := strings.TrimSpace(strings.Join(cur, "\n"))
		s = strings.TrimSpace(strings.TrimSuffix(s, ";"))
		if s != "" {
			stmts = append(stmts, s)
		}
		cur = cur[:0]
	}
	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && len(cur) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cur = append(cur, strings.TrimRight(line, " \t\r"))
		if strings.HasSuffix(trimmed, ";") {
			flush()
		}
	}
	flush()
	return stmts
}

func tableExists(ctx context.Context, db *sqlx.DB, table string) (bool, error) {
	var n int
	err := db.GetContext(ctx, &n,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table)
	if err != nil {
		return false, fmt.Errorf("check table %s: %w", table, err)
	}
	return n > 0, nil
}

func record(ctx context.Context, db *sqlx.DB, version string) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	return nil
}
