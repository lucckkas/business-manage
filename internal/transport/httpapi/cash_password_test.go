package httpapi_test

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCashExpenseHTTPPermissionsCSRFAndReplay(t *testing.T) {
	f := fixtureFor(t, true)
	s := f.login(t)
	user, e := f.app.CreateUser(context.Background(), s.User, application.CreateUserRequest{Username: "expense-operator", Name: "Operador de caja", Role: "operator", Password: password})
	if e != nil {
		t.Fatal(e)
	}
	op, e := f.app.Login(context.Background(), user.Username, password)
	if e != nil {
		t.Fatal(e)
	}
	body := map[string]string{"amountMinor": "125", "reason": "Comprar papel"}
	headers := map[string]string{"Idempotency-Key": core.Token()}
	for _, tc := range []struct {
		token, csrf string
		status      int
	}{{"", "", 401}, {s.Token, "", 403}, {op.Token, op.CSRFToken, 403}} {
		w := f.request("POST", "/api/v1/cash/expenses", body, tc.token, tc.csrf, headers)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := f.request("PATCH", "/api/v1/users/"+user.ID, map[string]bool{"canRecordCashExpenses": true}, s.Token, s.CSRFToken, nil)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("POST", "/api/v1/cash/expenses", body, op.Token, op.CSRFToken, headers)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct{ ID, AmountMinor, Reference string }
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.AmountMinor != "-125" || result.Reference != "Egreso #00001" {
		t.Fatal(result)
	}
	w = f.request("GET", "/api/v1/cash/movements/"+result.ID, nil, op.Token, "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("POST", "/api/v1/cash/expenses", body, op.Token, op.CSRFToken, headers)
	if w.Code != 200 || w.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("GET", "/api/v1/cash/movements?kind=expense&search=papel", nil, op.Token, "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), result.ID) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("GET", "/api/v1/cash/movements?kind=wrong", nil, op.Token, "", nil)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestOptionalPasswordSetupLoginAndChange(t *testing.T) {
	f := fixtureFor(t, false)
	r := application.SetupRequest{Company: admin.Company{Name: "Empresa sin clave", Currency: "CLP", Timezone: "UTC"}, Username: "admin", Name: "Administrador", Password: ""}
	if e := f.app.Setup(context.Background(), r); !core.HasCode(e, "validation") {
		t.Fatal("empty setup without acceptance", e)
	}
	r.AllowWeakPassword = true
	if e := f.app.Setup(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	s, e := f.app.Login(context.Background(), "admin", "")
	if e != nil {
		t.Fatal(e)
	}
	other, e := f.app.Login(context.Background(), "admin", "")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(s.User.PasswordHash, "$argon2id$") {
		t.Fatal("empty credential was not hashed")
	}
	w := f.request("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": ""}, "", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	short := map[string]any{"username": "short", "name": "Cuenta breve", "role": "operator", "password": "123"}
	w = f.request("POST", "/api/v1/users", short, s.Token, s.CSRFToken, nil)
	if w.Code != 400 {
		t.Fatal("short password without acceptance", w.Code)
	}
	short["allowWeakPassword"] = true
	w = f.request("POST", "/api/v1/users", short, s.Token, s.CSRFToken, nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = f.app.Login(context.Background(), "short", "123"); e != nil {
		t.Fatal(e)
	}
	// Empty current password must still be checked, and acceptance does not bypass it.
	change := map[string]any{"currentPassword": "incorrecta", "newPassword": "", "allowWeakPassword": true}
	w = f.request("POST", "/api/v1/auth/change-password", change, s.Token, s.CSRFToken, nil)
	if w.Code != 400 {
		t.Fatal("wrong current credential accepted", w.Code)
	}
	change["currentPassword"] = ""
	change["allowWeakPassword"] = false
	w = f.request("POST", "/api/v1/auth/change-password", change, s.Token, s.CSRFToken, nil)
	if w.Code != 400 {
		t.Fatal("empty change without acceptance", w.Code)
	}
	change["allowWeakPassword"] = true
	w = f.request("POST", "/api/v1/auth/change-password", change, s.Token, s.CSRFToken, nil)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, session := range []application.LoginResult{s, other} {
		w = f.request("GET", "/api/v1/auth/me", nil, session.Token, "", nil)
		if w.Code != 401 {
			t.Fatal("session survived credential change")
		}
	}
	s, e = f.app.Login(context.Background(), "admin", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = f.app.ChangePassword(context.Background(), s.User, "", strings.Repeat("á", 128)); e != nil {
		t.Fatal("unicode length mismatch", e)
	}
	if _, e = f.app.Login(context.Background(), "admin", strings.Repeat("á", 128)); e != nil {
		t.Fatal(e)
	}
	if e = f.app.ResetPassword(context.Background(), "admin", ""); !core.HasCode(e, "validation") {
		t.Fatal(e)
	}
	if e = f.app.ResetPassword(context.Background(), "admin", strings.Repeat("a", 129), true); !core.HasCode(e, "validation") {
		t.Fatal(e)
	}
	if e = f.app.ResetPassword(context.Background(), "admin", "", true); e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.Login(context.Background(), "admin", ""); e != nil {
		t.Fatal(e)
	}
	w = f.request("GET", "/api/v1/audit", nil, s.Token, "", nil)
	if w.Code != 401 {
		t.Fatal("reset did not revoke sessions")
	}
	entries, e := f.app.Store.Read().Reports.Audit(context.Background(), core.Query{})
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries.Items {
		raw, _ := json.Marshal(entry.Details)
		if strings.Contains(string(raw), "123") || strings.Contains(string(raw), "argon2") || strings.Contains(string(raw), "currentPassword") {
			t.Fatal("credential exposed", string(raw))
		}
	}
}
