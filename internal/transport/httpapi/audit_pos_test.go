package httpapi_test

import (
	"business-manage/internal/modules/audit"
	"business-manage/internal/modules/catalog"
	"encoding/json"
	"testing"
)

func TestAuditOperationsExactFilterAndBarcodeLookup(t *testing.T) {
	f := fixtureFor(t, true)
	session := f.login(t)
	body := map[string]any{"name": "Producto API", "sku": "API-BAR", "unit": "un", "priceMinor": "100", "barcodes": []map[string]any{{"value": "000987654321", "format": "CODE128", "primary": true}}}
	w := f.request("POST", "/api/v1/products", body, session.Token, session.CSRFToken, nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var product catalog.Product
	if e := json.Unmarshal(w.Body.Bytes(), &product); e != nil {
		t.Fatal(e)
	}
	body["name"] = "Nombre actualizado"
	body["version"] = product.Version
	w = f.request("PATCH", "/api/v1/products/"+product.ID, body, session.Token, session.CSRFToken, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/v1/audit?search=Ed", "/api/v1/audit?search=EDICI%C3%93N&action=product.update"} {
		w = f.request("GET", path, nil, session.Token, "", nil)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var page struct {
			Items []audit.Entry `json:"items"`
			Total int           `json:"total"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &page); e != nil {
			t.Fatal(e)
		}
		if page.Total != 1 || page.Items[0].Label != "Edición de producto" {
			t.Fatal(w.Body.String())
		}
		w = f.request("GET", "/api/v1/audit/"+page.Items[0].ID, nil, session.Token, "", nil)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w = f.request("GET", "/api/v1/products/lookup?barcode=000987654321", nil, session.Token, "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var lookup catalog.Product
	json.Unmarshal(w.Body.Bytes(), &lookup)
	if lookup.ID != product.ID {
		t.Fatal(lookup)
	}
	w = f.request("GET", "/api/v1/audit/operations", nil, session.Token, "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = f.request("POST", "/api/v1/pos/drawer/open", map[string]string{"reason": "Prueba sin CSRF"}, session.Token, "", nil)
	if w.Code != 403 {
		t.Fatal("drawer did not require CSRF", w.Code)
	}
}
