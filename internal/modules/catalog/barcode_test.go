package catalog_test

import (
	"business-manage/internal/modules/catalog"
	"testing"
)

func TestBarcodeChecksumsAndTextIdentity(t *testing.T) {
	for _, test := range []struct {
		value, format string
		valid         bool
	}{{"4006381333931", "EAN13", true}, {"4006381333932", "EAN13", false}, {"96385074", "EAN8", true}, {"036000291452", "UPC", true}, {"036000291453", "UPC", false}, {"000000000123", "CODE128", true}, {"A\x1bp", "CODE128", false}, {"123", "EAN13", false}} {
		b := catalog.Barcode{Value: test.value, Format: test.format}
		e := b.Validate()
		if (e == nil) != test.valid {
			t.Fatal(test, e)
		}
		if e == nil && b.Value != test.value {
			t.Fatal("identity changed")
		}
	}
}
