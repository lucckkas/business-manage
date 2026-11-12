package httpapi

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/inventory"
	"business-manage/internal/modules/promotions"
	"business-manage/internal/modules/reporting"
	"business-manage/internal/modules/sales"
	"net"
	"net/http"
)

func (s *Server) product(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	s.productResponse(w, r, r.PathValue("id"), 200)
}
func (s *Server) productResponse(w http.ResponseWriter, r *http.Request, id string, status int) {
	var view reporting.Product
	e := s.app.Store.ReadSnapshot(r.Context(), func(p application.Ports) error {
		var err error
		view, err = p.Reports.ProductView(r.Context(), id)
		return err
	})
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, status, view)
}
func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body application.ProductUpdate
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	p, e := s.app.UpdateProduct(r.Context(), u, r.PathValue("id"), body)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	s.productResponse(w, r, p.ID, 200)
}
func (s *Server) adjustStock(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body inventory.Adjustment
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	a, e := s.app.AdjustStock(r.Context(), u, body)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 201, a)
}
func (s *Server) quoteSale(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body sales.Request
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	q, e := s.app.QuoteSale(r.Context(), u, body)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, q)
}
func (s *Server) promotions(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, e := query(r)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	rules, e := s.app.Store.Read().Promotions.Rules(r.Context(), q)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, rules)
}
func (s *Server) promotion(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	rule, e := s.app.Store.Read().Promotions.Rule(r.Context(), r.PathValue("id"))
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, rule)
}
func (s *Server) savePromotion(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body promotions.Rule
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	rule, e := s.app.SavePromotion(r.Context(), u, r.PathValue("id"), body)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	jsonResponse(w, status, rule)
}
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		Name string `json:"name"`
	}
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	user, e := s.app.UpdateProfile(r.Context(), u, body.Name)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, user)
}
func (s *Server) updateCompany(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
	}
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	c, e := s.app.UpdateCompany(r.Context(), u, body.Name, body.Timezone)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, c)
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		CurrentPassword   string `json:"currentPassword"`
		NewPassword       string `json:"newPassword"`
		AllowWeakPassword bool   `json:"allowWeakPassword"`
	}
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.allow("password:" + host) {
		s.fail(w, r, &core.Error{Code: "rate_limit", Message: "Demasiados intentos; espera un minuto."})
		return
	}
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
	default:
		s.fail(w, r, &core.Error{Code: "busy", Message: "Espera un momento antes de cambiar la contraseña."})
		return
	}
	if e := s.app.ChangePassword(r.Context(), u, body.CurrentPassword, body.NewPassword, body.AllowWeakPassword); e != nil {
		s.fail(w, r, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "erp_session", Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
