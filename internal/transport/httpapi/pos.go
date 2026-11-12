package httpapi

import (
	"business-manage/internal/application"
	"business-manage/internal/core"
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/reporting"
	"business-manage/internal/modules/sales"
	"net/http"
)

func (s *Server) lookupBarcode(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	var product reporting.Product
	e := s.app.Store.ReadSnapshot(r.Context(), func(p application.Ports) error {
		v, e := p.Catalog.ProductByBarcode(r.Context(), r.URL.Query().Get("barcode"))
		if e != nil {
			return e
		}
		product, e = p.Reports.ProductView(r.Context(), v.ID)
		return e
	})
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, product)
}
func (s *Server) generateBarcode(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	p, e := s.app.GenerateBarcode(r.Context(), u, r.PathValue("id"))
	if e != nil {
		s.fail(w, r, e)
		return
	}
	s.productResponse(w, r, p.ID, 200)
}
func (s *Server) auditOperations(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	jsonResponse(w, 200, audit.Operations)
}
func (s *Server) auditDetail(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	var response struct {
		Entry audit.Entry `json:"entry"`
		Sale  *sales.Sale `json:"sale,omitempty"`
	}
	e := s.app.Store.ReadSnapshot(r.Context(), func(p application.Ports) error {
		var e error
		response.Entry, e = p.Audit.Entry(r.Context(), r.PathValue("id"))
		if e != nil {
			return e
		}
		if response.Entry.SubjectType == "sale" {
			sale, e := p.Sales.Sale(r.Context(), response.Entry.SubjectID)
			if e == nil {
				response.Sale = &sale
			} else if !core.HasCode(e, "not_found") {
				return e
			}
		}
		return nil
	})
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, response)
}
func (s *Server) posSettings(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	cfg, e := s.app.Store.Read().POS.Settings(r.Context())
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, cfg)
}
func (s *Server) savePOSSettings(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var cfg pos.Settings
	if e := decode(r, w, &cfg); e != nil {
		s.fail(w, r, e)
		return
	}
	cfg, e := s.app.SavePOSSettings(r.Context(), u, cfg)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, cfg)
}
func (s *Server) posDevices(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	devices, e := s.app.PrinterDevices(r.Context())
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, devices)
}
func (s *Server) posJobs(w http.ResponseWriter, r *http.Request, _ admin.User, _ admin.Session) {
	q, e := query(r)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jobs, e := s.app.Store.Read().POS.Jobs(r.Context(), q, r.URL.Query().Get("saleId"))
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, jobs)
}
func (s *Server) posJob(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	job, e := s.app.Store.Read().POS.Job(r.Context(), r.PathValue("id"))
	if e != nil {
		s.fail(w, r, e)
		return
	}
	if u.Role != "admin" && job.SaleID == "" && job.CashMovementID == "" {
		s.fail(w, r, core.Forbidden)
		return
	}
	jsonResponse(w, 200, job)
}
func (s *Server) posRequest(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	kind, sale := body.Kind, ""
	if r.PathValue("id") != "" {
		kind = "copy"
		sale = r.PathValue("id")
	} else if r.URL.Path == "/api/v1/pos/drawer/open" {
		kind = "drawer"
	} else if kind != "test_print" && kind != "test_drawer" {
		s.fail(w, r, core.Invalid("Selecciona una prueba de ticket o cajón."))
		return
	}
	job, replayed, e := s.app.RequestPrint(r.Context(), u, r.Header.Get("Idempotency-Key"), kind, sale, body.Reason)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	status := 202
	if replayed {
		status = 200
		w.Header().Set("Idempotency-Replayed", "true")
	}
	jsonResponse(w, status, job)
}
func (s *Server) posVerify(w http.ResponseWriter, r *http.Request, u admin.User, _ admin.Session) {
	var body struct {
		JobID string `json:"jobId"`
	}
	if e := decode(r, w, &body); e != nil {
		s.fail(w, r, e)
		return
	}
	cfg, e := s.app.VerifyPOSTest(r.Context(), u, body.JobID)
	if e != nil {
		s.fail(w, r, e)
		return
	}
	jsonResponse(w, 200, cfg)
}
