package application_test

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/treasury"
	"bytes"
	"errors"
	"sync"
	"testing"
)

func TestExpensePermissionExactBalancesAuditAndReplay(t *testing.T) {
	f := newFixture(t)
	p := f.product(t, "EXP-INCOME", "1250", "2")
	if _, _, e := f.confirm(ctx, f.user, core.Token(), request(p.ID, "2")); e != nil {
		t.Fatal(e)
	}
	operator := admin.User{ID: core.ID(), Username: "cashier", Name: "Persona original", Role: "operator", Active: true, CreatedAt: core.Now(), PasswordHash: "fixture"}
	if e := f.store.Read().Admin.CreateUser(ctx, operator); e != nil {
		t.Fatal(e)
	}
	r := treasury.ExpenseRequest{AmountMinor: "003101", Reason: " Compra de papel higiénico "}
	key := core.Token()
	if _, _, e := f.app.RecordCashExpense(ctx, operator, key, r); !core.HasCode(e, "forbidden") {
		t.Fatal(e)
	}
	if e := f.app.SetExpensePermission(ctx, f.user, operator.ID, true); e != nil {
		t.Fatal(e)
	}
	// The application must read current permission, not trust the caller's old object.
	m, replayed, e := f.app.RecordCashExpense(ctx, operator, key, r)
	if e != nil || replayed || m.AmountMinor != "-3101" || m.ExpenseNumber != 1 || m.Reference != "Egreso #00001" || *m.BalanceBefore != "2500" || *m.BalanceAfter != "-601" || m.DrawerJobID != "" {
		t.Fatalf("%+v %v %v", m, replayed, e)
	}
	summary, e := f.app.Summary(ctx)
	if e != nil || summary.CashMinor != "-601" || summary.CashIncomeMinor != "2500" || summary.CashExpenseMinor != "3101" || summary.RevenueMinor != "2500" || summary.Sales != 1 {
		t.Fatal(summary, e)
	}
	events, e := f.store.Read().Reports.Audit(ctx, core.Query{Search: "egreso", Action: "cash.expense"})
	if e != nil || events.Total != 1 || events.Items[0].Reference != m.Reference || events.Items[0].Details["balanceAfter"] != "-601" {
		t.Fatal(events, e)
	}
	if e = f.app.SetExpensePermission(ctx, f.user, operator.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.UpdateProfile(ctx, operator, "Nombre posterior"); e != nil {
		t.Fatal(e)
	}
	again, replayed, e := f.app.RecordCashExpense(ctx, operator, key, treasury.ExpenseRequest{AmountMinor: "3101", Reason: r.Reason})
	if e != nil || !replayed || again.ID != m.ID || again.ActorName != operator.Name || *again.BalanceAfter != "-601" {
		t.Fatal(again, replayed, e)
	}
	if _, _, e = f.app.RecordCashExpense(ctx, f.user, key, r); !core.HasCode(e, "conflict") {
		t.Fatal("cross-user replay", e)
	}
	r.AmountMinor = "3102"
	if _, _, e = f.app.RecordCashExpense(ctx, operator, key, r); !core.HasCode(e, "conflict") {
		t.Fatal("changed replay", e)
	}
	if _, _, e = f.app.RecordCashExpense(ctx, operator, core.Token(), r); !core.HasCode(e, "forbidden") {
		t.Fatal(e)
	}
	for _, sql := range []string{"UPDATE treasury_movements SET reason='changed' WHERE kind='expense'", "DELETE FROM treasury_movements WHERE kind='expense'"} {
		if _, e = f.store.DB.Exec(sql); e == nil {
			t.Fatal("ledger mutation accepted")
		}
	}
	if count(t, f.store, "treasury_movements") != 2 {
		t.Fatal("duplicate ledger entries")
	}
}

func TestExpenseValidationFilteringAndConcurrentCorrelatives(t *testing.T) {
	f := newFixture(t)
	for _, r := range []treasury.ExpenseRequest{{AmountMinor: "0", Reason: "Gasto"}, {AmountMinor: "-1", Reason: "Gasto"}, {AmountMinor: "1.25", Reason: "Gasto"}, {AmountMinor: "1000000000000001", Reason: "Gasto"}, {AmountMinor: "1", Reason: "  "}} {
		if _, _, e := f.app.RecordCashExpense(ctx, f.user, core.Token(), r); e == nil {
			t.Fatal("invalid expense", r)
		}
	}
	if count(t, f.store, "treasury_movements") != 0 {
		t.Fatal("invalid expense persisted")
	}
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	sharedKey := core.Token()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, _, e := f.app.RecordCashExpense(ctx, f.user, sharedKey, treasury.ExpenseRequest{AmountMinor: "1", Reason: "Papel común"})
			if e != nil {
				t.Error(e)
				return
			}
			ids <- m.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for next := range ids {
		if id != "" && id != next {
			t.Fatal("duplicate concurrent expense")
		}
		id = next
	}
	for i := 0; i < 22; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, e := f.app.RecordCashExpense(ctx, f.user, core.Token(), treasury.ExpenseRequest{AmountMinor: "1", Reason: "Otros suministros"}); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	var distinct, maxNumber int
	f.store.DB.QueryRow("SELECT count(DISTINCT expense_number),max(expense_number) FROM treasury_movements").Scan(&distinct, &maxNumber)
	if distinct != 23 || maxNumber != 23 {
		t.Fatal(distinct, maxNumber)
	}
	page, e := f.store.Read().Reports.Cash(ctx, core.Query{Kind: "expense", Search: "Papel", PageSize: 1})
	if e != nil || page.Total != 1 || page.Items[0].ID != id {
		t.Fatal(page, e)
	}
	page, e = f.store.Read().Reports.Cash(ctx, core.Query{Kind: "expense", Search: "Egreso #00001"})
	if e != nil || page.Total != 1 || page.Items[0].ID != id {
		t.Fatal(page, e)
	}
	page, e = f.store.Read().Reports.Cash(ctx, core.Query{Kind: "expense", Page: 2, PageSize: 20})
	if e != nil || page.Total != 23 || len(page.Items) != 3 {
		t.Fatal(page, e)
	}
	page, e = f.store.Read().Reports.Cash(ctx, core.Query{Kind: "sale"})
	if e != nil || page.Total != 0 {
		t.Fatal(page, e)
	}
}

func TestExpenseDrawerOutboxRollbackFailureAndRecovery(t *testing.T) {
	f := newFixture(t)
	printer, cfg := readyPrinter(t, f)
	beforeJobs, beforeAudit := count(t, f.store, "pos_jobs"), count(t, f.store, "audit_entries")
	if _, e := f.store.DB.Exec("CREATE TRIGGER reject_expense_job_audit BEFORE INSERT ON audit_entries WHEN NEW.action='pos.enqueue' BEGIN SELECT RAISE(ABORT,'injected'); END"); e != nil {
		t.Fatal(e)
	}
	r := treasury.ExpenseRequest{AmountMinor: "5025", Reason: "Compra de insumos"}
	if _, _, e := f.app.RecordCashExpense(ctx, f.user, core.Token(), r); e == nil {
		t.Fatal("injected failure not observed")
	}
	if count(t, f.store, "treasury_movements") != 0 || count(t, f.store, "pos_jobs") != beforeJobs || count(t, f.store, "audit_entries") != beforeAudit || len(printer.writes) != 2 {
		t.Fatal("partial expense or hardware effect")
	}
	f.store.DB.Exec("DROP TRIGGER reject_expense_job_audit")
	key := core.Token()
	m, _, e := f.app.RecordCashExpense(ctx, f.user, key, r)
	if e != nil || m.DrawerJobID == "" || m.ExpenseNumber != 1 {
		t.Fatal(m, e)
	}
	j, e := f.store.Read().POS.Job(ctx, m.DrawerJobID)
	if e != nil || j.Kind != "expense_drawer" || j.CashMovementID != m.ID || !bytes.Equal(j.Payload, pos.Drawer(cfg)) || len(printer.writes) != 2 {
		t.Fatal(j, e)
	}
	printer.failure = &pos.SendError{Err: errors.New("partial USB write"), Uncertain: true}
	if worked, e := f.app.ProcessPrintJob(ctx); e != nil || !worked {
		t.Fatal(e)
	}
	j, _ = f.store.Read().POS.Job(ctx, j.ID)
	if j.State != "uncertain" {
		t.Fatal(j)
	}
	again, replayed, e := f.app.RecordCashExpense(ctx, f.user, key, r)
	if e != nil || !replayed || again.DrawerJobID != j.ID {
		t.Fatal(again, e)
	}
	if worked, e := f.app.ProcessPrintJob(ctx); e != nil || worked || len(printer.writes) != 3 {
		t.Fatal("drawer resent", e)
	}
	// A disable before dispatch suspends automatic work.
	m, _, e = f.app.RecordCashExpense(ctx, f.user, core.Token(), r)
	if e != nil {
		t.Fatal(e)
	}
	cfg.DrawerEnabled = false
	cfg, e = f.app.SavePOSSettings(ctx, f.user, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.ProcessPrintJob(ctx); e != nil {
		t.Fatal(e)
	}
	j, _ = f.store.Read().POS.Job(ctx, m.DrawerJobID)
	if j.State != "suspended" || len(printer.writes) != 3 {
		t.Fatal(j)
	}
	m, _, e = f.app.RecordCashExpense(ctx, f.user, core.Token(), r)
	if e != nil || m.DrawerJobID != "" {
		t.Fatal("disabled drawer queued", m, e)
	}
	cfg.DrawerEnabled = true
	if _, e = f.app.SavePOSSettings(ctx, f.user, cfg); e != nil {
		t.Fatal(e)
	}
	m, _, e = f.app.RecordCashExpense(ctx, f.user, core.Token(), r)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.store.WithinTx(ctx, func(p application.Ports) error { _, e := p.POS.Claim(ctx); return e }); e != nil {
		t.Fatal(e)
	}
	if e = f.app.RecoverPrintJobs(ctx); e != nil {
		t.Fatal(e)
	}
	j, _ = f.store.Read().POS.Job(ctx, m.DrawerJobID)
	if j.State != "uncertain" || len(printer.writes) != 3 {
		t.Fatal("interrupted drawer replay", j)
	}
}
