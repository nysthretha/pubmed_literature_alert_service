package admin_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/admin"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/testsupport"
)

var (
	testDB    *sql.DB
	seedAdmin *auth.User
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	db, cleanup, err := testsupport.Postgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres unavailable: %v\n", err)
		os.Exit(0)
	}
	adm, err := testsupport.SeedFullSchema(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema setup failed: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	testDB = db
	seedAdmin = adm
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func resetNonAdminTables(t *testing.T) {
	t.Helper()
	for _, stmt := range []string{
		"DELETE FROM sessions WHERE user_id != $1",
		"DELETE FROM queries WHERE user_id != $1",
		"DELETE FROM users WHERE id != $1",
	} {
		if _, err := testDB.Exec(stmt, seedAdmin.ID); err != nil {
			t.Fatalf("cleanup: %s: %v", stmt, err)
		}
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	authReq := auth.Required(testDB)
	adminReq := func(next http.Handler) http.Handler {
		return authReq(auth.AdminRequired(next))
	}
	admin.RegisterRoutes(mux, testDB, adminReq)
	return mux
}

func withCookie(method, path string, body any, c *http.Cookie) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c != nil {
		req.AddCookie(c)
	}
	return req
}

func TestNonAdmin_Returns403(t *testing.T) {
	resetNonAdminTables(t)
	ctx := context.Background()
	alice, _ := testsupport.SeedUser(ctx, testDB, "alice", "alice@test", "alicepassword", false)
	aliceC, _ := testsupport.SeedSession(ctx, testDB, alice.ID)
	h := newHandler()

	cases := []struct {
		method, path string
	}{
		{"GET", "/api/admin/users"},
		{"POST", "/api/admin/users"},
		{"PATCH", "/api/admin/users/1"},
		{"DELETE", "/api/admin/users/1"},
		{"POST", "/api/admin/users/1/reset-password"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			req := withCookie(c.method, c.path, map[string]any{}, aliceC)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("non-admin got %d, want 403, body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAdmin_SelfDelete_Returns400(t *testing.T) {
	resetNonAdminTables(t)
	ctx := context.Background()
	adminC, _ := testsupport.SeedSession(ctx, testDB, seedAdmin.ID)
	h := newHandler()

	req := withCookie("DELETE", "/api/admin/users/"+strconv.FormatInt(seedAdmin.ID, 10), nil, adminC)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("self-delete got %d, want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestAdmin_CreateListDeleteOther(t *testing.T) {
	resetNonAdminTables(t)
	ctx := context.Background()
	adminC, _ := testsupport.SeedSession(ctx, testDB, seedAdmin.ID)
	h := newHandler()

	// Create
	req := withCookie("POST", "/api/admin/users", map[string]any{
		"username": "charlie",
		"email":    "charlie@test.com",
		"password": "charliepassword12",
	}, adminC)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		User *auth.User `json:"user"`
	}
	_ = json.NewDecoder(w.Body).Decode(&out)
	if out.User == nil || out.User.Username != "charlie" {
		t.Fatalf("unexpected user in response: %+v", out.User)
	}

	// List — includes admin and charlie
	req = withCookie("GET", "/api/admin/users", nil, adminC)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list got %d", w.Code)
	}
	var list struct {
		Users []auth.AdminUserRow `json:"users"`
		Total int                 `json:"total"`
	}
	_ = json.NewDecoder(w.Body).Decode(&list)
	if list.Total < 2 {
		t.Fatalf("total should be >= 2, got %d", list.Total)
	}

	// Delete charlie
	req = withCookie("DELETE", "/api/admin/users/"+strconv.FormatInt(out.User.ID, 10), nil, adminC)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdmin_ValidationFailures(t *testing.T) {
	resetNonAdminTables(t)
	ctx := context.Background()
	adminC, _ := testsupport.SeedSession(ctx, testDB, seedAdmin.ID)
	h := newHandler()

	cases := []struct {
		name string
		body map[string]any
	}{
		{"short username", map[string]any{"username": "ab", "email": "a@b.com", "password": "charliepassword12"}},
		{"bad email", map[string]any{"username": "charlie", "email": "no-at-sign", "password": "charliepassword12"}},
		{"short password", map[string]any{"username": "charlie", "email": "c@d.com", "password": "short"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withCookie("POST", "/api/admin/users", tc.body, adminC)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != 400 {
				t.Fatalf("got %d, want 400, body=%s", w.Code, w.Body.String())
			}
		})
	}
}
