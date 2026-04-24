// Package validation provides a hand-rolled Validator that collects all input
// errors in one pass and emits them in the API error shape.
//
// Usage pattern (in a request struct's Validate method):
//
//	func (r *createReq) Validate() *ValidationErrors {
//	    v := validation.New()
//	    v.Required("name", r.Name)
//	    v.MaxLen("name", r.Name, 100)
//	    if r.PollInterval != nil {
//	        v.MinInt("poll_interval_seconds", *r.PollInterval, 3600)
//	    }
//	    return v.Err()
//	}
//
// DB-dependent checks (unique name, existence of referenced ID) should happen
// in the handler AFTER Validate() returns nil — not in Validate itself.
package validation

import (
	"regexp"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/apiutil"
)

// ValidationErrors is a collected set of field-level failures.
// The zero value is valid and reports OK.
type ValidationErrors struct {
	Fields []apiutil.FieldError
}

func (v *ValidationErrors) Add(field, message string) {
	v.Fields = append(v.Fields, apiutil.FieldError{Field: field, Message: message})
}

// Validator accumulates errors. Call Err() to retrieve the collected result,
// or nil if no errors were recorded.
type Validator struct {
	errs ValidationErrors
}

func New() *Validator {
	return &Validator{}
}

// Err returns nil if no errors accumulated, otherwise a *ValidationErrors.
func (v *Validator) Err() *ValidationErrors {
	if len(v.errs.Fields) == 0 {
		return nil
	}
	out := v.errs
	return &out
}

// Required asserts the string is non-empty (after no trimming — leading/trailing
// spaces pass; trim at the caller if you want them to fail).
func (v *Validator) Required(field, value string) {
	if value == "" {
		v.errs.Add(field, "is required")
	}
}

// MinLen asserts len(value) >= min.
func (v *Validator) MinLen(field, value string, min int) {
	if len(value) < min {
		v.errs.Add(field, "must be at least "+itoa(min)+" characters")
	}
}

// MaxLen asserts len(value) <= max.
func (v *Validator) MaxLen(field, value string, max int) {
	if len(value) > max {
		v.errs.Add(field, "must be at most "+itoa(max)+" characters")
	}
}

// MinInt asserts value >= min.
func (v *Validator) MinInt(field string, value, min int) {
	if value < min {
		v.errs.Add(field, "must be >= "+itoa(min))
	}
}

// MaxInt asserts value <= max.
func (v *Validator) MaxInt(field string, value, max int) {
	if value > max {
		v.errs.Add(field, "must be <= "+itoa(max))
	}
}

// Matches asserts value matches the given regexp; pattern is provided as the
// user-facing description (e.g. "a valid email address").
func (v *Validator) Matches(field, value string, pattern *regexp.Regexp, desc string) {
	if !pattern.MatchString(value) {
		v.errs.Add(field, "must be "+desc)
	}
}

// HasField reports whether an error has been recorded for the given field.
// Useful for skipping subsequent checks that would be noise.
func (v *Validator) HasField(field string) bool {
	for _, f := range v.errs.Fields {
		if f.Field == field {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
