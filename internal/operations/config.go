package operations

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Listen          string `json:"listen"`
	TLS             bool   `json:"tls"`
	BackupDirectory string `json:"backupDirectory"`
	BackupRetention int    `json:"backupRetention"`
}

func LoopbackListen(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func (c Config) Validate() error {
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("listen debe ser una dirección IP y puerto: %w", err)
	}
	if !c.TLS && !LoopbackListen(c.Listen) {
		return fmt.Errorf("el acceso por red local requiere TLS; HTTP solo se admite en 127.0.0.1 o ::1")
	}
	if c.BackupRetention < 1 || c.BackupRetention > 365 {
		return fmt.Errorf("backupRetention debe estar entre 1 y 365")
	}
	if strings.TrimSpace(c.BackupDirectory) == "" {
		return fmt.Errorf("falta backupDirectory")
	}
	return nil
}
func ReadConfig(dir string) (Config, error) {
	var c Config
	f, e := os.Open(filepath.Join(dir, "config.json"))
	if e != nil {
		return c, fmt.Errorf("ejecuta primero 'erp init': %w", e)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	return c, c.Validate()
}
func WriteConfig(dir string, c Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	data, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), append(data, '\n'), 0600)
}
func (c Config) BackupPath(dataDir string) (string, error) {
	root, e := filepath.Abs(dataDir)
	if e != nil {
		return "", e
	}
	path := c.BackupDirectory
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, e = filepath.Abs(path)
	if e != nil {
		return "", e
	}
	// On Windows an external backup disk has a different volume and Rel cannot
	// describe it; it is still a valid destination outside the data directory.
	if !strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(path)) {
		return path, nil
	}
	rel, e := filepath.Rel(root, path)
	if e != nil {
		return "", e
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
		return "", fmt.Errorf("el directorio de respaldos debe estar fuera del directorio de datos para permitir restauraciones seguras")
	}
	return path, nil
}
