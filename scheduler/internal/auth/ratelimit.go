package auth

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// LoginRateLimiter combines per-IP and per-username token buckets.
// Thread-safe; in-memory only (fine for single-instance deployment).
//
// Per user spec:
//   - 5 attempts / IP / minute
//   - 10 attempts / username / hour
type LoginRateLimiter struct {
	mu        sync.Mutex
	perIP     map[string]*rate.Limiter
	perUser   map[string]*rate.Limiter
	ipRate    rate.Limit
	ipBurst   int
	userRate  rate.Limit
	userBurst int
}

// NewLoginRateLimiter builds a limiter with the spec's defaults.
func NewLoginRateLimiter() *LoginRateLimiter {
	return &LoginRateLimiter{
		perIP:     map[string]*rate.Limiter{},
		perUser:   map[string]*rate.Limiter{},
		ipRate:    rate.Every(60 * time.Second / 5),    // 5/min
		ipBurst:   5,
		userRate:  rate.Every(3600 * time.Second / 10), // 10/hour
		userBurst: 10,
	}
}

// Allow returns true if both the ip and username quotas permit another attempt.
func (l *LoginRateLimiter) Allow(ip, username string) bool {
	ip = strings.TrimSpace(ip)
	username = strings.ToLower(strings.TrimSpace(username))

	l.mu.Lock()
	defer l.mu.Unlock()

	ipLim, ok := l.perIP[ip]
	if !ok {
		ipLim = rate.NewLimiter(l.ipRate, l.ipBurst)
		l.perIP[ip] = ipLim
	}
	userLim, ok := l.perUser[username]
	if !ok {
		userLim = rate.NewLimiter(l.userRate, l.userBurst)
		l.perUser[username] = userLim
	}
	// Note: we must consume from both; calling Allow() reserves a token even if
	// the other denies. For the personal-scale volume here this is fine.
	return ipLim.Allow() && userLim.Allow()
}
