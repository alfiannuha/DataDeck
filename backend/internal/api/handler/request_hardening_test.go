package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	h, _, _ := newTestHandler(t)
	body := `{"name":"` + strings.Repeat("a", maxBodyBytes+64) + `"}`

	rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "exceeds") {
		t.Errorf("body = %s, want an explicit size-limit message", rec.Body.String())
	}
}

func TestDecodeJSONRejectsTrailingData(t *testing.T) {
	h, _, _ := newTestHandler(t)
	for name, body := range map[string]string{
		"second object": validBody + ` {}`,
		"garbage":       validBody + ` trailing`,
		"array":         `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDecodeJSONRequiresJSONContentType(t *testing.T) {
	h, _, _ := newTestHandler(t)
	for name, contentType := range map[string]string{
		"missing":     "",
		"text/plain":  "text/plain",
		"form":        "application/x-www-form-urlencoded",
		"json suffix": "application/json-patch+json",
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/connections", strings.NewReader(validBody))
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}
			h.Create(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
		})
	}

	// Parameters are allowed (e.g. charset).
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections", strings.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	h.Create(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("charset variant status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestDecodeJSONUnknownFieldsIgnoredWrongTypesRejected(t *testing.T) {
	h, _, _ := newTestHandler(t)

	withExtra := `{"name":"PG","driver":"postgres","host":"127.0.0.1","port":5432,` +
		`"database_name":"app","username":"u","ssl_mode":"disable","future_field":true}`
	if rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", withExtra); rec.Code != http.StatusOK {
		t.Fatalf("unknown field status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	wrongType := `{"name":"PG","driver":"postgres","host":"127.0.0.1","port":"not-a-number",` +
		`"database_name":"app","username":"u","ssl_mode":"disable"}`
	if rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", wrongType); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong type status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}

	if rec := doRequest(h.Create, http.MethodPost, "/api/v1/connections", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestParsePageParamsBounds(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr bool
		page    int
		size    int
	}{
		{"defaults", "", false, 1, defaultPageSize},
		{"valid", "?page=3&page_size=10", false, 3, 10},
		{"clamps size", "?page_size=100000", false, 1, maxPageSize},
		{"zero page", "?page=0", true, 0, 0},
		{"negative page", "?page=-1", true, 0, 0},
		{"huge page", "?page=1000001", true, 0, 0},
		{"overflow page", "?page=99999999999999999999", true, 0, 0},
		{"zero size", "?page_size=0", true, 0, 0},
		{"non numeric", "?page=abc&page_size=xyz", true, 0, 0},
		{"unknown params ignored", "?cursor=whatever", false, 1, defaultPageSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x"+tc.query, nil)
			params, err := parsePageParams(req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePageParams(%q) error = nil, want failure", tc.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePageParams(%q) error = %v", tc.query, err)
			}
			if params.Page != tc.page || params.PageSize != tc.size {
				t.Fatalf("params = %+v, want page=%d size=%d", params, tc.page, tc.size)
			}
			if params.Offset < 0 {
				t.Fatalf("offset = %d, want non-negative", params.Offset)
			}
		})
	}
}

func TestHistoryRejectsAbusivePagination(t *testing.T) {
	h, _ := newQueryHandler(t)

	rec := doRequest(h.History, http.MethodGet,
		"/api/v1/query/history?page=1000000000&page_size=999999", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("error = %+v, want VALIDATION_ERROR", env.Error)
	}
}
