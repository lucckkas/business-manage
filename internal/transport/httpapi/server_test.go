package httpapi_test

import (
	"business-manage/internal/adapters/sqlite"
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/sales"
	"business-manage/internal/transport/httpapi"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const password = "A long test password 2026"

func TestHealthFailureUsesCommonErrorContract(t *testing.T) {
	f := fixtureFor(t, false)
	f.store.Close()
	w := f.request("GET", "/api/v1/health", nil, "", "", nil)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	var result struct {
		Error struct{ Code, Message, RequestID string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Error.Code != "busy" || result.Error.RequestID == "" {
		t.Fatalf("health error: %s (%v)", w.Body.String(), err)
	}
}

type fixture struct {
	handler http.Handler
	app     *application.Service
	store   *sqlite.Store
	root    string
	token   string
}

func fixtureFor(t *testing.T, initialized bool) *fixture {
	t.Helper()
	root := t.TempDir()
	store, e := sqlite.Open(filepath.Join(root, "erp.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	if e = store.Migrate(context.Background(), nil); e != nil {
		t.Fatal(e)
	}
	app := application.New(store)
	token := core.Token()
	os.WriteFile(filepath.Join(root, "setup.token"), []byte(token), 0600)
	if initialized {
		if e = app.Setup(context.Background(), application.SetupRequest{Company: admin.Company{Name: "Empresa prueba", Currency: "CLP", Timezone: "America/Santiago"}, Username: "admin", Name: "Admin", Password: password}); e != nil {
			t.Fatal(e)
		}
	}
	return &fixture{httpapi.New(app, httpapi.Options{DataDir: root, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Ping: store.DB.PingContext, UI: http.NotFoundHandler()}), app, store, root, token}
}
func (f *fixture) request(method, path string, body any, token, csrf string, extra map[string]string) *httptest.ResponseRecorder {
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "http://erp.local"+path, bytes.NewReader(data))
	r.RemoteAddr = "127.0.0.1:5000"
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.AddCookie(&http.Cookie{Name: "erp_session", Value: token})
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	for k, v := range extra {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func (f *fixture) login(t *testing.T) application.LoginResult {
	t.Helper()
	login, e := f.app.Login(context.Background(), "admin", password)
	if e != nil {
		t.Fatal(e)
	}
	return login
}
func TestSetupRequiresLocalSingleUseToken(t *testing.T) {
	f := fixtureFor(t, false)
	body := application.SetupRequest{Company: admin.Company{Name: "Prueba", Currency: "CLP", Timezone: "UTC"}, Username: "admin", Name: "Admin", Password: password}
	w := f.request("POST", "/api/v1/setup", body, "", "", nil)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	r := httptest.NewRequest("POST", "http://erp.local/api/v1/setup", strings.NewReader("{}"))
	r.RemoteAddr = "192.168.1.50:4500"
	r.Header.Set("Authorization", "Setup "+f.token)
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("remote setup allowed")
	}
	w = f.request("POST", "/api/v1/setup", body, "", "", map[string]string{"Authorization": "Setup " + f.token})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(f.root, "setup.token")); !os.IsNotExist(err) {
		t.Fatal("token retained")
	}
	w = f.request("POST", "/api/v1/setup", body, "", "", map[string]string{"Authorization": "Setup " + f.token})
	if w.Code != 403 {
		t.Fatal("token reused")
	}
}
func TestAuthenticationCSRFOriginAndPermissions(t *testing.T) {
	f := fixtureFor(t, true)
	w := f.request("GET", "/api/v1/products", nil, "", "", nil)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	login := f.login(t)
	body := map[string]string{"sku": "P-1", "name": "Producto uno", "unit": "un", "priceMinor": "100"}
	w = f.request("POST", "/api/v1/products", body, login.Token, "", nil)
	if w.Code != 403 {
		t.Fatal("missing CSRF allowed")
	}
	w = f.request("POST", "/api/v1/products", body, login.Token, login.CSRFToken, map[string]string{"Origin": "https://evil.example"})
	if w.Code != 403 {
		t.Fatal("cross origin allowed")
	}
	w = f.request("POST", "/api/v1/products", body, login.Token, login.CSRFToken, nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	operator, err := f.app.CreateUser(context.Background(), login.User, application.CreateUserRequest{Username: "operator", Name: "Operador", Password: password, Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.app.Login(context.Background(), "operator", password)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/users", "/api/v1/audit", "/api/v1/system/diagnostics"} {
		w = f.request("GET", path, nil, other.Token, "", nil)
		if w.Code != 403 {
			t.Fatalf("operator accessed %s", path)
		}
	}
	w = f.request("POST", "/api/v1/products", body, other.Token, other.CSRFToken, nil)
	if w.Code != 403 {
		t.Fatal("operator wrote catalog")
	}
	if err = f.app.SetUserActive(context.Background(), login.User, operator.ID, false); err != nil {
		t.Fatal(err)
	}
	w = f.request("GET", "/api/v1/products", nil, other.Token, "", nil)
	if w.Code != 401 {
		t.Fatal("disabled session remained active")
	}
	w = f.request("POST", "/api/v1/auth/logout", nil, login.Token, login.CSRFToken, nil)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	w = f.request("GET", "/api/v1/auth/me", nil, login.Token, "", nil)
	if w.Code != 401 {
		t.Fatal("logged out session remained active")
	}
}
func TestLoginCookieFlagsAndValidation(t *testing.T) {
	f := fixtureFor(t, true)
	r := httptest.NewRequest("POST", "https://erp.local/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"`+password+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe cookie %+v", cookies)
	}
	login := f.login(t)
	w = f.request("POST", "/api/v1/products", map[string]string{"sku": "P-1", "name": "Producto", "unit": "un", "priceMinor": "1", "unrecognized": "x"}, login.Token, login.CSRFToken, nil)
	if w.Code != 400 {
		t.Fatal("unknown field accepted")
	}
	w = f.request("GET", "/api/v1/products?pageSize=1000", nil, login.Token, "", nil)
	if w.Code != 400 {
		t.Fatal("unbounded page accepted")
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing response metadata")
	}
}
func TestTenPersistentSessionsCanSellConcurrently(t *testing.T) {
	f := fixtureFor(t, true)
	var sessions []application.LoginResult
	for i := 0; i < 10; i++ {
		sessions = append(sessions, f.login(t))
	}
	p, e := f.app.CreateProduct(context.Background(), sessions[0].User, catalog.Product{SKU: "CARGA", Name: "Producto carga", Unit: "un", PriceMinor: "1500"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.app.ReceiveStock(context.Background(), sessions[0].User, inventory.Receipt{ProductID: p.ID, Quantity: "10", Reason: "Inicial"}); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	times := make(chan time.Duration, 10)
	started := time.Now()
	for _, session := range sessions {
		wg.Add(1)
		go func(session application.LoginResult) {
			defer wg.Done()
			start := time.Now()
			request := sales.Request{Lines: []sales.LineRequest{{ProductID: p.ID, Quantity: "1"}}}
			quote, err := f.app.QuoteSale(context.Background(), session.User, request)
			if err != nil {
				t.Error(err)
				return
			}
			request.QuoteFingerprint = quote.Fingerprint
			w := f.request("POST", "/api/v1/sales", request, session.Token, session.CSRFToken, map[string]string{"Idempotency-Key": core.Token()})
			times <- time.Since(start)
			if w.Code != 201 {
				t.Errorf("sale status=%d body=%s", w.Code, w.Body.String())
			}
		}(session)
	}
	wg.Wait()
	close(times)
	var samples []time.Duration
	for d := range times {
		samples = append(samples, d)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	summary, e := f.store.Read().Reports.Summary(context.Background(), "2000-01-01", "2100-01-01")
	if e != nil || summary.Sales != 10 || summary.CashMinor != "15000" {
		t.Fatalf("load consistency %+v %v", summary, e)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("10 authenticated HTTP sessions: total=%s p95=%s heap=%.1f MiB runtime=%.1f MiB", time.Since(started), samples[9], float64(memory.Alloc)/(1<<20), float64(memory.Sys)/(1<<20))
}
