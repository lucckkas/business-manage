//go:build !linux && !windows

package printing

import (
	"business-manage/internal/modules/pos"
	"context"
	"fmt"
)

func (n *Native) Devices(context.Context) ([]pos.Device, error) { return []pos.Device{}, nil }
func (n *Native) Send(context.Context, string, []byte) (string, error) {
	return "", &pos.SendError{Err: fmt.Errorf("impresión nativa disponible en Windows y Linux")}
}
