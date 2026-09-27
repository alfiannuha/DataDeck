package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestQueryExecutionSQLite(t *testing.T) {
	h, db := newQueryHandler(t)
	path := filepath.Join(t.TempDir(), "user.db")
	seedProfile(t, db, &model.ConnectionProfile{
		ID:           "sq",
		Name:         "SQLite",
		Driver:       model.DriverSQLite,
		DatabaseName: path,
	})

	execute := func(sql string) int {
		encoded, err := json.Marshal(sql)
		if err != nil {
			t.Fatalf("marshal sql: %v", err)
		}
		body := `{"connection_id":"sq","sql":` + string(encoded) + `}`
		rec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute", body)
		return rec.Code
	}

	for _, sql := range []string{
		`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO t VALUES (1, 'a'), (2, 'b')`,
	} {
		if code := execute(sql); code != http.StatusOK {
			t.Fatalf("execute %q status = %d, want 200", sql, code)
		}
	}

	selectRec := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"sq","sql":"SELECT id, name FROM t ORDER BY id"}`)
	if selectRec.Code != http.StatusOK || !decodeEnvelope(t, selectRec).Success {
		t.Fatalf("select status = %d (body=%s)", selectRec.Code, selectRec.Body.String())
	}

	failure := doRequest(h.Execute, http.MethodPost, "/api/v1/query/execute",
		`{"connection_id":"sq","sql":"SELCT 1"}`)
	if failure.Code != http.StatusBadRequest {
		t.Fatalf("syntax error status = %d, want 400", failure.Code)
	}
	if env := decodeEnvelope(t, failure); env.Error == nil || env.Error.Code != "SQL_SYNTAX_ERROR" {
		t.Errorf("syntax error = %+v, want SQL_SYNTAX_ERROR", env.Error)
	}

	historyRec := doRequest(h.History, http.MethodGet, "/api/v1/query/history?connection_id=sq", "")
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history status = %d", historyRec.Code)
	}
	var records []HistoryRecord
	if err := json.Unmarshal(decodeEnvelope(t, historyRec).Data, &records); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(records) < 4 {
		t.Fatalf("history len = %d, want at least 4", len(records))
	}
	if records[0].Status != string(model.QueryStatusError) {
		t.Errorf("newest history status = %q, want ERROR", records[0].Status)
	}
}
