package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"context"
)

func (r *repos) Company(ctx context.Context) (admin.Company, error) {
	var c admin.Company
	err := r.db.QueryRowContext(ctx, "SELECT name,currency,currency_scale,timezone FROM admin_company WHERE singleton=1").Scan(&c.Name, &c.Currency, &c.CurrencyScale, &c.Timezone)
	return c, translate(err)
}
func (r *repos) CreateCompany(ctx context.Context, c admin.Company) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO admin_company VALUES(1,?,?,?,?)", c.Name, c.Currency, c.CurrencyScale, c.Timezone)
	return translate(err)
}

const userColumns = "id,username,name,role,active,password_hash,created_at,can_manual_discount,can_cash_expense"

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (admin.User, error) {
	var u admin.User
	err := row.Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.Active, &u.PasswordHash, &u.CreatedAt, &u.CanApplyManualDiscounts, &u.CanRecordCashExpenses)
	if u.Role == "admin" {
		u.CanApplyManualDiscounts = true
		u.CanRecordCashExpenses = true
	}
	return u, translate(err)
}
func (r *repos) UserByUsername(ctx context.Context, name string) (admin.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM admin_users WHERE username=?", name))
}
func (r *repos) User(ctx context.Context, id string) (admin.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM admin_users WHERE id=?", id))
}
func (r *repos) CreateUser(ctx context.Context, u admin.User) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO admin_users("+userColumns+") VALUES(?,?,?,?,?,?,?,?,?)", u.ID, u.Username, u.Name, u.Role, u.Active, u.PasswordHash, u.CreatedAt, u.CanApplyManualDiscounts, u.CanRecordCashExpenses)
	return translate(err)
}
func (r *repos) Users(ctx context.Context, q core.Query) (core.Page[admin.User], error) {
	q = q.Normalized()
	result := core.Page[admin.User]{Items: []admin.User{}, Page: q.Page, PageSize: q.PageSize}
	filter := like(q.Search)
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM admin_users WHERE name LIKE ? ESCAPE '!' OR username LIKE ? ESCAPE '!'", filter, filter).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := r.db.QueryContext(ctx, "SELECT "+userColumns+" FROM admin_users WHERE name LIKE ? ESCAPE '!' OR username LIKE ? ESCAPE '!' ORDER BY created_at DESC,id LIMIT ? OFFSET ?", filter, filter, q.PageSize, q.Offset())
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, u)
	}
	return result, rows.Err()
}
func (r *repos) SetActive(ctx context.Context, id string, active bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admin_users SET active=? WHERE id=?", active, id)
	return err
}
func (r *repos) ActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM admin_users WHERE role='admin' AND active=1").Scan(&n)
	return n, err
}
func (r *repos) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admin_users SET password_hash=? WHERE id=?", hash, id)
	return err
}
func (r *repos) SaveSession(ctx context.Context, s admin.Session) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM admin_sessions WHERE expires_at < ?", core.Now()); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, "INSERT INTO admin_sessions(hash,user_id,csrf_token,expires_at) VALUES(?,?,?,?)", s.Hash, s.UserID, s.CSRFToken, s.ExpiresAt)
	return err
}
func (r *repos) Session(ctx context.Context, hash string) (admin.Session, error) {
	var s admin.Session
	err := r.db.QueryRowContext(ctx, "SELECT hash,user_id,csrf_token,expires_at FROM admin_sessions WHERE hash=?", hash).Scan(&s.Hash, &s.UserID, &s.CSRFToken, &s.ExpiresAt)
	return s, translate(err)
}
func (r *repos) DeleteSession(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM admin_sessions WHERE hash=?", hash)
	return err
}
func (r *repos) DeleteUserSessions(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM admin_sessions WHERE user_id=?", id)
	return err
}
func (r *repos) UpdateProfile(ctx context.Context, id, name string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admin_users SET name=? WHERE id=?", name, id)
	return err
}
func (r *repos) UpdateCompany(ctx context.Context, c admin.Company) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admin_company SET name=?,timezone=? WHERE singleton=1", c.Name, c.Timezone)
	return err
}
func (r *repos) SetDiscountPermission(ctx context.Context, id string, enabled bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admin_users SET can_manual_discount=? WHERE id=?", enabled, id)
	return err
}
func (r *repos) SetExpensePermission(ctx context.Context, id string, enabled bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admin_users SET can_cash_expense=? WHERE id=?", enabled, id)
	return err
}
