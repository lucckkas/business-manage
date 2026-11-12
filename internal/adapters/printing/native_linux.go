//go:build linux

package printing

import (
	"business-manage/internal/modules/pos"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func (n *Native) Devices(ctx context.Context) ([]pos.Device, error) {
	out := []pos.Device{}
	seen := map[string]bool{}
	for _, pattern := range []string{"/dev/erp-printer-*", "/dev/usb/lp*"} {
		paths, _ := filepath.Glob(pattern)
		for _, p := range paths {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			real, e := filepath.EvalSymlinks(p)
			if e != nil || !strings.HasPrefix(real, "/dev/usb/lp") {
				continue
			}
			info, e := os.Stat(real)
			if e != nil || info.Mode()&os.ModeCharDevice == 0 || seen[real] {
				continue
			}
			seen[real] = true
			out = append(out, pos.Device{ID: p, Name: filepath.Base(p) + " · USB del servidor", Transport: "linux-usb"})
		}
	}
	return out, nil
}
func (n *Native) Send(ctx context.Context, id string, data []byte) (string, error) {
	devices, e := n.Devices(ctx)
	if e != nil {
		return "", &pos.SendError{Err: e}
	}
	found := false
	for _, d := range devices {
		if d.ID == id {
			found = true
		}
	}
	if !found {
		return "", &pos.SendError{Err: fmt.Errorf("dispositivo USB no detectado")}
	}
	file, e := os.OpenFile(id, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if e != nil {
		return "", &pos.SendError{Err: e}
	}
	defer file.Close()
	written := 0
	for written < len(data) {
		if e = ctx.Err(); e != nil {
			return "", &pos.SendError{Err: e, Uncertain: written > 0}
		}
		count, err := file.Write(data[written:])
		written += count
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			timer := time.NewTimer(25 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", &pos.SendError{Err: ctx.Err(), Uncertain: written > 0}
			case <-timer.C:
			}
			continue
		}
		if err != nil {
			return "", &pos.SendError{Err: err, Uncertain: true}
		}
		if count == 0 {
			return "", &pos.SendError{Err: fmt.Errorf("escritura USB sin progreso"), Uncertain: written > 0}
		}
	}
	return "usb", nil
}
