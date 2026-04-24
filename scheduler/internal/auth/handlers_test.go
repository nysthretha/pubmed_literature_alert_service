package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	cfg := auth.Config{
		DB:           testDB,
		RateLimiter:  auth.NewLoginRateLimiter(),
		CookieSecure: false,
	}
	auth.RegisterRoutes(mux, cfg)
	return mux
}

func seedAlice(t *testing.T, password string) int64 {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := auth.CreateUser(context.Background(), testDB, "alice", "alice@example.com", hash, false)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return u.ID
}

func postLogin(t *testing.T, handler http.Handler, username, password string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Result()
}

func TestLogin_Success(t *testing.T) {
	resetDB(t)
	seedAlice(t, "s3cret")
	handler := newRouter(t)

	resp := postLogin(t, handler, "alice", "s3cret")
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	var body struct {
		User *auth.User `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.User == nil || body.User.Username != "alice" {
		t.Fatalf("unexpected response body: %+v", body)
	}

	// Session cookie set
	var got bool
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			got = true
			if !c.HttpOnly {
				t.Error("cookie must be HttpOnly")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Error("cookie must be SameSite=Strict")
			}
		}
	}
	if !got {
		t.Fatal("no session cookie on login success")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	resetDB(t)
	seedAlice(t, "s3cret")
	handler := newRouter(t)

	resp := postLogin(t, handler, "alice", "WRONG")
	if resp.StatusCode != 401 {
		t.Fatalf("status=%d, want 401", resp.StatusCode)
	}
}

func TestLogin_NoSuchUser(t *testing.T) {
	resetDB(t)
	handler := newRouter(t)

	resp := postLogin(t, handler, "ghost", "whatever")
	if resp.StatusCode != 401 {
		t.Fatalf("status=%d, want 401 (must not leak user existence)", resp.StatusCode)
	}
}

func TestLogin_RateLimitsAfterN(t *testing.T) {
	resetDB(t)
	handler := newRouter(t)

	// 5 attempts against nonexistent user from same RemoteAddr should be allowed;
	// 6th must be 429.
	for i := 0; i < 5; i++ {
		resp := postLogin(t, handler, "ghost", "x")
		if resp.StatusCode != 401 {
			t.Fatalf("attempt %d: got %d, want 401", i+1, resp.StatusCode)
		}
	}
	resp := postLogin(t, handler, "ghost", "x")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rate limit did not trigger: got %d, want 429", resp.StatusCode)
	}
}

func TestMe_WithValidSession(t *testing.T) {
	resetDB(t)
	seedAlice(t, "s3cret")
	handler := newRouter(t)

	loginResp := postLogin(t, handler, "alice", "s3cret")
	var cookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	var body struct {
		User *auth.User `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.User == nil || body.User.Username != "alice" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestMe_WithoutSession(t *testing.T) {
	resetDB(t)
	handler := newRouter(t)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != 401 {
		t.Fatalf("status=%d, want 401", w.Result().StatusCode)
	}
}

func TestLogout_InvalidatesSession(t *testing.T) {
	resetDB(t)
	seedAlice(t, "s3cret")
	handler := newRouter(t)

	loginResp := postLogin(t, handler, "alice", "s3cret")
	var cookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c
		}
	}

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != 200 {
		t.Fatalf("logout status=%d, want 200", w.Result().StatusCode)
	}

	// Now /me with the old cookie should 401.
	req2 := httptest.NewRequest("GET", "/api/auth/me", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Result().StatusCode != 401 {
		t.Fatalf("post-logout /me status=%d, want 401", w2.Result().StatusCode)
	}
}

func TestExpiredSession_Rejected(t *testing.T) {
	resetDB(t)
	uid := seedAlice(t, "s3cret")
	handler := newRouter(t)

	// Insert a session that's already past its idle timeout.
	expiredID := "expired-session-id"
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO sessions (id, user_id, created_at, expires_at, last_seen_at) VALUES ($1,$2,$3,$4,$5)",
		expiredID, uid,
		time.Now().Add(-14*24*time.Hour),
		time.Now().Add(16*24*time.Hour), // absolute TTL not yet hit
		time.Now().Add(-14*24*time.Hour), // but idle 14 days > 7-day timeout
	)
	if err != nil {
		t.Fatalf("seed expired: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: expiredID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != 401 {
		t.Fatalf("status=%d, want 401", w.Result().StatusCode)
	}
}
