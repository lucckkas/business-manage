package operations

import (
	"archive/zip"
	"business-manage/internal/adapters/sqlite"
	"business-manage/internal/core"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Manifest struct {
	Format        int               `json:"format"`
	SchemaVersion int               `json:"schemaVersion"`
	CreatedAt     string            `json:"createdAt"`
	Files         map[string]string `json:"files"`
}

var backupFiles = []string{"config.json", "certificates/ca.crt", "certificates/ca.key", "certificates/server.crt", "certificates/server.key"}

func Backup(ctx context.Context, store *sqlite.Store, dataDir string, c Config) (string, error) {
	if _, err := store.Read().Admin.Company(ctx); err != nil {
		return "", fmt.Errorf("completa la configuración de la empresa antes de respaldar: %w", err)
	}
	destination, err := c.BackupPath(dataDir)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(destination, 0700); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(destination, ".snapshot-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	snapshot := filepath.Join(work, "erp.db")
	if err = store.Snapshot(ctx, snapshot); err != nil {
		return "", err
	}
	check, err := sqlite.Open(snapshot)
	if err != nil {
		return "", err
	}
	version, err := check.Version(ctx)
	if err == nil {
		err = check.Integrity(ctx)
	}
	closeErr := check.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	manifest := Manifest{Format: 1, SchemaVersion: version, CreatedAt: core.Now(), Files: map[string]string{}}
	name := "erp-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + core.ID()[:8] + ".zip"
	temporary := filepath.Join(work, name)
	f, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	archive := zip.NewWriter(f)
	files := append([]string{"erp.db"}, backupFiles...)
	for _, name := range files {
		source := filepath.Join(dataDir, filepath.FromSlash(name))
		if name == "erp.db" {
			source = snapshot
		}
		input, e := os.Open(source)
		if e != nil {
			archive.Close()
			return "", e
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		entry, e := archive.CreateHeader(header)
		if e != nil {
			input.Close()
			archive.Close()
			return "", e
		}
		hash := sha256.New()
		_, e = io.Copy(io.MultiWriter(entry, hash), input)
		input.Close()
		if e != nil {
			archive.Close()
			return "", e
		}
		manifest.Files[name] = hex.EncodeToString(hash.Sum(nil))
	}
	entry, err := archive.Create("manifest.json")
	if err == nil {
		err = json.NewEncoder(entry).Encode(manifest)
	}
	if err != nil {
		archive.Close()
		return "", err
	}
	if err = archive.Close(); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	result := filepath.Join(destination, name)
	if err = os.Rename(temporary, result); err != nil {
		return "", err
	}
	if err = Retain(destination, c.BackupRetention); err != nil {
		return result, err
	}
	return result, nil
}
func Retain(dir string, count int) error {
	files, err := filepath.Glob(filepath.Join(dir, "erp-*.zip"))
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for i := count; i < len(files); i++ {
		if err = os.Remove(files[i]); err != nil {
			return err
		}
	}
	return nil
}
func BackupDue(c Config, dataDir string) bool {
	path, err := c.BackupPath(dataDir)
	if err != nil {
		return true
	}
	files, _ := filepath.Glob(filepath.Join(path, "erp-*.zip"))
	var newest time.Time
	for _, name := range files {
		if stat, err := os.Stat(name); err == nil && stat.ModTime().After(newest) {
			newest = stat.ModTime()
		}
	}
	return time.Since(newest) >= 24*time.Hour
}

// Restore validates in a sibling directory and swaps the whole installation state.
// The caller MUST hold the installation lock and close all database handles.
// The previous directory is retained, so every successful replacement is reversible.
func Restore(ctx context.Context, archivePath, dataDir string) (string, error) {
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(filepath.Dir(root), ".erp-restore-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	allowed := map[string]bool{"erp.db": true, "manifest.json": true}
	for _, name := range backupFiles {
		allowed[name] = true
	}
	seen := map[string]bool{}
	hashes := map[string]string{}
	var manifest Manifest
	var total uint64
	for _, file := range archive.File {
		if !allowed[file.Name] || seen[file.Name] {
			return "", fmt.Errorf("entrada de respaldo no válida: %s", file.Name)
		}
		seen[file.Name] = true
		total += file.UncompressedSize64
		if file.UncompressedSize64 > 8<<30 || total > 9<<30 {
			return "", fmt.Errorf("el respaldo supera el máximo de 9 GiB")
		}
		if file.Name != "erp.db" && file.UncompressedSize64 > 1<<20 {
			return "", fmt.Errorf("metadatos de respaldo demasiado grandes")
		}
		input, e := file.Open()
		if e != nil {
			return "", e
		}
		if file.Name == "manifest.json" {
			e = json.NewDecoder(io.LimitReader(input, 1<<20)).Decode(&manifest)
			input.Close()
			if e != nil {
				return "", e
			}
			continue
		}
		path := filepath.Join(work, filepath.FromSlash(file.Name))
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			input.Close()
			return "", e
		}
		output, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			input.Close()
			return "", e
		}
		hash := sha256.New()
		_, e = io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, int64(file.UncompressedSize64)+1))
		input.Close()
		syncErr := output.Sync()
		output.Close()
		if e != nil {
			return "", e
		}
		if syncErr != nil {
			return "", syncErr
		}
		hashes[file.Name] = hex.EncodeToString(hash.Sum(nil))
	}
	if len(seen) != len(allowed) || manifest.Format != 1 || manifest.SchemaVersion != sqlite.SchemaVersion || len(manifest.Files) != len(hashes) {
		return "", fmt.Errorf("respaldo incompleto o versión de esquema incompatible")
	}
	for name, hash := range hashes {
		if manifest.Files[name] != hash {
			return "", fmt.Errorf("checksum no válido: %s", name)
		}
	}
	c, err := ReadConfig(work)
	if err != nil {
		return "", err
	}
	if _, err = c.BackupPath(root); err != nil {
		return "", err
	}
	if _, err = tls.LoadX509KeyPair(filepath.Join(work, "certificates/server.crt"), filepath.Join(work, "certificates/server.key")); err != nil {
		return "", fmt.Errorf("certificados de respaldo inválidos: %w", err)
	}
	check, err := sqlite.Open(filepath.Join(work, "erp.db"))
	if err != nil {
		return "", err
	}
	version, e := check.Version(ctx)
	if e == nil && version != manifest.SchemaVersion {
		e = fmt.Errorf("la versión de la base no coincide con el manifiesto")
	}
	if e == nil {
		e = check.Integrity(ctx)
	}
	if e == nil {
		_, e = check.Read().Admin.Company(ctx)
	}
	if e == nil {
		_, e = check.DB.ExecContext(ctx, "DELETE FROM admin_sessions")
	}
	if e == nil {
		_, e = check.DB.ExecContext(ctx, "UPDATE pos_jobs SET state='suspended',error='Trabajo suspendido tras restaurar un respaldo.',updated_at=? WHERE state IN ('pending','sending')", core.Now())
	}
	if e == nil {
		_, e = check.DB.ExecContext(ctx, "UPDATE pos_settings SET config_json=json_set(config_json,'$.enabled',json('false'),'$.drawerEnabled',json('false'),'$.printVerified',json('false'),'$.drawerVerified',json('false'),'$.version',COALESCE(json_extract(config_json,'$.version'),0)+1)")
	}
	closeErr := check.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	if _, err = os.Stat(root); err == nil {
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			return "", readErr
		}
		if len(entries) > 0 {
			_, configErr := os.Stat(filepath.Join(root, "config.json"))
			_, databaseErr := os.Stat(filepath.Join(root, "erp.db"))
			_, certificateErr := os.Stat(filepath.Join(root, "certificates"))
			if configErr != nil || (databaseErr != nil && certificateErr != nil) {
				return "", fmt.Errorf("el destino contiene archivos ajenos a una instalación ERP; selecciona un directorio vacío o una instalación existente")
			}
		}
		previous := root + ".previous-" + time.Now().UTC().Format("20060102T150405") + "-" + core.ID()[:8]
		if err = os.Rename(root, previous); err != nil {
			return "", err
		}
		if err = os.Rename(work, root); err != nil {
			rollbackErr := os.Rename(previous, root)
			if rollbackErr != nil {
				return previous, fmt.Errorf("restauración: %v; recuperación del directorio anterior: %w", err, rollbackErr)
			}
			return "", err
		}
		return previous, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err = os.Rename(work, root); err != nil {
		return "", err
	}
	return "", nil
}
