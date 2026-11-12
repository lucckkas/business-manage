package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/pos"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

func (r *repos) Settings(ctx context.Context) (pos.Settings, error) {
	var raw string
	e := r.db.QueryRowContext(ctx, "SELECT config_json FROM pos_settings WHERE singleton=1").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return pos.Defaults(), nil
	}
	var s pos.Settings
	if e == nil {
		e = json.Unmarshal([]byte(raw), &s)
	}
	return s, e
}
func (r *repos) SaveSettings(ctx context.Context, s pos.Settings, expected int) error {
	old, e := r.Settings(ctx)
	if e != nil {
		return e
	}
	if old.Version != expected {
		return core.Conflict("La configuración cambió. Recarga antes de guardar.")
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, "INSERT INTO pos_settings(singleton,config_json) VALUES(1,?) ON CONFLICT(singleton) DO UPDATE SET config_json=excluded.config_json", string(raw))
	return e
}
func (r *repos) Enqueue(ctx context.Context, j pos.Job) error {
	config, e := json.Marshal(j.Destination)
	if e != nil {
		return e
	}
	var sale any
	if j.SaleID != "" {
		sale = j.SaleID
	}
	var cash any
	if j.CashMovementID != "" {
		cash = j.CashMovementID
	}
	_, e = r.db.ExecContext(ctx, "INSERT INTO pos_jobs(id,sale_id,cash_movement_id,kind,state,actor_id,created_at,updated_at,idempotency_key,request_hash,destination_json,payload,opens_drawer,reason) VALUES(?,?,?,?,'pending',?,?,?,?,?,?,?,?,?)", j.ID, sale, cash, j.Kind, j.ActorID, j.CreatedAt, j.CreatedAt, j.Key, j.RequestHash, string(config), j.Payload, j.OpensDrawer, j.Reason)
	return translate(e)
}
func (r *repos) loadJob(ctx context.Context, where string, value string) (pos.Job, error) {
	var j pos.Job
	var config string
	e := r.db.QueryRowContext(ctx, "SELECT j.id,COALESCE(j.sale_id,''),COALESCE(s.number,0),COALESCE(j.cash_movement_id,''),j.kind,j.state,j.actor_id,j.created_at,j.updated_at,j.idempotency_key,j.request_hash,j.destination_json,j.payload,j.opens_drawer,j.reason,j.error,j.spool_id FROM pos_jobs j LEFT JOIN sales_documents s ON s.id=j.sale_id WHERE "+where, value).Scan(&j.ID, &j.SaleID, &j.SaleNumber, &j.CashMovementID, &j.Kind, &j.State, &j.ActorID, &j.CreatedAt, &j.UpdatedAt, &j.Key, &j.RequestHash, &config, &j.Payload, &j.OpensDrawer, &j.Reason, &j.Error, &j.SpoolID)
	if e != nil {
		return j, translate(e)
	}
	e = json.Unmarshal([]byte(config), &j.Destination)
	j.PrinterName = j.Destination.DeviceName
	return j, e
}
func (r *repos) Job(ctx context.Context, id string) (pos.Job, error) {
	return r.loadJob(ctx, "j.id=?", id)
}
func (r *repos) JobByKey(ctx context.Context, key string) (pos.Job, error) {
	return r.loadJob(ctx, "j.idempotency_key=?", key)
}
func (r *repos) Jobs(ctx context.Context, q core.Query, sale string) (core.Page[pos.Job], error) {
	q = q.Normalized()
	out := core.Page[pos.Job]{Items: []pos.Job{}, Page: q.Page, PageSize: q.PageSize}
	const filter = " FROM pos_jobs WHERE (?='' OR sale_id=?)"
	if e := r.db.QueryRowContext(ctx, "SELECT count(*)"+filter, sale, sale).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := r.db.QueryContext(ctx, "SELECT id"+filter+" ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", sale, sale, q.PageSize, q.Offset())
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
		j, e := r.Job(ctx, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, j)
	}
	return out, nil
}
func (r *repos) Claim(ctx context.Context) (*pos.Job, error) {
	var id string
	e := r.db.QueryRowContext(ctx, "SELECT id FROM pos_jobs WHERE state='pending' AND NOT EXISTS(SELECT 1 FROM pos_jobs busy WHERE busy.state='sending') ORDER BY created_at,id LIMIT 1").Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	result, e := r.db.ExecContext(ctx, "UPDATE pos_jobs SET state='sending',updated_at=? WHERE id=? AND state='pending'", core.Now(), id)
	if e != nil {
		return nil, e
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return nil, e
	}
	j, e := r.Job(ctx, id)
	return &j, e
}
func (r *repos) Finish(ctx context.Context, id, state, message, spool string) error {
	result, e := r.db.ExecContext(ctx, "UPDATE pos_jobs SET state=?,error=?,spool_id=?,updated_at=? WHERE id=? AND state='sending'", state, message, spool, core.Now(), id)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e == nil && n != 1 {
		return core.Conflict("El trabajo ya cambió de estado.")
	}
	return e
}
func (r *repos) Interrupted(ctx context.Context) ([]pos.Job, error) {
	rows, e := r.db.QueryContext(ctx, "SELECT id FROM pos_jobs WHERE state='sending' ORDER BY created_at,id")
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []pos.Job{}
	for _, id := range ids {
		j, e := r.Job(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, nil
}
