package queries_test

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

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/queries"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/testsupport"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	db, cleanup, err := testsupport.Postgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres container unavailable: %v\n", err)
		os.Exit(0)
	}
	if _, err := testsupport.SeedFullSchema(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "schema setup failed: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	testDB = db
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// resetTables wipes per-test state while leaving the seed admin in place.
func resetTables(t *testing.T) {
	t.Helper()
	// CASCADE handles query_matches automatically.
	if _, err := testDB.Exec("DELETE FROM queries WHERE user_id != (SELECT id FROM users WHERE username = 'admin')"); err != nil {
		t.Fatalf("cleanup queries: %v", err)
	}
	if _, err := testDB.Exec("DELETE FROM sessions"); err != nil {
		t.Fatalf("cleanup sessions: %v", err)
	}
	if _, err := testDB.Exec("DELETE FROM users WHERE username != 'admin'"); err != nil {
		t.Fatalf("cleanup users: %v", err)
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	queries.RegisterRoutes(mux, testDB, auth.Required(testDB))
	return mux
}

func seedAliceBob(t *testing.T) (aliceCookie, bobCookie *http.Cookie, aliceID, bobID int64) {
	t.Helper()
	ctx := context.Background()
	alice, err := testsupport.SeedUser(ctx, testDB, "alice", "alice@test", "alicepassword", false)
	if err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	bob, err := testsupport.SeedUser(ctx, testDB, "bob", "bob@test", "bobpassword12", false)
	if err != nil {
		t.Fatalf("seed bob: %v", err)
	}
	aliceCookie, err = testsupport.SeedSession(ctx, testDB, alice.ID)
	if err != nil {
		t.Fatalf("seed alice session: %v", err)
	}
	bobCookie, err = testsupport.SeedSession(ctx, testDB, bob.ID)
	if err != nil {
		t.Fatalf("seed bob session: %v", err)
	}
	return aliceCookie, bobCookie, alice.ID, bob.ID
}

func jsonReq(t *testing.T, method, path string, body any, cookie *http.Cookie) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func do(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// --- happy paths ---

func TestCreateAndList(t *testing.T) {
	resetTables(t)
	alice, _, _, _ := seedAliceBob(t)
	h := newHandler()

	// Alice creates a query.
	req := jsonReq(t, "POST", "/api/queries", map[string]any{
		"name":         "sepsis",
		"query_string": "sepsis[tiab] AND humans[mh]",
	}, alice)
	w := do(t, h, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	// Alice lists — sees her one query.
	req = jsonReq(t, "GET", "/api/queries", nil, alice)
	w = do(t, h, req)
	if w.Code != 200 {
		t.Fatalf("list status=%d", w.Code)
	}
	var out struct {
		Queries []queries.Query `json:"queries"`
	}
	_ = json.NewDecoder(w.Body).Decode(&out)
	if len(out.Queries) != 1 || out.Queries[0].Name != "sepsis" {
		t.Fatalf("unexpected list: %+v", out.Queries)
	}
}

func TestCreateValidation(t *testing.T) {
	resetTables(t)
	alice, _, _, _ := seedAliceBob(t)
	h := newHandler()

	tests := []struct {
		name     string
		body     map[string]any
		wantCode int
	}{
		{"missing name", map[string]any{"query_string": "x"}, 400},
		{"missing query_string", map[string]any{"name": "x"}, 400},
		{"poll interval too low", map[string]any{"name": "x", "query_string": "x", "poll_interval_seconds": 300}, 400},
		{"negative min_abstract_length", map[string]any{"name": "x", "query_string": "x", "min_abstract_length": -1}, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := jsonReq(t, "POST", "/api/queries", tc.body, alice)
			w := do(t, h, req)
			if w.Code != tc.wantCode {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateDuplicate(t *testing.T) {
	resetTables(t)
	alice, _, _, _ := seedAliceBob(t)
	h := newHandler()

	body := map[string]any{"name": "dup", "query_string": "x"}

	req := jsonReq(t, "POST", "/api/queries", body, alice)
	if w := do(t, h, req); w.Code != 201 {
		t.Fatalf("first create status=%d", w.Code)
	}
	req = jsonReq(t, "POST", "/api/queries", body, alice)
	if w := do(t, h, req); w.Code != 409 {
		t.Fatalf("duplicate create should 409, got %d", w.Code)
	}
}

func TestPatchAndRepoll(t *testing.T) {
	resetTables(t)
	alice, _, _, _ := seedAliceBob(t)
	h := newHandler()

	// Create
	req := jsonReq(t, "POST", "/api/queries", map[string]any{"name": "n", "query_string": "x"}, alice)
	w := do(t, h, req)
	var q queries.Query
	_ = json.NewDecoder(w.Body).Decode(&q)

	// Patch
	newName := "renamed"
	req = jsonReq(t, "PATCH", "/api/queries/"+strconv.FormatInt(q.ID, 10),
		map[string]any{"name": newName, "is_active": false}, alice)
	w = do(t, h, req)
	if w.Code != 200 {
		t.Fatalf("patch status=%d body=%s", w.Code, w.Body.String())
	}
	var q2 queries.Query
	_ = json.NewDecoder(w.Body).Decode(&q2)
	if q2.Name != newName || q2.IsActive {
		t.Fatalf("patch didn't stick: %+v", q2)
	}

	// Simulate a prior poll so the repoll endpoint has something to clear.
	_, err := testDB.Exec("UPDATE queries SET last_polled_at = now() WHERE id = $1", q.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Repoll clears last_polled_at.
	req = jsonReq(t, "POST", "/api/queries/"+strconv.FormatInt(q.ID, 10)+"/repoll", nil, alice)
	w = do(t, h, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("repoll status=%d", w.Code)
	}
	var lastPolled sql.NullTime
	_ = testDB.QueryRow("SELECT last_polled_at FROM queries WHERE id = $1", q.ID).Scan(&lastPolled)
	if lastPolled.Valid {
		t.Fatal("repoll did not clear last_polled_at")
	}
}

// --- ownership isolation (the mandatory tests) ---

func TestIsolation_AliceCannotSeeBobsQueries(t *testing.T) {
	resetTables(t)
	alice, bob, _, _ := seedAliceBob(t)
	h := newHandler()

	// Bob creates a query.
	req := jsonReq(t, "POST", "/api/queries", map[string]any{"name": "bobsecret", "query_string": "x"}, bob)
	w := do(t, h, req)
	if w.Code != 201 {
		t.Fatalf("bob create status=%d", w.Code)
	}
	var bobQ queries.Query
	_ = json.NewDecoder(w.Body).Decode(&bobQ)

	// Alice lists — should see zero.
	req = jsonReq(t, "GET", "/api/queries", nil, alice)
	w = do(t, h, req)
	var out struct {
		Queries []queries.Query `json:"queries"`
	}
	_ = json.NewDecoder(w.Body).Decode(&out)
	if len(out.Queries) != 0 {
		t.Fatalf("alice sees bob's queries: %+v", out.Queries)
	}

	// GET bob's id as alice → 404 (not 403, not 200).
	req = jsonReq(t, "GET", "/api/queries/"+strconv.FormatInt(bobQ.ID, 10), nil, alice)
	if w := do(t, h, req); w.Code != 404 {
		t.Fatalf("alice GET bob's query: got %d, want 404", w.Code)
	}

	// PATCH bob's id as alice → 404.
	req = jsonReq(t, "PATCH", "/api/queries/"+strconv.FormatInt(bobQ.ID, 10),
		map[string]any{"name": "hijacked"}, alice)
	if w := do(t, h, req); w.Code != 404 {
		t.Fatalf("alice PATCH bob's query: got %d, want 404", w.Code)
	}

	// DELETE bob's id as alice → 404.
	req = jsonReq(t, "DELETE", "/api/queries/"+strconv.FormatInt(bobQ.ID, 10), nil, alice)
	if w := do(t, h, req); w.Code != 404 {
		t.Fatalf("alice DELETE bob's query: got %d, want 404", w.Code)
	}

	// Repoll → 404.
	req = jsonReq(t, "POST", "/api/queries/"+strconv.FormatInt(bobQ.ID, 10)+"/repoll", nil, alice)
	if w := do(t, h, req); w.Code != 404 {
		t.Fatalf("alice repoll bob's query: got %d, want 404", w.Code)
	}

	// Confirm bob's query is untouched.
	var stillName string
	_ = testDB.QueryRow("SELECT name FROM queries WHERE id = $1", bobQ.ID).Scan(&stillName)
	if stillName != "bobsecret" {
		t.Fatalf("bob's query was mutated through alice's requests: name=%s", stillName)
	}
}

func TestNoAuth_Returns401(t *testing.T) {
	resetTables(t)
	h := newHandler()
	for _, path := range []string{
		"GET /api/queries",
		"POST /api/queries",
		"GET /api/queries/1",
	} {
		parts := bytes.SplitN([]byte(path), []byte(" "), 2)
		req := httptest.NewRequest(string(parts[0]), string(parts[1]), bytes.NewReader(nil))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("%s without auth: got %d, want 401", path, w.Code)
		}
	}
}
