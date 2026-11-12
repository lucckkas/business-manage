package operations

import (
	"business-manage/internal/core"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type LogWriter struct {
	mu   sync.Mutex
	dir  string
	file *os.File
	size int64
	day  string
}

func NewLogWriter(dir string) (*LogWriter, error) {
	l := &LogWriter{dir: dir}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return l, nil
}
func (l *LogWriter) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	day := time.Now().UTC().Format("20060102")
	if l.file == nil || l.day != day || l.size+int64(len(data)) > 10<<20 {
		if l.file != nil {
			l.file.Close()
		}
		name := "erp-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + core.ID()[:8] + ".jsonl"
		f, err := os.OpenFile(filepath.Join(l.dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return 0, err
		}
		l.file = f
		l.size = 0
		l.day = day
		files, _ := filepath.Glob(filepath.Join(l.dir, "erp-*.jsonl"))
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
		for i := 10; i < len(files); i++ {
			_ = os.Remove(files[i])
		}
	}
	n, err := l.file.Write(data)
	l.size += int64(n)
	return n, err
}
func (l *LogWriter) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
