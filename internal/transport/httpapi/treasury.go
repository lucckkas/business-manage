package httpapi

import (
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/treasury"
	"net/http"
)

func (s *Server) cashMovement(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	m, e := s.app.Store.Read().Treasury.Movement(r.Context(), r.PathValue("id"))
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, m)
}
func (s *Server) cashExpense(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body treasury.ExpenseRequest
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	m, replayed, e := s.app.RecordCashExpense(r.Context(), u, r.Header.Get("Idempotency-Key"), body)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	status := 201
	if replayed {
		status = 200
		w.Header().Set("Idempotency-Replayed", "true")
	}
	jsonResponse(w, status, m)
}
