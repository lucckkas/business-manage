package httpapi_test

import (
	"encoding/json"
	"testing"
)

func TestProductResponsesIncludeExactInventoryAndTrackingState(t *testing.T) {
	f := fixtureFor(t, true)
	session := f.login(t)
	for _, policy := range []string{"block", "negative", "untracked"} {
		body := map[string]any{"name": "Producto " + policy, "sku": policy, "unit": "un", "priceMinor": "100", "sellable": true, "stockMode": "own", "stockPolicy": policy, "components": []any{}}
		w := f.request("POST", "/api/v1/products", body, session.Token, session.CSRFToken, nil)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var product map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &product); e != nil {
			t.Fatal(e)
		}
		if _, exists := product["stock"]; !exists {
			t.Fatal("create response omitted stock")
		}
		id := product["id"].(string)
		if policy == "negative" {
			w = f.request("POST", "/api/v1/inventory/adjustments", map[string]string{"productId": id, "countedQuantity": "-1.250001", "reason": "Conteo verificado"}, session.Token, session.CSRFToken, nil)
			if w.Code != 201 {
				t.Fatal(w.Code, w.Body.String())
			}
		}
		w = f.request("GET", "/api/v1/products/"+id, nil, session.Token, "", nil)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if e := json.Unmarshal(w.Body.Bytes(), &product); e != nil {
			t.Fatal(e)
		}
		if _, exists := product["availableQuantity"]; !exists {
			t.Fatal("read response omitted availability")
		}
		if policy == "untracked" {
			if product["stock"] != nil || product["availableQuantity"] != nil {
				t.Fatal(product)
			}
		} else {
			expected := "0.000000"
			if policy == "negative" {
				expected = "-1.250001"
			}
			if product["stock"] != expected {
				t.Fatal(product)
			}
		}
	}
}
