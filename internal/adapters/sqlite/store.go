// Package sqlite implements module repositories. SQL never escapes this adapter.
package sqlite

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	modernsqlite "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

const SchemaVersion = 4

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type Store struct {
	DB   *sql.DB
	Path string
}
type repos struct{ db executor }

func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	params := url.Values{}
	for _, p := range []string{"foreign_keys(1)", "synchronous(FULL)", "busy_timeout(5000)"} {
		params.Add("_pragma", p)
	}
	params.Set("_txlock", "immediate")
	dsn := (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: params.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if err = os.Chmod(abs, 0600); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db, Path: abs}, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Version(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "PRAGMA user_version").Scan(&n)
	return n, err
}
func (s *Store) Migrate(ctx context.Context, before func() error) error {
	version, err := s.Version(ctx)
	if err != nil {
		return err
	}
	if version > SchemaVersion {
		return fmt.Errorf("base de datos versión %d; este ejecutable admite hasta %d", version, SchemaVersion)
	}
	if version == SchemaVersion {
		return nil
	}
	var tables int
	if err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		return err
	}
	if tables > 0 && before != nil {
		if err = before(); err != nil {
			return fmt.Errorf("respaldo previo a migración: %w", err)
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	applied := version
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		next, e := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if e != nil {
			return fmt.Errorf("nombre de migración inválido: %s", entry.Name())
		}
		if next <= version || next > SchemaVersion {
			continue
		}
		if next != applied+1 {
			return fmt.Errorf("se esperaba la migración %d", applied+1)
		}
		data, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("migración %d: %w", next, err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?,?)", next, core.Now()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", next)); err != nil {
			return err
		}
		applied = next
	}
	if applied != SchemaVersion {
		return fmt.Errorf("faltan migraciones hasta la versión %d", SchemaVersion)
	}
	return tx.Commit()
}
func ports(db executor) application.Ports {
	r := &repos{db: db}
	return application.Ports{Admin: r, Catalog: r, Inventory: r, Sales: r, Treasury: r, Audit: r, Reports: r, Promotions: r, POS: r}
}
func (s *Store) ReadSnapshot(ctx context.Context, fn func(application.Ports) error) error {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN DEFERRED"); err != nil {
		return translate(err)
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	if err = fn(ports(conn)); err != nil {
		return translate(err)
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return translate(err)
}
func (s *Store) Read() application.Ports { return ports(s.DB) }
func (s *Store) WithinTx(ctx context.Context, fn func(application.Ports) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return translate(err)
	}
	defer tx.Rollback()
	if err = fn(ports(tx)); err != nil {
		return translate(err)
	}
	return translate(tx.Commit())
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return core.NotFound
	}
	var se *modernsqlite.Error
	if errors.As(err, &se) {
		switch se.Code() & 0xff {
		case 19:
			return core.Conflict("El registro ya existe o no cumple las restricciones de integridad.")
		case 5, 6:
			return &core.Error{Code: "busy", Message: "El servidor está ocupado. Vuelve a intentarlo conservando la misma operación."}
		}
	}
	return err
}
func like(search string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(search) + "%"
}
func (s *Store) Integrity(ctx context.Context) error {
	var result string
	if err := s.DB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integridad: %s", result)
	}
	rows, err := s.DB.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("la base contiene referencias inválidas")
	}
	return rows.Err()
}

// Snapshot uses SQLite's online backup API, which includes committed WAL content.
func (s *Store) Snapshot(ctx context.Context, destination string) error {
	connection, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	return connection.Raw(func(driverConn any) (result error) {
		backupSource, ok := driverConn.(interface {
			NewBackup(string) (*modernsqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("el controlador no admite respaldos en línea")
		}
		backup, err := backupSource.NewBackup(destination)
		if err != nil {
			return err
		}
		defer func() {
			if err := backup.Finish(); result == nil {
				result = err
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			more, err := backup.Step(128)
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	})
}
