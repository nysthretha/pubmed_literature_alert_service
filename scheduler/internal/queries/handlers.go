package queries

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/apiutil"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/validation"
)

// Defaults applied on create when a field is omitted from the request.
const (
	defaultPollIntervalSeconds = 21_600 // 6h
	defaultMinAbstractLength   = 0
	minPollIntervalSeconds     = 3_600 // 1h — be nice to NCBI
)

var defaultBlocklist = []string{"Comment", "Retraction of Publication", "Published Erratum"}

// --- POST /api/queries body ---

type createReq struct {
	Name                     string    `json:"name"`
	QueryString              string    `json:"query_string"`
	PollIntervalSeconds      *int      `json:"poll_interval_seconds,omitempty"`
	IsActive                 *bool     `json:"is_active,omitempty"`
	MinAbstractLength        *int      `json:"min_abstract_length,omitempty"`
	PublicationTypeAllowlist *[]string `json:"publication_type_allowlist,omitempty"`
	PublicationTypeBlocklist *[]string `json:"publication_type_blocklist,omitempty"`
	Notes                    *string   `json:"notes,omitempty"`
}

func (r *createReq) Validate() *validation.ValidationErrors {
	v := validation.New()
	v.Required("name", r.Name)
	v.MaxLen("name", r.Name, 100)
	v.Required("query_string", r.QueryString)
	v.MaxLen("query_string", r.QueryString, 500)
	if r.PollIntervalSeconds != nil {
		v.MinInt("poll_interval_seconds", *r.PollIntervalSeconds, minPollIntervalSeconds)
	}
	if r.MinAbstractLength != nil {
		v.MinInt("min_abstract_length", *r.MinAbstractLength, 0)
	}
	return v.Err()
}

func (r *createReq) toInput() CreateInput {
	in := CreateInput{
		Name:                r.Name,
		QueryString:         r.QueryString,
		PollIntervalSeconds: defaultPollIntervalSeconds,
		IsActive:            true,
		MinAbstractLength:   defaultMinAbstractLength,
		PublicationTypeBlocklist: defaultBlocklist,
		Notes:               r.Notes,
	}
	if r.PollIntervalSeconds != nil {
		in.PollIntervalSeconds = *r.PollIntervalSeconds
	}
	if r.IsActive != nil {
		in.IsActive = *r.IsActive
	}
	if r.MinAbstractLength != nil {
		in.MinAbstractLength = *r.MinAbstractLength
	}
	if r.PublicationTypeAllowlist != nil {
		in.PublicationTypeAllowlist = *r.PublicationTypeAllowlist
	}
	if r.PublicationTypeBlocklist != nil {
		in.PublicationTypeBlocklist = *r.PublicationTypeBlocklist
	}
	return in
}

// --- PATCH /api/queries/:id body ---

type patchReq struct {
	Name                     *string   `json:"name,omitempty"`
	QueryString              *string   `json:"query_string,omitempty"`
	PollIntervalSeconds      *int      `json:"poll_interval_seconds,omitempty"`
	IsActive                 *bool     `json:"is_active,omitempty"`
	MinAbstractLength        *int      `json:"min_abstract_length,omitempty"`
	PublicationTypeAllowlist *[]string `json:"publication_type_allowlist,omitempty"`
	PublicationTypeBlocklist *[]string `json:"publication_type_blocklist,omitempty"`
	Notes                    *string   `json:"notes,omitempty"`
}

func (r *patchReq) Validate() *validation.ValidationErrors {
	v := validation.New()
	if r.Name != nil {
		v.Required("name", *r.Name)
		v.MaxLen("name", *r.Name, 100)
	}
	if r.QueryString != nil {
		v.Required("query_string", *r.QueryString)
		v.MaxLen("query_string", *r.QueryString, 500)
	}
	if r.PollIntervalSeconds != nil {
		v.MinInt("poll_interval_seconds", *r.PollIntervalSeconds, minPollIntervalSeconds)
	}
	if r.MinAbstractLength != nil {
		v.MinInt("min_abstract_length", *r.MinAbstractLength, 0)
	}
	return v.Err()
}

func (r *patchReq) toInput() UpdateInput {
	return UpdateInput{
		Name:                     r.Name,
		QueryString:              r.QueryString,
		PollIntervalSeconds:      r.PollIntervalSeconds,
		IsActive:                 r.IsActive,
		MinAbstractLength:        r.MinAbstractLength,
		PublicationTypeAllowlist: r.PublicationTypeAllowlist,
		PublicationTypeBlocklist: r.PublicationTypeBlocklist,
		Notes:                    r.Notes,
	}
}

// --- handlers ---

func listHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userOr401(w, r)
		if u == nil {
			return
		}
		list, err := List(r.Context(), db, u.ID)
		if err != nil {
			apiutil.WriteInternal(w, err, "queries.list")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"queries": list})
	}
}

func createHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userOr401(w, r)
		if u == nil {
			return
		}
		var req createReq
		if !apiutil.DecodeJSON(w, r, &req) {
			return
		}
		if errs := req.Validate(); errs != nil {
			apiutil.WriteValidation(w, errs.Fields)
			return
		}
		q, err := Create(r.Context(), db, u.ID, req.toInput())
		if err != nil {
			if errors.Is(err, ErrNameExists) {
				apiutil.WriteConflict(w, "a query with that name already exists")
				return
			}
			apiutil.WriteInternal(w, err, "queries.create")
			return
		}
		slog.Info("queries.create", "user_id", u.ID, "query_id", q.ID, "name", q.Name)
		apiutil.WriteJSON(w, http.StatusCreated, q)
	}
}

func getHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userOr401(w, r)
		if u == nil {
			return
		}
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}
		q, err := GetByID(r.Context(), db, u.ID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "query")
				return
			}
			apiutil.WriteInternal(w, err, "queries.get")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, q)
	}
}

func patchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userOr401(w, r)
		if u == nil {
			return
		}
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}
		var req patchReq
		if !apiutil.DecodeJSON(w, r, &req) {
			return
		}
		if errs := req.Validate(); errs != nil {
			apiutil.WriteValidation(w, errs.Fields)
			return
		}
		q, err := Update(r.Context(), db, u.ID, id, req.toInput())
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "query")
				return
			}
			if errors.Is(err, ErrNameExists) {
				apiutil.WriteConflict(w, "a query with that name already exists")
				return
			}
			apiutil.WriteInternal(w, err, "queries.update")
			return
		}
		slog.Info("queries.update", "user_id", u.ID, "query_id", q.ID)
		apiutil.WriteJSON(w, http.StatusOK, q)
	}
}

func deleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userOr401(w, r)
		if u == nil {
			return
		}
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}

		// Fetch existence + metadata for audit logging BEFORE the DELETE.
		q, err := GetByID(r.Context(), db, u.ID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "query")
				return
			}
			apiutil.WriteInternal(w, err, "queries.delete.lookup")
			return
		}
		cascadeCount, _ := CountMatches(r.Context(), db, u.ID, id)

		slog.Info("queries.delete.intent",
			"user_id", u.ID, "query_id", q.ID,
			"name", q.Name, "query_string", q.QueryString,
			"cascade_query_matches", cascadeCount,
		)

		if err := Delete(r.Context(), db, u.ID, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "query")
				return
			}
			apiutil.WriteInternal(w, err, "queries.delete")
			return
		}
		slog.Info("queries.delete.done", "user_id", u.ID, "query_id", id, "cascaded", cascadeCount)
		w.WriteHeader(http.StatusNoContent)
	}
}

func repollHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userOr401(w, r)
		if u == nil {
			return
		}
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}
		if err := ClearLastPolled(r.Context(), db, u.ID, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "query")
				return
			}
			apiutil.WriteInternal(w, err, "queries.repoll")
			return
		}
		slog.Info("queries.repoll", "user_id", u.ID, "query_id", id)
		apiutil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "repoll_queued"})
	}
}

// --- helpers ---

func userOr401(w http.ResponseWriter, r *http.Request) *auth.User {
	u, ok := auth.UserFromContext(r)
	if !ok {
		apiutil.WriteUnauthorized(w)
		return nil
	}
	return u
}

func parseIDPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		apiutil.WriteBadRequest(w, "invalid id in path")
		return 0, false
	}
	return id, true
}
