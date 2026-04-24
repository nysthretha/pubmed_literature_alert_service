package auth_test

import (
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if h == "" {
		t.Fatal("hash must not be empty")
	}

	ok, err := auth.VerifyPassword("correct horse battery staple", h)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("verify returned false for correct password")
	}

	ok, err = auth.VerifyPassword("wrong password", h)
	if err != nil {
		t.Fatalf("verify (wrong): %v", err)
	}
	if ok {
		t.Fatal("verify returned true for wrong password")
	}
}

func TestHashPasswordsAreSalted(t *testing.T) {
	h1, _ := auth.HashPassword("samesame")
	h2, _ := auth.HashPassword("samesame")
	if h1 == h2 {
		t.Fatal("hashes of the same password collided — salt missing?")
	}
}
