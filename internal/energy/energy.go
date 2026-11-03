// Package energy holds electricity quantities. Energy is stored in hundredths
// of a kilowatt-hour so readings and units add up exactly.
package energy

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// KWh is an amount of energy in hundredths of a kWh (1 = 0.01 kWh).
type KWh int64

// FromFloat converts a float kWh value, rounding to the nearest hundredth.
func FromFloat(f float64) KWh { return KWh(math.Round(f * 100)) }

// Float returns the value in kWh.
func (e KWh) Float() float64 { return float64(e) / 100 }

// String formats the value like 1,204.50.
func (e KWh) String() string {
	sign := ""
	v := int64(e)
	if v < 0 {
		sign = "-"
		v = -v
	}
	whole := strconv.FormatInt(v/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s.%02d", sign, b.String(), v%100)
}

// ErrInvalid is returned for input that is not a kWh value.
var ErrInvalid = errors.New("enter the meter reading in kWh, for example 15230.4")

// Parse reads a kWh value typed by a person: "15230.4", "15,230.40", "15230 kWh".
func Parse(s string) (KWh, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "kWh"))
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, ErrInvalid
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return 0, ErrInvalid
	}
	return FromFloat(f), nil
}
