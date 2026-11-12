package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/audit"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (r *repos) describeAudit(ctx context.Context, a *audit.Entry) error {
	meta := audit.Metadata(a.Action)
	a.Label = meta.Label
	if a.ActorName == "" {
		if u, e := r.User(ctx, a.ActorID); e == nil {
			a.ActorName = u.Name
		} else if !core.HasCode(e, "not_found") {
			return e
		} else {
			a.ActorName = "Consola local"
		}
	}
	if a.Reference != "" {
		return nil
	}
	a.SubjectType = meta.SubjectType
	a.SubjectID = a.EntityID
	switch a.SubjectType {
	case "sale":
		sale, e := r.Sale(ctx, a.EntityID)
		if e != nil {
			if core.HasCode(e, "not_found") {
				break
			}
			return e
		}
		a.Reference = fmt.Sprintf("Venta #%05d", sale.Number)
		items := []string{}
		for _, l := range sale.Lines {
			items = append(items, l.Name+" × "+l.Quantity)
		}
		a.Summary = strings.Join(items, "; ")
	case "product":
		id := a.EntityID
		if a.Action == "inventory.receive" {
			if e := r.db.QueryRowContext(ctx, "SELECT product_id FROM inventory_receipts WHERE id=?", id).Scan(&id); e != nil {
				return translate(e)
			}
		}
		a.SubjectID = id
		product, e := r.Product(ctx, id)
		if e != nil {
			if core.HasCode(e, "not_found") {
				break
			}
			return e
		}
		sku, name := product.SKU, product.Name
		for _, key := range []string{"after", "product"} {
			if m, ok := a.Details[key].(map[string]any); ok {
				if v, ok := m["sku"].(string); ok {
					sku = v
				}
				if v, ok := m["name"].(string); ok {
					name = v
				}
			}
		}
		a.Reference = "Producto " + sku
		a.Summary = name
	case "user":
		u, e := r.User(ctx, a.EntityID)
		if e == nil {
			a.Reference = "Usuario " + u.Username
			a.Summary = u.Name
		} else if !core.HasCode(e, "not_found") {
			return e
		}
	case "cash":
		m, e := r.Movement(ctx, a.EntityID)
		if e != nil {
			return e
		}
		a.Reference, a.Summary = m.Reference, m.Reason
	case "company":
		c, e := r.Company(ctx)
		if e == nil {
			a.Reference = c.Name
			a.Summary = "Datos de la empresa"
		} else if !core.HasCode(e, "not_found") {
			return e
		}
	case "promotion":
		p, e := r.Rule(ctx, a.EntityID)
		if e == nil {
			a.Reference = p.Name
			a.Summary = "Regla comercial"
		} else if !core.HasCode(e, "not_found") {
			return e
		}
	case "printer":
		if a.Action != "pos.settings" {
			j, e := r.Job(ctx, a.EntityID)
			if e == nil {
				a.Reference = j.PrinterName
				a.Summary = j.Description()
				if j.SaleID != "" {
					a.SubjectType = "sale"
					a.SubjectID = j.SaleID
					a.Reference = fmt.Sprintf("Venta #%05d", j.SaleNumber)
				}
				if j.CashMovementID != "" {
					m, e := r.Movement(ctx, j.CashMovementID)
					if e != nil {
						return e
					}
					a.SubjectType, a.SubjectID, a.Reference = "cash", m.ID, m.Reference
				}
				break
			} else if !core.HasCode(e, "not_found") {
				return e
			}
		}
		cfg, e := r.Settings(ctx)
		if e != nil {
			return e
		}
		a.Reference = cfg.DeviceName
		a.SubjectID = ""
		a.Summary = "Periféricos de caja"
	}
	if a.Reference == "" {
		a.Reference = meta.Label
	}
	return nil
}
func productIDs(value any, ids map[string]bool) {
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			if k == "productId" {
				if id, ok := item.(string); ok {
					ids[id] = true
				}
			}
			if k == "productIds" || k == "rewardProductIds" {
				if all, ok := item.([]any); ok {
					for _, id := range all {
						if s, ok := id.(string); ok {
							ids[s] = true
						}
					}
				}
			}
			productIDs(item, ids)
		}
	case []any:
		for _, item := range v {
			productIDs(item, ids)
		}
	}
}
func (r *repos) Append(ctx context.Context, a audit.Entry) error {
	a.Details = audit.SafeDetails(a.Details)
	names := map[string]string{}
	ids := map[string]bool{}
	productIDs(a.Details, ids)
	for id := range ids {
		p, e := r.Product(ctx, id)
		if e == nil {
			names[id] = p.Name + " (" + p.SKU + ")"
		} else if !core.HasCode(e, "not_found") {
			return e
		}
	}
	if len(names) > 0 {
		a.Details["productNames"] = names
	}
	if e := r.describeAudit(ctx, &a); e != nil {
		return e
	}
	raw, e := json.Marshal(a.Details)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, "INSERT INTO audit_entries(id,actor_id,action,entity_id,created_at,details_json,actor_name,reference,summary,subject_type,subject_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)", a.ID, a.ActorID, a.Action, a.EntityID, a.CreatedAt, string(raw), a.ActorName, a.Reference, a.Summary, a.SubjectType, a.SubjectID)
	return e
}
func (r *repos) Entry(ctx context.Context, id string) (audit.Entry, error) {
	var a audit.Entry
	var raw string
	e := r.db.QueryRowContext(ctx, "SELECT id,actor_id,action,entity_id,created_at,details_json,actor_name,reference,summary,subject_type,subject_id FROM audit_entries WHERE id=?", id).Scan(&a.ID, &a.ActorID, &a.Action, &a.EntityID, &a.CreatedAt, &raw, &a.ActorName, &a.Reference, &a.Summary, &a.SubjectType, &a.SubjectID)
	if e != nil {
		return a, translate(e)
	}
	if e = json.Unmarshal([]byte(raw), &a.Details); e != nil {
		return a, e
	}
	a.Details = audit.SafeDetails(a.Details)
	a.Historical = a.Reference == ""
	e = r.describeAudit(ctx, &a)
	return a, e
}
func (r *repos) Audit(ctx context.Context, q core.Query) (core.Page[audit.Entry], error) {
	q = q.Normalized()
	out := core.Page[audit.Entry]{Items: []audit.Entry{}, Page: q.Page, PageSize: q.PageSize}
	matches := []string{}
	args := []any{q.Action, q.Action, like(audit.Fold(q.Search))}
	for _, o := range audit.Operations {
		if strings.Contains(audit.Fold(o.Label+" "+o.Code), audit.Fold(q.Search)) {
			matches = append(matches, "?")
			args = append(args, o.Code)
		}
	}
	where := " FROM audit_entries a WHERE (?='' OR a.action=?) AND (a.action LIKE ? ESCAPE '!'"
	if len(matches) > 0 {
		where += " OR a.action IN (" + strings.Join(matches, ",") + ")"
	}
	where += ")"
	if e := r.db.QueryRowContext(ctx, "SELECT count(*)"+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	params := append(append([]any{}, args...), q.PageSize, q.Offset())
	rows, e := r.db.QueryContext(ctx, "SELECT a.id"+where+" ORDER BY a.created_at DESC,a.id DESC LIMIT ? OFFSET ?", params...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		a, e := r.Entry(ctx, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, a)
	}
	return out, nil
}
