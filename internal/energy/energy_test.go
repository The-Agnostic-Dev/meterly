package energy

import "testing"

func TestStringAndParse(t *testing.T) {
	if got := KWh(120450).String(); got != "1,204.50" {
		t.Fatalf("String = %q", got)
	}
	if got := KWh(9820).String(); got != "98.20" {
		t.Fatalf("String = %q", got)
	}
	for in, want := range map[string]KWh{"15230.4": 1523040, "15,230.40": 1523040, "12 kWh": 1200} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v", in, got, err)
		}
	}
	if _, err := Parse("-4"); err == nil {
		t.Error("negative readings should be refused")
	}
}
