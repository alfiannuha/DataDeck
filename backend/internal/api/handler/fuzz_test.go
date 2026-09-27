package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// FuzzParsePageParams asserts the pagination parser never panics and always
// returns either an error or bounded, non-negative parameters. Only the seed
// corpus runs during `go test`; `-fuzz` explores further.
func FuzzParsePageParams(f *testing.F) {
	for _, seed := range []struct{ page, size string }{
		{"1", "50"}, {"", ""}, {"0", "0"}, {"-1", "-1"},
		{"1000000", "200"}, {"1000001", "201"},
		{"99999999999999999999", "99999999999999999999"}, {"abc", "xyz"},
	} {
		f.Add(seed.page, seed.size)
	}

	f.Fuzz(func(t *testing.T, page, size string) {
		target := "/x?page=" + url.QueryEscape(page) + "&page_size=" + url.QueryEscape(size)
		req := httptest.NewRequest(http.MethodGet, target, nil)

		params, err := parsePageParams(req)
		if err != nil {
			return
		}
		if params.Page < 1 || params.Page > maxPage {
			t.Fatalf("page out of bounds: %d", params.Page)
		}
		if params.PageSize < 1 || params.PageSize > maxPageSize {
			t.Fatalf("page_size out of bounds: %d", params.PageSize)
		}
		if params.Offset < 0 {
			t.Fatalf("offset is negative: %d", params.Offset)
		}
	})
}

// FuzzDecodeJSON asserts the decoder never panics and never returns success for
// malformed input. The body size is bounded so `-fuzz` cannot exhaust memory.
func FuzzDecodeJSON(f *testing.F) {
	for _, seed := range []string{
		`{}`,
		`{"name":"x"}`,
		`{"name":"x"} {}`,
		`not json`,
		``,
		`{"name":123}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 2*maxBodyBytes {
			t.Skip()
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		var dst struct {
			Name string `json:"name"`
		}
		if decodeJSON(rec, req, &dst) {
			if rec.Code != http.StatusOK {
				t.Fatalf("decodeJSON reported success with status %d", rec.Code)
			}
			return
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("decodeJSON failure status = %d, want 400", rec.Code)
		}
	})
}
