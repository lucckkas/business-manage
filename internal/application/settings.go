package application

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) UpdateProfile(ctx context.Context, actor admin.User, name string) (admin.User, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 120 {
		return actor, core.Invalid("Indica un nombre de 2 a 120 caracteres.")
	}
	var result admin.User
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		old, e := p.Admin.User(ctx, actor.ID)
		if e != nil {
			return e
		}
		if e := p.Admin.UpdateProfile(ctx, actor.ID, name); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "user.profile", actor.ID)
		entry.Details = map[string]any{"before": map[string]any{"name": old.Name}, "after": map[string]any{"name": name}}
		if e := p.Audit.Append(ctx, entry); e != nil {
			return e
		}
		result, e = p.Admin.User(ctx, actor.ID)
		return e
	})
	return result, e
}
func (s *Service) ChangePassword(ctx context.Context, actor admin.User, current, next string, allowWeak ...bool) error {
	if e := validatePassword(next, allowWeak...); e != nil {
		return e
	}
	if utf8.RuneCountInString(current) > 128 {
		return core.Invalid("La contraseña actual no es correcta.")
	}
	user, e := s.Store.Read().Admin.User(ctx, actor.ID)
	if e != nil {
		return e
	}
	if !checkPassword(current, user.PasswordHash) {
		return core.Invalid("La contraseña actual no es correcta.")
	}
	hash := hashPassword(next)
	return s.Store.WithinTx(ctx, func(p Ports) error {
		fresh, e := p.Admin.User(ctx, actor.ID)
		if e != nil {
			return e
		}
		if fresh.PasswordHash != user.PasswordHash {
			return core.Conflict("La contraseña cambió durante la operación. Inténtalo nuevamente.")
		}
		if e = p.Admin.UpdatePassword(ctx, actor.ID, hash); e != nil {
			return e
		}
		if e = p.Admin.DeleteUserSessions(ctx, actor.ID); e != nil {
			return e
		}
		return p.Audit.Append(ctx, auditEntry(actor.ID, "user.password_change", actor.ID))
	})
}
func (s *Service) UpdateCompany(ctx context.Context, actor admin.User, name, timezone string) (admin.Company, error) {
	var company admin.Company
	if actor.Role != "admin" {
		return company, core.Forbidden
	}
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 160 {
		return company, core.Invalid("Indica un nombre de empresa de 2 a 160 caracteres.")
	}
	if _, e := time.LoadLocation(timezone); e != nil {
		return company, core.Invalid("Zona horaria inválida.")
	}
	e := s.Store.WithinTx(ctx, func(p Ports) error {
		var e error
		company, e = p.Admin.Company(ctx)
		if e != nil {
			return e
		}
		before := company
		company.Name = name
		company.Timezone = timezone
		if e = p.Admin.UpdateCompany(ctx, company); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "company.update", "company")
		entry.Details = map[string]any{"before": before, "after": company}
		return p.Audit.Append(ctx, entry)
	})
	return company, e
}
func (s *Service) SetDiscountPermission(ctx context.Context, actor admin.User, id string, enabled bool) error {
	if actor.Role != "admin" {
		return core.Forbidden
	}
	return s.Store.WithinTx(ctx, func(p Ports) error {
		target, e := p.Admin.User(ctx, id)
		if e != nil {
			return e
		}
		if target.Role == "admin" && !enabled {
			return core.Invalid("Los administradores siempre pueden aplicar descuentos manuales.")
		}
		if e = p.Admin.SetDiscountPermission(ctx, id, enabled); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "user.discount_permission", id)
		entry.Details = map[string]any{"before": target.CanDiscount(), "after": enabled}
		return p.Audit.Append(ctx, entry)
	})
}

func (s *Service) SetExpensePermission(ctx context.Context, actor admin.User, id string, enabled bool) error {
	if actor.Role != "admin" {
		return core.Forbidden
	}
	return s.Store.WithinTx(ctx, func(p Ports) error {
		target, e := p.Admin.User(ctx, id)
		if e != nil {
			return e
		}
		if target.Role == "admin" && !enabled {
			return core.Invalid("Los administradores siempre pueden registrar egresos de caja.")
		}
		if e = p.Admin.SetExpensePermission(ctx, id, enabled); e != nil {
			return e
		}
		entry := auditEntry(actor.ID, "user.expense_permission", id)
		entry.Details = map[string]any{"before": target.CanExpense(), "after": enabled}
		return p.Audit.Append(ctx, entry)
	})
}
