//go:build !windows

package platform

import (
	"context"
	"golang.org/x/sys/unix"
	"os"
)

func lockFile(f *os.File) error                                             { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func unlockFile(f *os.File)                                                 { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }
func RunService(ctx context.Context, run func(context.Context) error) error { return run(ctx) }
