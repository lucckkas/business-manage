//go:build windows

package operations_test

import (
	"business-manage/internal/operations"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupOnAnotherWindowsVolume(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	volume := "Z:"
	if strings.EqualFold(filepath.VolumeName(root), volume) {
		volume = "Y:"
	}
	destination := volume + `\RaizBackups`
	config := operations.Config{BackupDirectory: destination}
	got, err := config.BackupPath(root)
	if err != nil || got != destination {
		t.Fatalf("external volume: %q, %v", got, err)
	}
}
