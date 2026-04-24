package validation_test

import (
	"regexp"
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/validation"
)

func TestRequired(t *testing.T) {
	v := validation.New()
	v.Required("name", "")
	v.Required("other", "x")
	errs := v.Err()
	if errs == nil || len(errs.Fields) != 1 {
		t.Fatalf("expected 1 error, got %+v", errs)
	}
	if errs.Fields[0].Field != "name" {
		t.Fatalf("got %s, want name", errs.Fields[0].Field)
	}
}

func TestMinLenMaxLen(t *testing.T) {
	v := validation.New()
	v.MinLen("name", "", 1)
	v.MaxLen("name", "abcdefghij", 5)
	errs := v.Err()
	if errs == nil || len(errs.Fields) != 2 {
		t.Fatalf("expected 2 errors, got %+v", errs)
	}
}

func TestMinIntMaxInt(t *testing.T) {
	v := validation.New()
	v.MinInt("poll_interval_seconds", 1800, 3600)
	v.MaxInt("limit", 500, 200)
	errs := v.Err()
	if errs == nil || len(errs.Fields) != 2 {
		t.Fatalf("expected 2 errors, got %+v", errs)
	}
}

func TestMatches(t *testing.T) {
	v := validation.New()
	emailRE := regexp.MustCompile(`^[^@]+@[^@]+$`)
	v.Matches("email", "no-at-sign", emailRE, "a valid email")
	errs := v.Err()
	if errs == nil || len(errs.Fields) != 1 {
		t.Fatal("expected match failure")
	}
}

func TestNoErrors(t *testing.T) {
	v := validation.New()
	v.Required("name", "ahmet")
	v.MinLen("name", "ahmet", 1)
	v.MaxLen("name", "ahmet", 10)
	if v.Err() != nil {
		t.Fatal("expected no errors, got some")
	}
}

func TestCollectsAllErrors(t *testing.T) {
	v := validation.New()
	v.Required("a", "")
	v.Required("b", "")
	v.MinInt("c", 0, 10)
	errs := v.Err()
	if errs == nil || len(errs.Fields) != 3 {
		t.Fatalf("expected 3 errors, got %+v", errs)
	}
}
