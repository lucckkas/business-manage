//go:build windows

package printing

import (
	"business-manage/internal/modules/pos"
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"strconv"
	"unsafe"
)

var spool = windows.NewLazySystemDLL("winspool.drv")
var enumPrinters = spool.NewProc("EnumPrintersW")
var openPrinter = spool.NewProc("OpenPrinterW")
var closePrinter = spool.NewProc("ClosePrinter")
var startDoc = spool.NewProc("StartDocPrinterW")
var startPage = spool.NewProc("StartPagePrinter")
var writePrinter = spool.NewProc("WritePrinter")
var endPage = spool.NewProc("EndPagePrinter")
var endDoc = spool.NewProc("EndDocPrinter")
var abortPrinter = spool.NewProc("AbortPrinter")

type printerInfo4 struct {
	Name       *uint16
	Server     *uint16
	Attributes uint32
}
type docInfo1 struct {
	Name     *uint16
	Output   *uint16
	Datatype *uint16
}

func (n *Native) Devices(ctx context.Context) ([]pos.Device, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var needed, count uint32
	ret, _, e := enumPrinters.Call(2, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if ret == 0 && e != windows.ERROR_INSUFFICIENT_BUFFER {
		return nil, fmt.Errorf("enumerar impresoras: %w", e)
	}
	out := []pos.Device{}
	if needed == 0 {
		return out, nil
	}
	if needed > 8<<20 {
		return nil, fmt.Errorf("lista de impresoras demasiado grande")
	}
	buffer := make([]byte, needed)
	ret, _, e = enumPrinters.Call(2, 0, 4, uintptr(unsafe.Pointer(&buffer[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if ret == 0 {
		return nil, fmt.Errorf("enumerar impresoras: %w", e)
	}
	size := unsafe.Sizeof(printerInfo4{})
	if uintptr(count)*size > uintptr(len(buffer)) {
		return nil, fmt.Errorf("respuesta de impresoras inválida")
	}
	for i := uint32(0); i < count; i++ {
		info := (*printerInfo4)(unsafe.Pointer(&buffer[uintptr(i)*size]))
		name := windows.UTF16PtrToString(info.Name)
		out = append(out, pos.Device{ID: name, Name: name, Transport: "windows-raw"})
	}
	runtime.KeepAlive(buffer)
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
		return "", &pos.SendError{Err: fmt.Errorf("impresora no detectada")}
	}
	name, e := windows.UTF16PtrFromString(id)
	if e != nil {
		return "", &pos.SendError{Err: e}
	}
	var handle windows.Handle
	ret, _, e := openPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0)
	runtime.KeepAlive(name)
	if ret == 0 {
		return "", &pos.SendError{Err: e}
	}
	defer closePrinter.Call(uintptr(handle))
	title, _ := windows.UTF16PtrFromString("Raíz ERP")
	raw, _ := windows.UTF16PtrFromString("RAW")
	doc := docInfo1{Name: title, Datatype: raw}
	job, _, e := startDoc.Call(uintptr(handle), 1, uintptr(unsafe.Pointer(&doc)))
	runtime.KeepAlive(doc)
	if job == 0 {
		return "", &pos.SendError{Err: e}
	}
	completed := false
	defer func() {
		if !completed {
			abortPrinter.Call(uintptr(handle))
		}
	}()
	if ret, _, e = startPage.Call(uintptr(handle)); ret == 0 {
		return "", &pos.SendError{Err: e}
	}
	written := 0
	for written < len(data) {
		if e = ctx.Err(); e != nil {
			return "", &pos.SendError{Err: e, Uncertain: written > 0}
		}
		var count uint32
		ret, _, e = writePrinter.Call(uintptr(handle), uintptr(unsafe.Pointer(&data[written])), uintptr(len(data)-written), uintptr(unsafe.Pointer(&count)))
		runtime.KeepAlive(data)
		if ret == 0 || count == 0 {
			return "", &pos.SendError{Err: fmt.Errorf("WritePrinter: %v", e), Uncertain: true}
		}
		written += int(count)
	}
	runtime.KeepAlive(data)
	if ret, _, e = endPage.Call(uintptr(handle)); ret == 0 {
		return "", &pos.SendError{Err: e, Uncertain: true}
	}
	if ret, _, e = endDoc.Call(uintptr(handle)); ret == 0 {
		return "", &pos.SendError{Err: e, Uncertain: true}
	}
	completed = true
	return strconv.FormatUint(uint64(job), 10), nil
}
