package core

import "testing"

func TestExactQuantitiesAndRounding(t *testing.T) {
	for _, value := range []string{"NaN", "1e3", "-1", "+1", "1.0001", " 1", ".5", "1."} {
		if _, err := Quantity(value); err == nil {
			t.Errorf("accepted quantity %q", value)
		}
	}
	q, err := Quantity("0001.125")
	if err != nil || q != 1125 {
		t.Fatalf("quantity: %d %v", q, err)
	}
	for _, test := range []struct{ price, quantity, want int64 }{{101, 500, 51}, {1250, 1125, 1406}, {1, 499, 0}, {1, 500, 1}, {999999999999999, 1000, 999999999999999}} {
		got, err := LineTotal(test.price, test.quantity)
		if err != nil || got != test.want {
			t.Errorf("line %+v got %d, %v", test, got, err)
		}
	}
	if _, err = LineTotal(1_000_000_000_000_000, 1_000_000_000_000); err == nil {
		t.Fatal("overflow accepted")
	}
	for _, value := range []string{"1.2", "-1", "+1", "1e6", "", "1000000000000001"} {
		if _, err := Minor(value); err == nil {
			t.Errorf("accepted money %q", value)
		}
	}
}
