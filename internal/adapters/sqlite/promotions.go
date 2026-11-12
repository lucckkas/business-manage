package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/promotions"
	"context"
	"encoding/json"
)

func (r *repos) Rules(ctx context.Context, q core.Query) (core.Page[promotions.Rule], error) {
	q = q.Normalized()
	out := core.Page[promotions.Rule]{Items: []promotions.Rule{}, Page: q.Page, PageSize: q.PageSize}
	filter := like(q.Search)
	if e := r.db.QueryRowContext(ctx, "SELECT count(*) FROM promotion_rules WHERE name LIKE ? ESCAPE '!'", filter).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := r.db.QueryContext(ctx, "SELECT config_json FROM promotion_rules WHERE name LIKE ? ESCAPE '!' ORDER BY name,id LIMIT ? OFFSET ?", filter, q.PageSize, q.Offset())
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var rule promotions.Rule
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if e = json.Unmarshal([]byte(raw), &rule); e != nil {
			return out, e
		}
		out.Items = append(out.Items, rule)
	}
	return out, rows.Err()
}
func (r *repos) All(ctx context.Context) ([]promotions.Rule, error) {
	out := []promotions.Rule{}
	rows, e := r.db.QueryContext(ctx, "SELECT config_json FROM promotion_rules ORDER BY id")
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var rule promotions.Rule
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if e = json.Unmarshal([]byte(raw), &rule); e != nil {
			return out, e
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}
func (r *repos) Rule(ctx context.Context, id string) (promotions.Rule, error) {
	var rule promotions.Rule
	var raw string
	e := r.db.QueryRowContext(ctx, "SELECT config_json FROM promotion_rules WHERE id=?", id).Scan(&raw)
	if e != nil {
		return rule, translate(e)
	}
	e = json.Unmarshal([]byte(raw), &rule)
	return rule, e
}
func (r *repos) Save(ctx context.Context, rule promotions.Rule, expected int) error {
	raw, e := json.Marshal(rule)
	if e != nil {
		return e
	}
	if expected == 0 {
		_, e = r.db.ExecContext(ctx, "INSERT INTO promotion_rules VALUES(?,?,?,?)", rule.ID, rule.Name, rule.Version, string(raw))
		return translate(e)
	}
	result, e := r.db.ExecContext(ctx, "UPDATE promotion_rules SET name=?,version=?,config_json=? WHERE id=? AND version=?", rule.Name, rule.Version, string(raw), rule.ID, expected)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return core.Conflict("La oferta cambió. Recarga antes de guardar.")
	}
	return nil
}
