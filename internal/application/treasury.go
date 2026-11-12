package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/treasury"
	"context"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (s *Service) RecordCashExpense(ctx context.Context, actor admin.User, key string, r treasury.ExpenseRequest) (treasury.Movement, bool, error) {
	var result treasury.Movement
	if len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key {
		return result, false, core.Invalid("Incluye una clave de idempotencia de 16 a 128 caracteres.")
	}
	n, e := core.Minor(r.AmountMinor)
	if e != nil {
		return result, false, e
	}
	if n <= 0 {
		return result, false, core.Invalid("El monto del egreso debe ser mayor que cero.")
	}
	r.AmountMinor = strconv.FormatInt(n, 10)
	r.Reason = strings.TrimSpace(r.Reason)
	if utf8.RuneCountInString(r.Reason) < 3 || utf8.RuneCountInString(r.Reason) > 240 {
		return result, false, core.Invalid("Indica un motivo de 3 a 240 caracteres.")
	}
	raw, _ := json.Marshal(r)
	hash := digest(actor.ID + ":" + string(raw))
	replayed := false
	e = s.Store.WithinTx(ctx, func(p Ports) error {
		prior, e := p.Treasury.ExpenseByKey(ctx, key)
		if e == nil {
			if prior.RequestHash != hash {
				return core.Conflict("La clave de idempotencia ya corresponde a otro egreso.")
			}
			result, replayed = prior, true
			return nil
		}
		if !core.HasCode(e, "not_found") {
			return e
		}
		current, e := p.Admin.User(ctx, actor.ID)
		if e != nil {
			return e
		}
		if !current.Active {
			return core.Unauthorized
		}
		if !current.CanExpense() {
			return core.Forbidden
		}
		before, e := p.Treasury.Balance(ctx)
		if e != nil {
			return e
		}
		balance, ok := new(big.Int).SetString(before, 10)
		if !ok {
			return core.Invalid("No se pudo calcular el saldo de caja.")
		}
		after := balance.Sub(balance, big.NewInt(n)).String()
		cfg, e := p.POS.Settings(ctx)
		if e != nil {
			return e
		}
		result = treasury.Movement{ID: core.ID(), Kind: "expense", AmountMinor: "-" + r.AmountMinor, ActorID: current.ID, ActorName: current.Name, Reason: r.Reason, CreatedAt: core.Now(), BalanceBefore: &before, BalanceAfter: &after, IdempotencyKey: key, RequestHash: hash}
		if cfg.Enabled && cfg.DrawerEnabled && cfg.PrintVerified && cfg.DrawerVerified && cfg.DeviceID != "" {
			result.DrawerJobID = core.ID()
		}
		if e = p.Treasury.CreateExpense(ctx, &result); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "cash.expense", result.ID)
		entry.Details = map[string]any{"amountMinor": result.AmountMinor, "reason": result.Reason, "balanceBefore": before, "balanceAfter": after, "drawerJobId": result.DrawerJobID}
		if e = p.Audit.Append(ctx, entry); e != nil {
			return e
		}
		if result.DrawerJobID != "" {
			job := newPrintJob(actor.ID, "expense_drawer", "expense:"+result.ID, hash, cfg, pos.Drawer(cfg), true, result.Reason)
			job.ID, job.CashMovementID = result.DrawerJobID, result.ID
			if e = p.POS.Enqueue(ctx, job); e != nil {
				return e
			}
			return logPrintJob(ctx, p, job, "pos.enqueue")
		}
		return nil
	})
	return result, replayed, e
}
