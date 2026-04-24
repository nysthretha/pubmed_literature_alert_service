package auth_test

import (
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

func TestLoginRateLimiter_IPBucketExhausts(t *testing.T) {
	lim := auth.NewLoginRateLimiter()

	for i := 0; i < 5; i++ {
		if !lim.Allow("10.0.0.1", "alice") {
			t.Fatalf("attempt %d should have been allowed (burst=5)", i+1)
		}
	}
	if lim.Allow("10.0.0.1", "alice") {
		t.Fatal("6th attempt from same IP should be denied")
	}
}

func TestLoginRateLimiter_UsernameBucketExhausts(t *testing.T) {
	lim := auth.NewLoginRateLimiter()

	// 10 attempts for same username from rotating IPs — username bucket is 10/hr.
	for i := 0; i < 10; i++ {
		ip := fakeIP(i)
		if !lim.Allow(ip, "bob") {
			t.Fatalf("attempt %d should have been allowed (username burst=10)", i+1)
		}
	}
	if lim.Allow(fakeIP(11), "bob") {
		t.Fatal("11th attempt against same username should be denied")
	}
}

func TestLoginRateLimiter_DifferentUsersIndependent(t *testing.T) {
	lim := auth.NewLoginRateLimiter()

	// Exhaust alice
	for i := 0; i < 5; i++ {
		lim.Allow("10.0.0.1", "alice")
	}
	// bob from a different IP should still be allowed
	if !lim.Allow("10.0.0.2", "bob") {
		t.Fatal("unrelated user from different IP should not be blocked")
	}
}

func fakeIP(i int) string {
	return "10.0.0." + itoa3(i)
}

func itoa3(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
