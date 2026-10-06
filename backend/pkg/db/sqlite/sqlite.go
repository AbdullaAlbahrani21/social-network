package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"social-network/backend/pkg/db/migrations"
)

// An fs.ReadFileFS rather than embed.FS, so a test can run the runner over a subset of the migrations.
var migrationFiles fs.ReadFileFS = migrations.Files

func Connect(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Closed on failure: the caller gets no handle to close, and an open one keeps the file locked.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	entries, err := fs.Glob(migrationFiles, "sqlite/*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries) // 000001_... sorts correctly as plain strings

	for _, filename := range entries {
		var alreadyApplied bool
		err := db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = ?)`,
			filename,
		).Scan(&alreadyApplied)
		if err != nil {
			return err
		}
		if alreadyApplied {
			continue
		}

		sqlBytes, err := migrationFiles.ReadFile(filename)
		if err != nil {
			return err
		}

		if err := applyMigration(db, filename, string(sqlBytes)); err != nil {
			return err
		}

		log.Printf("applied migration: %s", filename)
	}

	return nil
}

// Foreign keys are off while a migration runs -- a table rebuild's DROP would cascade -- and the pragma does nothing inside a transaction, so this runs on its own connection before BEGIN.
func applyMigration(db *sql.DB, filename, sqlText string) (err error) {
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("%s: disable foreign keys: %w", filename, err)
	}
	defer func() {
		if _, onErr := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); onErr != nil {
			conn.Raw(func(any) error { return driver.ErrBadConn })
			if err == nil {
				err = fmt.Errorf("%s: re-enable foreign keys: %w", filename, onErr)
			}
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range strings.Split(sqlText, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
	}

	if err := checkForeignKeys(ctx, tx); err != nil {
		return fmt.Errorf("%s: %w", filename, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES (?)`, filename); err != nil {
		return err
	}

	return tx.Commit()
}

func checkForeignKeys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var violations []string
	for rows.Next() {
		var table, parent string
		var rowID sql.NullInt64
		var constraint int64
		if err := rows.Scan(&table, &rowID, &parent, &constraint); err != nil {
			return err
		}
		violations = append(violations, fmt.Sprintf("%s row %d references a missing %s row", table, rowID.Int64, parent))
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(violations) > 0 {
		return fmt.Errorf("foreign key check failed: %s", strings.Join(violations, ", "))
	}
	return nil
}
