package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/pos"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"
)

func (s *Service) PrinterDevices(ctx context.Context) ([]pos.Device, error) {
	if s.Printer == nil {
		return []pos.Device{}, nil
	}
	return s.Printer.Devices(ctx)
}
func (s *Service) SavePOSSettings(ctx context.Context, actor admin.User, next pos.Settings) (pos.Settings, error) {
	if actor.Role != "admin" {
		return next, core.Forbidden
	}
	if e := next.Validate(); e != nil {
		return next, e
	}
	previous, e := s.Store.Read().POS.Settings(ctx)
	if e != nil {
		return next, e
	}
	detectedName := ""
	if next.DeviceID != "" && next.DeviceID != previous.DeviceID {
		devices, e := s.PrinterDevices(ctx)
		if e != nil {
			return next, e
		}
		for _, d := range devices {
			if d.ID == next.DeviceID {
				detectedName = d.Name
			}
		}
		if detectedName == "" {
			return next, core.Invalid("Selecciona una impresora detectada en el servidor.")
		}
	}
	e = s.Store.WithinTx(ctx, func(p Ports) error {
		old, e := p.POS.Settings(ctx)
		if e != nil {
			return e
		}
		if old.Version != next.Version {
			return core.Conflict("La configuración cambió. Recarga sus datos.")
		}
		if next.DeviceID != "" {
			if next.DeviceID != old.DeviceID {
				next.DeviceName = detectedName
			} else {
				next.DeviceName = old.DeviceName
			}
		} else {
			next.DeviceName = ""
		}
		same := old.PhysicalKey() == next.PhysicalKey()
		next.PrintVerified = same && old.PrintVerified
		next.DrawerVerified = same && old.DrawerVerified
		if next.Enabled && (!next.PrintVerified || next.DeviceID == "") {
			return core.Invalid("Guarda la configuración y verifica un ticket de prueba antes de activar la impresión.")
		}
		if next.DrawerEnabled && (!next.Enabled || !next.DrawerVerified) {
			return core.Invalid("Verifica la apertura del cajón antes de activarla automáticamente.")
		}
		expected := next.Version
		next.Version++
		if e = p.POS.SaveSettings(ctx, next, expected); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "pos.settings", "printer")
		entry.Details = map[string]any{"before": old, "after": next}
		return p.Audit.Append(ctx, entry)
	})
	return next, e
}
func newPrintJob(actor, kind, key, hash string, cfg pos.Settings, payload []byte, drawer bool, reason string) pos.Job {
	return pos.Job{ID: core.ID(), Kind: kind, State: "pending", ActorID: actor, CreatedAt: core.Now(), UpdatedAt: core.Now(), Destination: cfg, PrinterName: cfg.DeviceName, Payload: payload, OpensDrawer: drawer, Reason: reason, Key: key, RequestHash: hash}
}
func logPrintJob(ctx context.Context, p Ports, j pos.Job, action string) error {
	entry := auditEntry(j.ActorID, action, j.ID)
	entry.Details = map[string]any{"jobId": j.ID, "saleId": j.SaleID, "cashMovementId": j.CashMovementID, "kind": j.Kind, "state": j.State, "printer": j.PrinterName, "opensDrawer": j.OpensDrawer, "reason": j.Reason, "error": j.Error, "spoolId": j.SpoolID}
	return p.Audit.Append(ctx, entry)
}
func (s *Service) RequestPrint(ctx context.Context, actor admin.User, key, kind, saleID, reason string) (pos.Job, bool, error) {
	var result pos.Job
	replayed := false
	if len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key {
		return result, false, core.Invalid("Incluye una clave de idempotencia de 16 a 128 caracteres.")
	}
	reason = strings.TrimSpace(reason)
	switch kind {
	case "copy":
		if reason == "" {
			reason = "Reimpresión solicitada"
		}
	case "test_print", "test_drawer", "drawer":
		if actor.Role != "admin" {
			return result, false, core.Forbidden
		}
	default:
		return result, false, core.Invalid("Tipo de trabajo no válido.")
	}
	if len(reason) > 240 || (kind == "drawer" && len(reason) < 3) {
		return result, false, core.Invalid("Indica un motivo de 3 a 240 caracteres.")
	}
	raw, _ := json.Marshal([]string{actor.ID, kind, saleID, reason})
	hash := digest(string(raw))
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		prior, e := p.POS.JobByKey(ctx, "manual:"+key)
		if e == nil {
			if prior.RequestHash != hash {
				return core.Conflict("La clave corresponde a otra solicitud.")
			}
			result = prior
			replayed = true
			return nil
		}
		if !core.HasCode(e, "not_found") {
			return e
		}
		cfg, e := p.POS.Settings(ctx)
		if e != nil {
			return e
		}
		if cfg.DeviceID == "" {
			return core.Invalid("Configura una impresora en Ajustes.")
		}
		var payload []byte
		drawer := false
		switch kind {
		case "copy":
			if !cfg.PrintVerified {
				return core.Invalid("Verifica la impresora antes de solicitar tickets.")
			}
			sale, e := p.Sales.Sale(ctx, saleID)
			if e != nil {
				return e
			}
			payload = pos.Receipt(sale, cfg, true)
		case "test_print":
			payload = pos.TestTicket(cfg)
		case "test_drawer", "drawer":
			if !cfg.PrintVerified {
				return core.Invalid("Primero verifica un ticket de prueba.")
			}
			if kind == "drawer" && !cfg.DrawerVerified {
				return core.Invalid("Primero verifica el cajón en Ajustes.")
			}
			payload = pos.Drawer(cfg)
			drawer = true
		}
		result = newPrintJob(actor.ID, kind, "manual:"+key, hash, cfg, payload, drawer, reason)
		result.SaleID = saleID
		if e = p.POS.Enqueue(ctx, result); e != nil {
			return e
		}
		return logPrintJob(ctx, p, result, "pos.enqueue")
	})
	return result, replayed, e
}
func (s *Service) VerifyPOSTest(ctx context.Context, actor admin.User, id string) (pos.Settings, error) {
	var cfg pos.Settings
	if actor.Role != "admin" {
		return cfg, core.Forbidden
	}
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		var e error
		cfg, e = p.POS.Settings(ctx)
		if e != nil {
			return e
		}
		j, e := p.POS.Job(ctx, id)
		if e != nil {
			return e
		}
		if j.State != "sent" || j.Destination.PhysicalKey() != cfg.PhysicalKey() || j.Destination.Version != cfg.Version {
			return core.Conflict("La prueba debe estar enviada y corresponder a la configuración actual.")
		}
		if j.Kind == "test_print" {
			cfg.PrintVerified = true
		} else if j.Kind == "test_drawer" {
			cfg.DrawerVerified = true
		} else {
			return core.Invalid("Este trabajo no es una prueba de configuración.")
		}
		expected := cfg.Version
		cfg.Version++
		if e = p.POS.SaveSettings(ctx, cfg, expected); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "pos.settings", "printer")
		entry.Details = map[string]any{"verifiedJobId": id, "after": cfg}
		return p.Audit.Append(ctx, entry)
	})
	return cfg, e
}
func (s *Service) RecoverPrintJobs(ctx context.Context) error {
	return s.Store.WithinTx(ctx, func(p Ports) error {
		jobs, e := p.POS.Interrupted(ctx)
		if e != nil {
			return e
		}
		for _, j := range jobs {
			j.State = "uncertain"
			j.Error = "El servidor se detuvo durante el envío. Revisa el papel antes de solicitar una copia."
			if j.OpensDrawer {
				j.Error = "El servidor se detuvo durante el envío. Comprueba el cajón antes de solicitar otra apertura."
			}
			if e = p.POS.Finish(ctx, j.ID, j.State, j.Error, ""); e != nil {
				return e
			}
			if e = logPrintJob(ctx, p, j, "pos.recover"); e != nil {
				return e
			}
		}
		return nil
	})
}

// ProcessPrintJob claims work transactionally, releases SQLite, then contacts the
// printer. Any possibly partial output is terminal until an explicit copy.
func (s *Service) ProcessPrintJob(ctx context.Context) (bool, error) {
	var j *pos.Job
	e := s.Store.WithinTx(ctx, func(p Ports) error { var e error; j, e = p.POS.Claim(ctx); return e })
	if e != nil || j == nil {
		return false, e
	}
	state, message, spool := "sent", "", ""
	cfg, e := s.Store.Read().POS.Settings(ctx)
	if e != nil {
		state = "error"
		message = "No se pudo consultar la configuración. No se envió el trabajo."
	} else if j.Kind == "sale" && !cfg.Enabled {
		state = "suspended"
		message = "La impresión automática está desactivada."
	} else if j.Kind == "expense_drawer" && (!cfg.Enabled || !cfg.DrawerEnabled || !cfg.DrawerVerified || !cfg.PrintVerified || cfg.PhysicalKey() != j.Destination.PhysicalKey()) {
		state = "suspended"
		message = "La apertura automática está desactivada o cambió el cajón configurado. El egreso sigue registrado."
	} else {
		if s.Printer == nil {
			e = errors.New("transporte de impresora no disponible")
		} else {
			sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			spool, e = s.Printer.Send(sendCtx, j.Destination.DeviceID, j.Payload)
			cancel()
		}
		if e != nil {
			if s.Log != nil {
				s.Log.Warn("printer_delivery_failed", "jobId", j.ID, "error", e)
			}
			state = "error"
			message = "No se pudo enviar el trabajo. Revisa la conexión y los permisos de la impresora."
			var delivery *pos.SendError
			if errors.As(e, &delivery) && delivery.Uncertain {
				state = "uncertain"
				message = "El envío pudo haberse realizado. Revisa el papel antes de solicitar una copia."
				if j.OpensDrawer {
					message = "El envío pudo haberse realizado. Comprueba el cajón antes de solicitar otra apertura."
				}
			}
		}
	}
	j.State = state
	j.Error = message
	j.SpoolID = spool
	finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e = s.Store.WithinTx(finishCtx, func(p Ports) error {
		if e := p.POS.Finish(finishCtx, j.ID, state, message, spool); e != nil {
			return e
		}
		return logPrintJob(finishCtx, p, *j, "pos.result")
	})
	return true, e
}
func (s *Service) RunPrinter(ctx context.Context, logger *slog.Logger) {
	for ctx.Err() == nil {
		worked, e := s.ProcessPrintJob(ctx)
		if e != nil {
			logger.Error("printer_job_failed", "error", e)
		}
		if worked && e == nil {
			continue
		}
		timer := time.NewTimer(400 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
