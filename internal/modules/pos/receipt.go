package pos

import (
	"business-manage/internal/modules/sales"
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"
)

// Text never includes printer control bytes from user-entered names or SKUs.
// The profile selects CP858 for Spanish and keeps rendering independent of OS.
func text(s string) []byte {
	var b bytes.Buffer
	cp := map[rune]byte{'á': 160, 'é': 130, 'í': 161, 'ó': 162, 'ú': 163, 'ñ': 164, 'Ñ': 165, 'ü': 129, 'Ü': 154, 'Á': 181, 'É': 144, 'Í': 214, 'Ó': 224, 'Ú': 233, '¿': 168, '¡': 173, '€': 213}
	for _, r := range s {
		if c, ok := cp[r]; ok {
			b.WriteByte(c)
		} else if r >= 32 && r < 127 {
			b.WriteByte(byte(r))
		} else if unicode.IsSpace(r) {
			b.WriteByte(' ')
		} else {
			b.WriteByte('?')
		}
	}
	return b.Bytes()
}
func amount(value string, scale int) string {
	n, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return "?"
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, factor, r)
	if scale == 0 {
		return q.String()
	}
	return fmt.Sprintf("%s.%0*s", q.String(), scale, r.String())
}
func Drawer(s Settings) []byte {
	return []byte{27, 'p', byte(s.DrawerPin), byte(s.PulseOnMS / 2), byte(s.PulseOffMS / 2)}
}
func Receipt(s sales.Sale, cfg Settings, copy bool) []byte {
	var b bytes.Buffer
	b.Write([]byte{27, '@', 27, 't', 19, 27, 'a', 1})
	line := func(v string) { b.Write(text(v)); b.WriteByte('\n') }
	line(s.Company.Name)
	if copy {
		line("COPIA - REIMPRESION")
	}
	line(fmt.Sprintf("Venta #%05d", s.Number))
	at, e := time.Parse(time.RFC3339Nano, s.CreatedAt)
	zone, ze := time.LoadLocation(s.Company.Timezone)
	if e == nil && ze == nil {
		line(at.In(zone).Format("02/01/2006 15:04:05"))
	}
	b.Write([]byte{27, 'a', 0})
	width := 42
	if cfg.Width == 58 {
		width = 30
	}
	line(strings.Repeat("-", width))
	wrap := func(v string) {
		chars := []rune(v)
		for len(chars) > width {
			line(string(chars[:width]))
			chars = chars[width:]
		}
		if len(chars) > 0 {
			line(string(chars))
		}
	}
	for _, l := range s.Lines {
		wrap(l.Name)
		label := strings.TrimRight(strings.TrimRight(l.Quantity, "0"), ".")
		if !strings.Contains(l.Quantity, ".") {
			label = l.Quantity
		}
		if l.Kind == "gift" {
			label += " " + l.Unit + " - REGALO"
		} else {
			label += " " + l.Unit + " x " + amount(l.PriceMinor, s.Company.CurrencyScale)
		}
		wrap(label + " = " + amount(l.TotalMinor, s.Company.CurrencyScale))
	}
	line(strings.Repeat("-", width))
	line("Subtotal: " + amount(s.SubtotalMinor, s.Company.CurrencyScale))
	line("Ofertas: -" + amount(s.PromotionDiscountMinor, s.Company.CurrencyScale))
	line("Descuentos: -" + amount(s.ManualDiscountMinor, s.Company.CurrencyScale))
	line("TOTAL " + s.Company.Currency + " " + amount(s.TotalMinor, s.Company.CurrencyScale))
	line("Pago al contado")
	wrap("Comprobante interno sin validez tributaria")
	b.Write([]byte{'\n', '\n', '\n'})
	if cfg.Cutter {
		b.Write([]byte{29, 'V', 66, 0})
	}
	return b.Bytes()
}
func TestTicket(cfg Settings) []byte {
	var b bytes.Buffer
	b.Write([]byte{27, '@', 27, 't', 19})
	b.Write(text(fmt.Sprintf("RAIZ ERP - PRUEBA %d mm", cfg.Width)))
	b.WriteByte('\n')
	b.Write(text("Español: áéíóú ñ Ñ ü ¿¡"))
	b.WriteByte('\n')
	b.Write(text("0123456789 - sin apertura de cajon"))
	b.Write([]byte{'\n', '\n', '\n'})
	if cfg.Cutter {
		b.Write([]byte{29, 'V', 66, 0})
	}
	return b.Bytes()
}
