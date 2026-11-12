package pos_test

import (
	"business-manage/internal/modules/admin"
	"business-manage/internal/modules/pos"
	"business-manage/internal/modules/sales"
	"bytes"
	"testing"
)

func TestReceiptSanitizesControlsAndKeepsExactAmounts(t *testing.T) {
	sale := sales.Sale{Company: admin.Company{Name: "Negocio\x1bp", Currency: "USD", CurrencyScale: 2, Timezone: "UTC"}, Number: 12, Result: sales.Result{SubtotalMinor: "123456789012345", TotalMinor: "123456789012345", PromotionDiscountMinor: "0", ManualDiscountMinor: "0", Lines: []sales.Line{{Name: "Café\x1bp", Quantity: "1.250", PriceMinor: "100", TotalMinor: "125"}}}}
	cfg := pos.Defaults()
	data := pos.Receipt(sale, cfg, true)
	if bytes.Contains(data, []byte{27, 'p'}) || !bytes.Contains(data, []byte("1234567890123.45")) || !bytes.Contains(data, []byte("COPIA")) {
		t.Fatal(data)
	}
}
