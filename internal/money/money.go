// Package money handles naira amounts. Amounts are stored in kobo to avoid
// floating point surprises.
package money

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Kobo is an amount of money in kobo (1/100 naira).
type Kobo int64

// Naira builds an amount from whole naira.
func Naira(n int64) Kobo { return Kobo(n * 100) }

// String formats the amount like ₦12,500.00.
func (k Kobo) String() string {
	sign := ""
	v := int64(k)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s₦%s.%02d", sign, groupThousands(v/100), v%100)
}

// Plain formats the amount without the currency sign, e.g. 12,500.00.
func (k Kobo) Plain() string { return strings.TrimPrefix(k.String(), "₦") }

// Float returns the amount in naira, for display maths only.
func (k Kobo) Float() float64 { return float64(k) / 100 }

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// ErrInvalid is returned for input that is not an amount.
var ErrInvalid = errors.New("enter an amount in naira, for example 5,000 or 5000.50")

// Parse reads an amount typed by a person: "5000", "5,000", "₦5,000.50", "N5000".
func Parse(s string) (Kobo, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "₦")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "NGN"), "N")
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, ErrInvalid
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrInvalid
	}
	return Kobo(math.Round(f * 100)), nil
}

// VATRate is the Nigerian VAT rate in basis points (7.5%).
const VATRate = 750

// SplitVAT splits a VAT-inclusive total into the net amount and the VAT in it.
func SplitVAT(total Kobo) (net, vat Kobo) {
	net = Kobo(math.Round(float64(total) * 10000 / float64(10000+VATRate)))
	return net, total - net
}
