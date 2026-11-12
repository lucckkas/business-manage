package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

type Lock struct{ file *os.File }

func Acquire(dataDir string) (*Lock, error) {
	path, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("otra instancia utiliza este directorio; detén el servicio antes de continuar: %w", err)
	}
	return &Lock{file: f}, nil
}
func (l *Lock) Close() error { unlockFile(l.file); return l.file.Close() }
