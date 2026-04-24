package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

func TestChangePassword_Success(t *testing.T) {
	resetDB(t)
	seedAlice(t, "current-password")
	router := newRouter(t)

	// Log in to get a session cookie.
	loginResp := postLogin(t, router, "alice", "current-password")
	var cookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie after login")
	}

	// Change password.
	body, _ := json.Marshal(map[string]string{
		"current_password": "current-password",
		"new_password":     "new-password-12",
	})
	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("change status=%d body=%s", w.Code, w.Body.String())
	}

	// Log out and confirm new password works.
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	router.ServeHTTP(httptest.NewRecorder(), logoutReq)

	// Old password should now fail.
	if r := postLogin(t, router, "alice", "current-password"); r.StatusCode != 401 {
		t.Fatalf("old password login got %d, want 401", r.StatusCode)
	}
	// New password should succeed.
	if r := postLogin(t, router, "alice", "new-password-12"); r.StatusCode != 200 {
		t.Fatalf("new password login got %d, want 200", r.StatusCode)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	resetDB(t)
	seedAlice(t, "current-password")
	router := newRouter(t)

	loginResp := postLogin(t, router, "alice", "current-password")
	var cookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c
		}
	}

	body, _ := json.Marshal(map[string]string{
		"current_password": "WRONG",
		"new_password":     "new-password-12",
	})
	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestChangePassword_TooShort(t *testing.T) {
	resetDB(t)
	seedAlice(t, "current-password")
	router := newRouter(t)

	loginResp := postLogin(t, router, "alice", "current-password")
	var cookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c
		}
	}

	body, _ := json.Marshal(map[string]string{
		"current_password": "current-password",
		"new_password":     "short",
	})
	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("got %d, want 400 body=%s", w.Code, w.Body.String())
	}
}

func TestChangePassword_NoSession_401(t *testing.T) {
	resetDB(t)
	router := newRouter(t)
	body, _ := json.Marshal(map[string]string{"current_password": "x", "new_password": "yyyyyyyy"})
	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("got %d, want 401", w.Code)
	}
}
