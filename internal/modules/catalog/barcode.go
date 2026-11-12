package catalog

import (
	"business-manage/internal/core"
	"strings"
)

type Barcode struct {
	Value     string `json:"value"`
	Format    string `json:"format"`
	Generated bool   `json:"generated"`
	Primary   bool   `json:"primary"`
}

func (b *Barcode) Validate() error {
	b.Value = strings.TrimSpace(b.Value)
	b.Format = strings.ToUpper(b.Format)
	if b.Format == "" {
		b.Format = "CODE128"
	}
	if len(b.Value) < 1 || len(b.Value) > 64 {
		return core.Invalid("El código de barras debe tener entre 1 y 64 caracteres.")
	}
	for _, c := range b.Value {
		if c < 32 || c > 126 {
			return core.Invalid("El código de barras debe contener caracteres ASCII imprimibles.")
		}
	}
	length := 0
	switch b.Format {
	case "CODE128":
		return nil
	case "EAN8":
		length = 8
	case "EAN13":
		length = 13
	case "UPC":
		length = 12
	default:
		return core.Invalid("Formato de código de barras no compatible.")
	}
	if len(b.Value) != length || strings.Trim(b.Value, "0123456789") != "" {
		return core.Invalid("Longitud o caracteres inválidos para el formato seleccionado.")
	}
	sum, weight := 0, 3
	for i := len(b.Value) - 2; i >= 0; i-- {
		sum += int(b.Value[i]-'0') * weight
		weight = 4 - weight
	}
	if (10-sum%10)%10 != int(b.Value[len(b.Value)-1]-'0') {
		return core.Invalid("El dígito de control del código de barras no es válido.")
	}
	return nil
}

func ValidateBarcodes(codes []Barcode) error {
	if len(codes) > 30 {
		return core.Invalid("Un producto admite hasta treinta códigos de barras.")
	}
	seen := map[string]bool{}
	primary, generated := 0, 0
	for i := range codes {
		if e := codes[i].Validate(); e != nil {
			return e
		}
		if seen[codes[i].Value] {
			return core.Invalid("Hay códigos de barras repetidos.")
		}
		seen[codes[i].Value] = true
		if codes[i].Primary {
			primary++
		}
		if codes[i].Generated {
			generated++
		}
	}
	if len(codes) > 0 && primary == 0 {
		codes[0].Primary = true
	}
	if primary > 1 || generated > 1 {
		return core.Invalid("Solo puede haber un código principal y un código generado por producto.")
	}
	return nil
}
