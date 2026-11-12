package sqlite

import (
	"business-manage/internal/core"
	"business-manage/internal/modules/treasury"
	"context"
	"fmt"
)

const cashFrom = ` FROM treasury_movements m LEFT JOIN sales_documents s ON s.id=m.sale_id JOIN admin_users u ON u.id=m.actor_id `
const cashColumns = `m.id,m.kind,COALESCE(m.sale_id,''),COALESCE(s.number,0),COALESCE(m.expense_number,0),CAST(m.amount_minor AS TEXT),m.actor_id,CASE WHEN m.actor_name='' THEN u.name ELSE m.actor_name END,m.reason,m.created_at,m.balance_before,m.balance_after,m.drawer_job_id,COALESCE(m.idempotency_key,''),m.request_hash`

func scanMovement(row scanner) (treasury.Movement, error) {
	var m treasury.Movement
	err := row.Scan(&m.ID, &m.Kind, &m.SaleID, &m.SaleNumber, &m.ExpenseNumber, &m.AmountMinor, &m.ActorID, &m.ActorName, &m.Reason, &m.CreatedAt, &m.BalanceBefore, &m.BalanceAfter, &m.DrawerJobID, &m.IdempotencyKey, &m.RequestHash)
	if m.Kind == "expense" {
		m.Reference = fmt.Sprintf("Egreso #%05d", m.ExpenseNumber)
	} else {
		m.Reference = fmt.Sprintf("Venta #%05d", m.SaleNumber)
	}
	return m, translate(err)
}
func (r *repos) Movement(ctx context.Context, id string) (treasury.Movement, error) {
	return scanMovement(r.db.QueryRowContext(ctx, "SELECT "+cashColumns+cashFrom+" WHERE m.id=?", id))
}
func (r *repos) ExpenseByKey(ctx context.Context, key string) (treasury.Movement, error) {
	return scanMovement(r.db.QueryRowContext(ctx, "SELECT "+cashColumns+cashFrom+" WHERE m.idempotency_key=?", key))
}
func (r *repos) Balance(ctx context.Context) (string, error) {
	var balance string
	err := r.db.QueryRowContext(ctx, "SELECT CAST(COALESCE(sum(amount_minor),0) AS TEXT) FROM treasury_movements").Scan(&balance)
	return balance, err
}
func (r *repos) CreditSale(ctx context.Context, m treasury.Movement, actor string) error {
	u, err := r.User(ctx, actor)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, "INSERT INTO treasury_movements(id,kind,sale_id,amount_minor,actor_id,actor_name,created_at) VALUES(?,'sale',?,?,?,?,?)", m.ID, m.SaleID, m.AmountMinor, actor, u.Name, m.CreatedAt)
	return translate(err)
}
func (r *repos) CreateExpense(ctx context.Context, m *treasury.Movement) error {
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(max(expense_number),0)+1 FROM treasury_movements").Scan(&m.ExpenseNumber); err != nil {
		return err
	}
	m.Reference = fmt.Sprintf("Egreso #%05d", m.ExpenseNumber)
	_, err := r.db.ExecContext(ctx, `INSERT INTO treasury_movements(id,kind,expense_number,amount_minor,actor_id,actor_name,reason,created_at,balance_before,balance_after,idempotency_key,request_hash,drawer_job_id) VALUES(?,'expense',?,?,?,?,?,?,?,?,?,?,?)`, m.ID, m.ExpenseNumber, m.AmountMinor, m.ActorID, m.ActorName, m.Reason, m.CreatedAt, m.BalanceBefore, m.BalanceAfter, m.IdempotencyKey, m.RequestHash, m.DrawerJobID)
	return translate(err)
}
func (r *repos) Cash(ctx context.Context, q core.Query) (core.Page[treasury.Movement], error) {
	q = q.Normalized()
	out := core.Page[treasury.Movement]{Items: []treasury.Movement{}, Page: q.Page, PageSize: q.PageSize}
	filter := like(q.Search)
	const where = ` WHERE (?='' OR m.kind=?) AND ((CASE WHEN m.kind='sale' THEN 'Venta #'||printf('%05d',s.number) ELSE 'Egreso #'||printf('%05d',m.expense_number) END) LIKE ? ESCAPE '!' OR m.reason LIKE ? ESCAPE '!')`
	if err := r.db.QueryRowContext(ctx, "SELECT count(*)"+cashFrom+where, q.Kind, q.Kind, filter, filter).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := r.db.QueryContext(ctx, "SELECT "+cashColumns+cashFrom+where+" ORDER BY m.created_at DESC,m.id DESC LIMIT ? OFFSET ?", q.Kind, q.Kind, filter, filter, q.PageSize, q.Offset())
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		m, err := scanMovement(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, m)
	}
	return out, rows.Err()
}
