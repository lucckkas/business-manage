package audit

import (
	"encoding/json"
	"strings"
)

type Operation struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	SubjectType string `json:"subjectType"`
}

var Operations = []Operation{
	{"company.setup", "Configuración de empresa", "company"},
	{"company.update", "Cambio de datos de empresa", "company"},
	{"session.login", "Inicio de sesión", "user"},
	{"user.create", "Creación de usuario", "user"},
	{"user.active", "Cambio de acceso", "user"},
	{"user.password_reset_local", "Recuperación de contraseña", "user"},
	{"user.password_change", "Cambio de contraseña", "user"},
	{"user.profile", "Edición del perfil", "user"},
	{"user.discount_permission", "Permiso de descuentos manuales", "user"},
	{"user.expense_permission", "Permiso de egresos de caja", "user"},
	{"cash.expense", "Registro de egreso de caja", "cash"},
	{"product.create", "Creación de producto", "product"},
	{"product.update", "Edición de producto", "product"},
	{"product.barcode_generate", "Generación de código de barras", "product"},
	{"inventory.receive", "Ingreso de existencias", "product"},
	{"inventory.adjust", "Ajuste de existencias", "product"},
	{"promotion.create", "Creación de oferta", "promotion"},
	{"promotion.update", "Cambio de oferta", "promotion"},
	{"sale.confirm", "Confirmación de venta", "sale"},
	{"sale.manual_discount", "Descuento manual aplicado", "sale"},
	{"pos.settings", "Configuración de periféricos", "printer"},
	{"pos.enqueue", "Solicitud de impresión o apertura", "printer"},
	{"pos.result", "Resultado de impresión o apertura", "printer"},
	{"pos.recover", "Recuperación de trabajo interrumpido", "printer"},
}

func Metadata(code string) Operation {
	for _, o := range Operations {
		if o.Code == code {
			return o
		}
	}
	return Operation{code, code, ""}
}
func Fold(s string) string {
	return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(strings.ToLower(strings.TrimSpace(s)))
}
func Scrub(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range v {
			key := Fold(k)
			if strings.Contains(key, "password") || strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "hash") || key == "payload" {
				continue
			}
			out[k] = Scrub(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = Scrub(item)
		}
		return out
	}
	return value
}
func SafeDetails(details map[string]any) map[string]any {
	raw, _ := json.Marshal(details)
	var mapped map[string]any
	_ = json.Unmarshal(raw, &mapped)
	if mapped == nil {
		return map[string]any{}
	}
	return Scrub(mapped).(map[string]any)
}
