package httpapi_test

import (
	"business-manage/internal/application"
	"testing"
)

func TestProfileCompanyAndPasswordSettings(t *testing.T) {
	f := fixtureFor(t, true)
	session := f.login(t)
	other := f.login(t)
	w := f.request("PATCH", "/api/v1/auth/me", map[string]string{"name": "Nombre nuevo"}, session.Token, session.CSRFToken, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("PATCH", "/api/v1/company", map[string]string{"name": "Empresa nueva", "timezone": "UTC"}, session.Token, session.CSRFToken, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("PATCH", "/api/v1/company", map[string]string{"name": "Empresa nueva", "timezone": "UTC", "currency": "USD"}, session.Token, session.CSRFToken, nil)
	if w.Code != 400 {
		t.Fatal("currency was editable")
	}
	next := "Nueva frase segura 2026"
	w = f.request("POST", "/api/v1/auth/change-password", map[string]string{"currentPassword": "incorrecta", "newPassword": next}, session.Token, session.CSRFToken, nil)
	if w.Code != 400 {
		t.Fatal("wrong password accepted")
	}
	w = f.request("POST", "/api/v1/auth/change-password", map[string]string{"currentPassword": password, "newPassword": next}, session.Token, session.CSRFToken, nil)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, s := range []application.LoginResult{session, other} {
		w = f.request("GET", "/api/v1/auth/me", nil, s.Token, "", nil)
		if w.Code != 401 {
			t.Fatal("password change did not revoke session")
		}
	}
	w = f.request("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": next}, "", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
