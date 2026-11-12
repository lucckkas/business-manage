// Package httpapi owns HTTP, authentication and DTO translation, not business SQL.
package httpapi

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/catalog"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/sales"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Options struct {
	DataDir string
	Log     *slog.Logger
	UI      http.Handler
	OpenAPI []byte
	Ping    func(context.Context) error
	Backup  func(context.Context) (string, error)
	Version string
}
type Server struct {
	app           *application.Service
	options       Options
	limiter       rateLimiter
	passwordSlots chan struct{}
	requests      atomic.Uint64
	started       time.Time
}

func New(app *application.Service, options Options) http.Handler {
	if options.Log == nil {
		options.Log = slog.Default()
	}
	s := &Server{app: app, options: options, passwordSlots: make(chan struct{}, 2), started: time.Now()}
	s.limiter.items = map[string]attempts{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/setup", s.setupStatus)
	mux.HandleFunc("POST /api/v1/setup", s.setup)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("GET /api/v1/auth/me", s.auth(false, s.me))
	mux.HandleFunc("POST /api/v1/auth/logout", s.auth(false, s.logout))
	mux.HandleFunc("PATCH /api/v1/auth/me", s.auth(false, s.updateProfile))
	mux.HandleFunc("POST /api/v1/auth/change-password", s.auth(false, s.changePassword))
	mux.HandleFunc("PATCH /api/v1/company", s.auth(true, s.updateCompany))
	mux.HandleFunc("GET /api/v1/products", s.auth(false, s.products))
	mux.HandleFunc("GET /api/v1/products/lookup", s.auth(false, s.lookupBarcode))
	mux.HandleFunc("POST /api/v1/products/{id}/barcodes/generate", s.auth(true, s.generateBarcode))
	mux.HandleFunc("POST /api/v1/products", s.auth(true, s.createProduct))
	mux.HandleFunc("GET /api/v1/products/{id}", s.auth(false, s.product))
	mux.HandleFunc("PATCH /api/v1/products/{id}", s.auth(true, s.updateProduct))
	mux.HandleFunc("GET /api/v1/inventory/stocks", s.auth(false, s.products))
	mux.HandleFunc("POST /api/v1/inventory/receipts", s.auth(true, s.receive))
	mux.HandleFunc("POST /api/v1/inventory/adjustments", s.auth(true, s.adjustStock))
	mux.HandleFunc("GET /api/v1/promotions", s.auth(true, s.promotions))
	mux.HandleFunc("POST /api/v1/promotions", s.auth(true, s.savePromotion))
	mux.HandleFunc("GET /api/v1/promotions/{id}", s.auth(true, s.promotion))
	mux.HandleFunc("PATCH /api/v1/promotions/{id}", s.auth(true, s.savePromotion))
	mux.HandleFunc("GET /api/v1/sales", s.auth(false, s.sales))
	mux.HandleFunc("GET /api/v1/sales/{id}", s.auth(false, s.sale))
	mux.HandleFunc("POST /api/v1/sales", s.auth(false, s.confirmSale))
	mux.HandleFunc("POST /api/v1/sales/quote", s.auth(false, s.quoteSale))
	mux.HandleFunc("GET /api/v1/cash/movements", s.auth(false, s.cash))
	mux.HandleFunc("GET /api/v1/cash/movements/{id}", s.auth(false, s.cashMovement))
	mux.HandleFunc("POST /api/v1/cash/expenses", s.auth(false, s.cashExpense))
	mux.HandleFunc("GET /api/v1/reports/summary", s.auth(false, s.summary))
	mux.HandleFunc("GET /api/v1/users", s.auth(true, s.users))
	mux.HandleFunc("POST /api/v1/users", s.auth(true, s.createUser))
	mux.HandleFunc("PATCH /api/v1/users/{id}", s.auth(true, s.setUserActive))
	mux.HandleFunc("GET /api/v1/audit", s.auth(true, s.audit))
	mux.HandleFunc("GET /api/v1/audit/operations", s.auth(true, s.auditOperations))
	mux.HandleFunc("GET /api/v1/audit/{id}", s.auth(true, s.auditDetail))
	mux.HandleFunc("GET /api/v1/pos/settings", s.auth(false, s.posSettings))
	mux.HandleFunc("PUT /api/v1/pos/settings", s.auth(true, s.savePOSSettings))
	mux.HandleFunc("GET /api/v1/pos/devices", s.auth(true, s.posDevices))
	mux.HandleFunc("GET /api/v1/pos/jobs", s.auth(true, s.posJobs))
	mux.HandleFunc("GET /api/v1/pos/jobs/{id}", s.auth(false, s.posJob))
	mux.HandleFunc("POST /api/v1/pos/tests", s.auth(true, s.posRequest))
	mux.HandleFunc("POST /api/v1/pos/verify", s.auth(true, s.posVerify))
	mux.HandleFunc("POST /api/v1/pos/drawer/open", s.auth(true, s.posRequest))
	mux.HandleFunc("POST /api/v1/sales/{id}/print", s.auth(false, s.posRequest))
	mux.HandleFunc("POST /api/v1/system/backups", s.auth(true, s.backup))
	mux.HandleFunc("GET /api/v1/system/diagnostics", s.auth(true, s.diagnostics))
	mux.HandleFunc("GET /api/v1/openapi.json", s.auth(true, func(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(options.OpenAPI)
	}))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { s.fail(w, r, core.NotFound) })
	if options.UI != nil {
		mux.Handle("/", options.UI)
	}
	return s.middleware(mux)
}
func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	code := "internal"
	message := "No fue posible completar la operación. Revisa el diagnóstico del servidor."
	var known *core.Error
	if errors.As(err, &known) {
		code = known.Code
		message = known.Message
		switch code {
		case "validation":
			status = 400
		case "unauthorized":
			status = 401
		case "forbidden":
			status = 403
		case "not_found":
			status = 404
		case "conflict":
			status = 409
		case "busy":
			status = 503
		case "rate_limit":
			status = 429
		}
	}
	if status == 500 {
		s.options.Log.Error("request_failed", "requestId", w.Header().Get("X-Request-ID"), "error", err)
	}
	jsonResponse(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "requestId": w.Header().Get("X-Request-ID")}})
}
func decode(r *http.Request, w http.ResponseWriter, target any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return core.Invalid("Envía el contenido como application/json.")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return core.Invalid("El contenido JSON no es válido o contiene campos desconocidos.")
	}
	if err = d.Decode(&struct{}{}); err != io.EOF {
		return core.Invalid("Envía un único objeto JSON.")
	}
	return nil
}
func query(r *http.Request) (core.Query, error) {
	q := core.Query{Search: r.URL.Query().Get("search"), Page: 1, PageSize: 20}
	q.SellableOnly = r.URL.Query().Get("sellable") == "true"
	q.OwnOnly = r.URL.Query().Get("stockMode") == "own"
	q.Action = r.URL.Query().Get("action")
	for name, target := range map[string]*int{"page": &q.Page, "pageSize": &q.PageSize} {
		if text := r.URL.Query().Get(name); text != "" {
			n, err := strconv.Atoi(text)
			if err != nil || n < 1 || n > 1_000_000 || (name == "pageSize" && n > 100) {
				return q, core.Invalid("Paginación inválida.")
			}
			*target = n
		}
	}
	if len(q.Search) > 160 {
		return q, core.Invalid("La búsqueda es demasiado larga.")
	}
	return q, nil
}
func local(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type action func(http.ResponseWriter, *http.Request, admin.User, admin.Session)

func (s *Server) auth(adminOnly bool, next action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("erp_session")
		if err != nil {
			s.fail(w, r, core.Unauthorized)
			return
		}
		user, session, err := s.app.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if adminOnly && user.Role != "admin" {
			s.fail(w, r, core.Forbidden)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(session.CSRFToken)) != 1 {
			s.fail(w, r, core.Forbidden)
			return
		}
		next(w, r, user, session)
	}
}
func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	_, err := s.app.Store.Read().Admin.Company(r.Context())
	if err != nil && !core.HasCode(err, "not_found") {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"initialized": err == nil, "localSetupAllowed": local(r)})
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if !local(r) {
		s.fail(w, r, core.Forbidden)
		return
	}
	token, err := os.ReadFile(filepath.Join(s.options.DataDir, "setup.token"))
	if err != nil || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(string(token))), []byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Setup "))) != 1 {
		s.fail(w, r, core.Forbidden)
		return
	}
	var body application.SetupRequest
	if err = decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
	default:
		s.fail(w, r, &core.Error{Code: "busy", Message: "Espera a que termine el alta inicial."})
		return
	}
	if err = s.app.Setup(r.Context(), body); err != nil {
		s.fail(w, r, err)
		return
	}
	if err = os.Remove(filepath.Join(s.options.DataDir, "setup.token")); err != nil {
		s.options.Log.Warn("setup_token_cleanup", "error", err)
	}
	jsonResponse(w, 201, map[string]bool{"initialized": true})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.allow(host) {
		w.Header().Set("Retry-After", "60")
		s.fail(w, r, &core.Error{Code: "rate_limit", Message: "Demasiados intentos. Vuelve a intentarlo en un minuto."})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
	default:
		s.fail(w, r, &core.Error{Code: "busy", Message: "El acceso está ocupado. Inténtalo nuevamente."})
		return
	}
	result, err := s.app.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		if core.HasCode(err, "unauthorized") {
			err = &core.Error{Code: "unauthorized", Message: "Usuario o contraseña incorrectos."}
		}
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "erp_session", Value: result.Token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, Expires: result.Expires, MaxAge: 12 * 60 * 60})
	jsonResponse(w, 200, result)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request, u admin.User, session admin.Session) {
	company, err := s.app.Store.Read().Admin.Company(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"user": u, "company": company, "csrfToken": session.CSRFToken})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request, _ admin.User, session admin.Session) {
	if err := s.app.Logout(r.Context(), session); err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "erp_session", Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (s *Server) products(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, err := query(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.app.Store.Read().Reports.Products(r.Context(), q)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 200, p)
}
func (s *Server) createProduct(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body catalog.Product
	if err := decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.app.CreateProduct(r.Context(), u, body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.productResponse(w, r, p.ID, 201)
}
func (s *Server) receive(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		ProductID string `json:"productId"`
		Quantity  string `json:"quantity"`
		Reason    string `json:"reason"`
	}
	if err := decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	receipt, err := s.app.ReceiveStock(r.Context(), u, inventory.Receipt{ProductID: body.ProductID, Quantity: body.Quantity, Reason: body.Reason})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 201, receipt)
}
func (s *Server) sales(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, e := query(r)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	p, e := s.app.Store.Read().Reports.Sales(r.Context(), q)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, p)
}
func (s *Server) sale(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	sale, err := s.app.Store.Read().Sales.Sale(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 200, sale)
}
func (s *Server) confirmSale(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body sales.Request
	if err := decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	sale, replayed, err := s.app.ConfirmSale(r.Context(), u, r.Header.Get("Idempotency-Key"), body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := 201
	if replayed {
		status = 200
		w.Header().Set("Idempotency-Replayed", "true")
	}
	jsonResponse(w, status, sale)
}
func (s *Server) cash(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, e := query(r)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	q.Kind = r.URL.Query().Get("kind")
	if q.Kind != "" && q.Kind != "sale" && q.Kind != "expense" {
		s.fail(w, r, core.Invalid("Selecciona ingresos o egresos."))
		return
	}
	p, e := s.app.Store.Read().Reports.Cash(r.Context(), q)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, p)
}
func (s *Server) summary(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	result, err := s.app.Summary(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 200, result)
}
func (s *Server) users(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, e := query(r)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	p, e := s.app.Store.Read().Admin.Users(r.Context(), q)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, p)
}
func (s *Server) createUser(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body application.CreateUserRequest
	if err := decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	user, err := s.app.CreateUser(r.Context(), u, body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 201, user)
}
func (s *Server) setUserActive(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		Active                  *bool `json:"active"`
		CanApplyManualDiscounts *bool `json:"canApplyManualDiscounts"`
		CanRecordCashExpenses   *bool `json:"canRecordCashExpenses"`
	}
	if err := decode(r, w, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	count := 0
	for _, field := range []*bool{body.Active, body.CanApplyManualDiscounts, body.CanRecordCashExpenses} {
		if field != nil {
			count++
		}
	}
	if count != 1 {
		s.fail(w, r, core.Invalid("Modifica un solo estado o permiso en cada petición."))
		return
	}
	if body.CanRecordCashExpenses != nil {
		if err := s.app.SetExpensePermission(r.Context(), u, r.PathValue("id"), *body.CanRecordCashExpenses); err != nil {
			s.fail(w, r, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	if body.CanApplyManualDiscounts != nil {
		if err := s.app.SetDiscountPermission(r.Context(), u, r.PathValue("id"), *body.CanApplyManualDiscounts); err != nil {
			s.fail(w, r, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	if err := s.app.SetUserActive(r.Context(), u, r.PathValue("id"), *body.Active); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, e := query(r)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	var p core.Page[audit.Entry]
	e = s.app.Store.ReadSnapshot(r.Context(), func(ports application.Ports) error {
		var err error
		p, err = ports.Reports.Audit(r.Context(), q)
		return err
	})
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, p)
}
func (s *Server) backup(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	if s.options.Backup == nil {
		s.fail(w, r, fmt.Errorf("backup unavailable"))
		return
	}
	path, err := s.options.Backup(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	jsonResponse(w, 201, map[string]string{"filename": filepath.Base(path)})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if s.options.Ping != nil {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := s.options.Ping(ctx); err != nil {
			s.fail(w, r, &core.Error{Code: "busy", Message: "El servidor no está listo para recibir solicitudes."})
			return
		}
	}
	jsonResponse(w, 200, map[string]string{"status": "ok", "version": s.options.Version})
}
func (s *Server) diagnostics(w http.ResponseWriter, _ *http.Request, _ admin.User, _ admin.Session) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	jsonResponse(w, 200, map[string]any{"version": s.options.Version, "uptimeSeconds": int64(time.Since(s.started).Seconds()), "requests": s.requests.Load(), "heapBytes": m.Alloc, "runtimeBytes": m.Sys})
}

type attempts struct {
	count int
	start time.Time
}
type rateLimiter struct {
	mu    sync.Mutex
	items map[string]attempts
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, a := range l.items {
		if now.Sub(a.start) > time.Minute {
			delete(l.items, k)
		}
	}
	a, ok := l.items[key]
	if !ok {
		if len(l.items) >= 2048 {
			return false
		}
		a = attempts{start: now}
	}
	a.count++
	l.items[key] = a
	return a.count <= 10
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(n int) {
	if r.status == 0 {
		r.status = n
		r.ResponseWriter.WriteHeader(n)
	}
}
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(200)
	}
	return r.ResponseWriter.Write(b)
}
func sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(u.Host, r.Host) && u.Scheme == scheme
}
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w}
		id := core.ID()
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		s.requests.Add(1)
		defer func() {
			if recovered := recover(); recovered != nil {
				s.options.Log.Error("panic", "requestId", id, "panic", recovered)
				if rec.status == 0 {
					s.fail(rec, r, fmt.Errorf("internal panic"))
				}
			}
			s.options.Log.Info("http", "requestId", id, "method", r.Method, "path", r.URL.Path, "status", rec.status, "durationMs", time.Since(start).Milliseconds())
		}()
		if r.Method != "GET" && r.Method != "HEAD" && !sameOrigin(r) {
			s.fail(rec, r, core.Forbidden)
			return
		}
		next.ServeHTTP(rec, r)
	})
}
