package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExportFilename(t *testing.T) {
	date := time.Now().Format("20060102")
	cases := []struct {
		database, schema, table, format string
		want                            string
	}{
		{"app_dev", "public", "customers", "csv", "app_dev_public_customers_" + date + ".csv"},
		{"my db!", "finance", "orders", "xlsx", "my_db_finance_orders_" + date + ".xlsx"},
		{"/var/lib/data.db", "", "metrics", "sql", "var_lib_data.db_metrics_" + date + ".sql"},
		{"..", "", ";DROP", "csv", "export_DROP_" + date + ".csv"},
		{"", "", "orders", "csv", "export_orders_" + date + ".csv"},
	}
	for _, tc := range cases {
		if got := exportFilename(tc.database, tc.schema, tc.table, tc.format); got != tc.want {
			t.Errorf("exportFilename(%q,%q,%q,%q) = %q, want %q", tc.database, tc.schema, tc.table, tc.format, got, tc.want)
		}
	}
}

func TestSanitizeFilenamePart(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"customers", "customers"},
		{"my db!", "my_db"},
		{"a/b\\c:d", "a_b_c_d"},
		{"..", "export"},
		{"", "export"},
		{"a....b", "a....b"}, // dots are kept; only separators collapse
		{"язык", "export"},   // non-ASCII collapses entirely
	}
	for _, tc := range cases {
		if got := sanitizeFilenamePart(tc.in); got != tc.want {
			t.Errorf("sanitizeFilenamePart(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	long := strings.Repeat("a", 200)
	if got := sanitizeFilenamePart(long); len(got) != 64 {
		t.Errorf("sanitizeFilenamePart(long) len = %d, want 64", len(got))
	}
}

func TestLazyHeaderWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	committed := false
	lazy := &lazyHeaderWriter{w: rec, contentType: "text/csv", filename: "x.csv", committed: &committed}
	if _, err := lazy.Write([]byte("a,b\r\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !committed {
		t.Error("Write must commit the headers")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="x.csv"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "a,b\r\n" {
		t.Errorf("body = %q", rec.Body.String())
	}
}
