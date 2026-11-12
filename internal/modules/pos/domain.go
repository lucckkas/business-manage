// Package pos owns persistent, auditable printer and drawer work, independent of
// the OS transport. A sent job means delivery to a device/spooler, not paper.
package pos

import (
	"business-manage/internal/core"
	"context"
	"fmt"
)

type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Transport string `json:"transport"`
}
type Settings struct {
	Version        int    `json:"version"`
	DeviceID       string `json:"deviceId"`
	DeviceName     string `json:"deviceName"`
	Width          int    `json:"width"`
	Cutter         bool   `json:"cutter"`
	DrawerPin      int    `json:"drawerPin"`
	PulseOnMS      int    `json:"pulseOnMs"`
	PulseOffMS     int    `json:"pulseOffMs"`
	Enabled        bool   `json:"enabled"`
	DrawerEnabled  bool   `json:"drawerEnabled"`
	PrintVerified  bool   `json:"printVerified"`
	DrawerVerified bool   `json:"drawerVerified"`
}

func Defaults() Settings { return Settings{Width: 80, PulseOnMS: 100, PulseOffMS: 500} }
func (s Settings) Validate() error {
	if s.Width != 58 && s.Width != 80 {
		return core.Invalid("Selecciona papel de 58 u 80 mm.")
	}
	if s.DrawerPin != 0 && s.DrawerPin != 1 {
		return core.Invalid("Selecciona el pin 2 o 5 del cajón.")
	}
	if s.PulseOnMS < 2 || s.PulseOnMS > 510 || s.PulseOffMS < s.PulseOnMS || s.PulseOffMS > 510 || s.PulseOnMS%2 != 0 || s.PulseOffMS%2 != 0 {
		return core.Invalid("Los tiempos deben ser pares entre 2 y 510 ms; el reposo debe ser igual o mayor al pulso.")
	}
	return nil
}
func (s Settings) PhysicalKey() string {
	return fmt.Sprintf("%s|%d|%t|%d|%d|%d", s.DeviceID, s.Width, s.Cutter, s.DrawerPin, s.PulseOnMS, s.PulseOffMS)
}

type Job struct {
	ID             string   `json:"id"`
	SaleID         string   `json:"saleId,omitempty"`
	SaleNumber     int64    `json:"saleNumber,omitempty"`
	CashMovementID string   `json:"cashMovementId,omitempty"`
	Kind           string   `json:"kind"`
	State          string   `json:"state"`
	ActorID        string   `json:"actorId"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	OpensDrawer    bool     `json:"opensDrawer"`
	Reason         string   `json:"reason"`
	Error          string   `json:"error"`
	SpoolID        string   `json:"spoolId"`
	PrinterName    string   `json:"printerName"`
	Key            string   `json:"-"`
	RequestHash    string   `json:"-"`
	Destination    Settings `json:"-"`
	Payload        []byte   `json:"-"`
}
type Repository interface {
	Settings(context.Context) (Settings, error)
	SaveSettings(context.Context, Settings, int) error
	Enqueue(context.Context, Job) error
	Job(context.Context, string) (Job, error)
	JobByKey(context.Context, string) (Job, error)
	Jobs(context.Context, core.Query, string) (core.Page[Job], error)
	Claim(context.Context) (*Job, error)
	Finish(context.Context, string, string, string, string) error
	Interrupted(context.Context) ([]Job, error)
}
type Transport interface {
	Devices(context.Context) ([]Device, error)
	Send(context.Context, string, []byte) (string, error)
}
type SendError struct {
	Err       error
	Uncertain bool
}

func (j Job) Description() string {
	kinds := map[string]string{"sale": "Ticket de venta", "copy": "Copia de ticket", "test_print": "Prueba de impresión", "test_drawer": "Prueba del cajón", "drawer": "Apertura manual", "expense_drawer": "Apertura por egreso"}
	states := map[string]string{"pending": "Pendiente", "sending": "Enviando", "sent": "Enviado", "error": "Error", "uncertain": "Resultado incierto", "suspended": "Suspendido"}
	return kinds[j.Kind] + " · " + states[j.State]
}

func (e *SendError) Error() string { return e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }
