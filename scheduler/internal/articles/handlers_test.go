package articles_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/articles"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/testsupport"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	db, cleanup, err := testsupport.Postgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres unavailable: %v\n", err)
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

func resetTables(t *testing.T) {
	t.Helper()
	// query_matches CASCADEs from queries and articles.
	for _, stmt := range []string{
		"DELETE FROM articles",
		"DELETE FROM queries WHERE user_id != (SELECT id FROM users WHERE username = 'admin')",
		"DELETE FROM sessions",
		"DELETE FROM users WHERE username != 'admin'",
	} {
		if _, err := testDB.Exec(stmt); err != nil {
			t.Fatalf("cleanup: %s: %v", stmt, err)
		}
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	articles.RegisterRoutes(mux, testDB, auth.Required(testDB))
	return mux
}

// seedMatchedArticle inserts an article and wires it to a query owned by
// userID via an inserted query_match. Returns (pmid, queryID).
func seedMatchedArticle(t *testing.T, userID int64, queryName, pmid, title string) (string, int64) {
	t.Helper()
	ctx := context.Background()
	var qid int64
	if err := testDB.QueryRowContext(ctx, `
		INSERT INTO queries (user_id, name, query_string)
		VALUES ($1, $2, 'x')
		RETURNING id
	`, userID, queryName).Scan(&qid); err != nil {
		t.Fatalf("insert query: %v", err)
	}
	if _, err := testDB.ExecContext(ctx, `
		INSERT INTO articles (pmid, title) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, pmid, title); err != nil {
		t.Fatalf("insert article: %v", err)
	}
	if _, err := testDB.ExecContext(ctx, `
		INSERT INTO query_matches (query_id, pmid) VALUES ($1, $2)
	`, qid, pmid); err != nil {
		t.Fatalf("insert match: %v", err)
	}
	return pmid, qid
}

func TestIsolation_ArticlesScopedToUser(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	alice, err := testsupport.SeedUser(ctx, testDB, "alice", "alice@test", "alicepassword", false)
	if err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	bob, err := testsupport.SeedUser(ctx, testDB, "bob", "bob@test", "bobpassword12", false)
	if err != nil {
		t.Fatalf("seed bob: %v", err)
	}
	aliceC, _ := testsupport.SeedSession(ctx, testDB, alice.ID)
	bobC, _ := testsupport.SeedSession(ctx, testDB, bob.ID)

	// Alice's article
	alicePMID, _ := seedMatchedArticle(t, alice.ID, "alice-q", "1111", "Alice's article")
	// Bob's article
	bobPMID, _ := seedMatchedArticle(t, bob.ID, "bob-q", "2222", "Bob's article")

	h := newHandler()

	// Alice's list includes only hers.
	req := httptest.NewRequest("GET", "/api/articles", nil)
	req.AddCookie(aliceC)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("alice list status=%d", w.Code)
	}
	var out struct {
		Articles []articles.Article `json:"articles"`
		Total    int                `json:"total"`
		HasMore  bool               `json:"has_more"`
	}
	_ = json.NewDecoder(w.Body).Decode(&out)
	if out.Total != 1 || len(out.Articles) != 1 || out.Articles[0].PMID != alicePMID {
		t.Fatalf("alice got unexpected list: %+v", out)
	}

	// Alice GET bob's PMID → 404.
	req = httptest.NewRequest("GET", "/api/articles/"+bobPMID, nil)
	req.AddCookie(aliceC)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("alice GET bob's article: got %d, want 404", w.Code)
	}

	// Bob's list includes only his.
	req = httptest.NewRequest("GET", "/api/articles", nil)
	req.AddCookie(bobC)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("bob list status=%d", w.Code)
	}
	var bobOut struct {
		Articles []articles.Article `json:"articles"`
		Total    int                `json:"total"`
	}
	_ = json.NewDecoder(w.Body).Decode(&bobOut)
	if bobOut.Total != 1 || bobOut.Articles[0].PMID != bobPMID {
		t.Fatalf("bob got unexpected list: %+v", bobOut)
	}
}
