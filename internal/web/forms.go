package web

import (
	"net/mail"
	"regexp"
	"strings"
)

// residentForm is the household details form on the flat page.
type residentForm struct {
	Name   string
	Phone  string
	Email  string
	Errors map[string]string
}

// Nigerian mobile numbers: 080, 081, 070, 090, 091 and so on, 11 digits.
var phonePattern = regexp.MustCompile(`^0[789][01]\d{8}$`)

func cleanPhone(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(s))
}

func (f *residentForm) validate() bool {
	f.Errors = map[string]string{}
	f.Name = strings.Join(strings.Fields(f.Name), " ")
	f.Email = strings.ToLower(strings.TrimSpace(f.Email))
	f.Phone = cleanPhone(f.Phone)

	if len(f.Name) < 3 {
		f.Errors["name"] = "Enter the resident's full name."
	}
	if !phonePattern.MatchString(f.Phone) {
		f.Errors["phone"] = "Enter a Nigerian mobile number, for example 0803 123 4567."
	}
	if f.Email != "" {
		if _, err := mail.ParseAddress(f.Email); err != nil || !strings.Contains(f.Email, ".") {
			f.Errors["email"] = "Enter a valid email address or leave it blank."
		}
	}
	return len(f.Errors) == 0
}
