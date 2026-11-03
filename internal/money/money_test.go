package money

import "testing"

func TestString(t *testing.T) {
	cases := map[Kobo]string{
		0:          "₦0.00",
		5:          "₦0.05",
		1250000:    "₦12,500.00",
		123456789:  "₦1,234,567.89",
		-250050:    "-₦2,500.50",
		Naira(999): "₦999.00",
	}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("Kobo(%d).String() = %q, want %q", int64(in), got, want)
		}
	}
}

func TestParse(t *testing.T) {
	cases := map[string]Kobo{
		"5000":        500000,
		"5,000":       500000,
		"₦12,500.50":  1250050,
		" N10000 ":    1000000,
		"NGN 2,000":   200000,
		"0.1":         10,
		"1,000,000.5": 100000050,
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "₦", "12a"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

func TestSplitVAT(t *testing.T) {
	net, vat := SplitVAT(Naira(10_000))
	if net != 930233 || vat != 69767 {
		t.Fatalf("SplitVAT(10,000) = %v + %v", net, vat)
	}
	if net+vat != Naira(10_000) {
		t.Fatal("net and VAT must add back to the total")
	}
}
