package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func TestGuardFormula(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"=1+1", "'=1+1"},
		{"+SUM(A1)", "'+SUM(A1)"},
		{"@cmd", "'@cmd"},
		{"-text", "'-text"},
		{"-", "'-"},
		{"-42", "-42"},
		{"+42", "'+42"}, // plus is always a vector; only negative numbers pass
		{"plain", "plain"},
		{"", ""},
		{"0", "0"},
		{"-Infinity", "'-Infinity"},
	}
	for _, tc := range cases {
		if got := guardFormula(tc.in); got != tc.want {
			t.Errorf("guardFormula(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCellString(t *testing.T) {
	raw := json.RawMessage(`{"a":1}`)
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		in   any
		want string
		null bool
	}{
		{nil, "", true},
		{"9223372036854775807", "9223372036854775807", false},
		{raw, `{"a":1}`, false},
		{true, "true", false},
		{now, "2024-01-02T03:04:05Z", false},
		{1.5, "1.5", false},
		{[]byte("abc"), "abc", false},
	}
	for _, tc := range cases {
		got, null := cellString(tc.in)
		if null != tc.null || got != tc.want {
			t.Errorf("cellString(%v) = (%q,%v), want (%q,%v)", tc.in, got, null, tc.want, tc.null)
		}
	}
}

func TestCRLFAndFormulaPreserved(t *testing.T) {
	var buf bytes.Buffer
	sink := NewCSV(&buf, true).(*csvSink)
	_ = sink.Begin([]model.QueryColumn{
		{Name: "id", Type: "BIGINT"},
		{Name: "=note", Type: "TEXT"}, // header guard
	})
	_ = sink.Write([]any{"9007199254740993", "=1+1"})
	_ = sink.Write([]any{nil, "-42"})
	_ = sink.Write([]any{"3", "a,b\"c\nd"})
	if err := sink.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\r\n") {
		t.Errorf("output must use CRLF lines, got %q", out)
	}
	wantHeader := `id,'=note`
	want := []string{
		`9007199254740993,'=1+1`,
		`,-42`,
		`3,"a,b""c`,
	}
	if !strings.Contains(out, wantHeader) {
		t.Errorf("missing guarded header %q in %q", wantHeader, out)
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing CSV line %q in output:\n%s", w, out)
		}
	}
}

func TestCSVWithoutHeaders(t *testing.T) {
	var buf bytes.Buffer
	sink := NewCSV(&buf, false).(*csvSink)
	_ = sink.Begin([]model.QueryColumn{{Name: "id", Type: "INTEGER"}})
	_ = sink.Write([]any{"1"})
	_ = sink.End()
	if strings.Contains(buf.String(), "id") {
		t.Errorf("no-header export must not emit names: %q", buf.String())
	}
	if buf.String() != "1\r\n" {
		t.Errorf("got %q, want 1\\r\\n", buf.String())
	}
}

func TestSQLQuotingPerDialect(t *testing.T) {
	sink := &sqlSink{opts: SQLOptions{Driver: "postgres"}}
	if got := sink.quoteString("it's"); got != `'it''s'` {
		t.Errorf("pg quote = %q", got)
	}
	if got := sink.quoteString(`a\b`); got != `'a\b'` {
		t.Errorf("pg must not double backslashes: %q", got)
	}
	mysql := &sqlSink{opts: SQLOptions{Driver: "mysql"}}
	if got := mysql.quoteString(`a\b`); got != `'a\\b'` {
		t.Errorf("mysql must double backslashes: %q", got)
	}
	if got := mysql.quoteString("it's"); got != `'it''s'` {
		t.Errorf("mysql quote = %q", got)
	}
}

func TestSQLLiterals(t *testing.T) {
	cases := []struct {
		driver string
		typ    string
		value  any
		want   string
	}{
		{"postgres", "int8", nil, "NULL"},
		{"postgres", "bool", true, "TRUE"},
		{"postgres", "bool", false, "FALSE"},
		{"postgres", "bytea", "QUJD", "decode('QUJD','base64')"},
		{"postgres", "jsonb", json.RawMessage(`{"a":1}`), `'{"a":1}'`},
		{"postgres", "varchar", "it's", `'it''s'`},
		{"postgres", "numeric", "123.4500", `'123.4500'`},
		{"mysql", "BOOL", true, "1"},
		{"mysql", "BOOL", false, "0"},
		{"mysql", "BLOB", "QUJD", "X'414243'"},
		{"mysql", "VARBINARY", "QUJD", "X'414243'"},
		{"mysql", "VARCHAR", "o'brien", `'o''brien'`},
		{"mysql", "DECIMAL", "3.14", `'3.14'`},
		{"sqlite", "BLOB", "QUJD", "X'414243'"},
		{"sqlite", "TEXT", "hi", `'hi'`},
	}
	for _, tc := range cases {
		s := &sqlSink{opts: SQLOptions{Driver: tc.driver}}
		if got := s.sqlLiteral(tc.typ, tc.value); got != tc.want {
			t.Errorf("sqlLiteral(%s,%s,%v) = %q, want %q", tc.driver, tc.typ, tc.value, got, tc.want)
		}
	}
}

func TestSQLScriptStructure(t *testing.T) {
	var buf bytes.Buffer
	sink := NewSQL(&buf, SQLOptions{Driver: "postgres", Schema: "public", Table: "t", Transaction: true}).(*sqlSink)
	_ = sink.Begin([]model.QueryColumn{
		{Name: "id", Type: "BIGINT"},
		{Name: "note", Type: "TEXT"},
	})
	_ = sink.Write([]any{"1", "hi"})
	_ = sink.Write([]any{"2", nil})
	if err := sink.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`-- data for "public"."t"`,
		"BEGIN;",
		`INSERT INTO "public"."t" ("id", "note") VALUES ('1', 'hi');`,
		`INSERT INTO "public"."t" ("id", "note") VALUES ('2', NULL);`,
		"COMMIT;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SQL output missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "INSERT INTO") != 2 {
		t.Errorf("expected 2 INSERT statements, got %q", out)
	}
}

func TestSQLTransactionDisabled(t *testing.T) {
	var buf bytes.Buffer
	sink := NewSQL(&buf, SQLOptions{Driver: "mysql", Schema: "", Table: "t"}).(*sqlSink)
	_ = sink.Begin([]model.QueryColumn{{Name: "id", Type: "INT"}})
	_ = sink.End()
	if strings.Contains(buf.String(), "TRANSACTION") || strings.Contains(buf.String(), "COMMIT") {
		t.Errorf("no-transaction script must not wrap: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "`t`") {
		t.Errorf("mysql identifiers must be backtick-quoted: %q", buf.String())
	}
}

func TestXLSXRoundTripPreservesText(t *testing.T) {
	var buf bytes.Buffer
	sink := NewXLSX(&buf).(*xlsxSink)
	_ = sink.Begin([]model.QueryColumn{
		{Name: "id", Type: "BIGINT"},
		{Name: "payload", Type: "TEXT"},
	})
	_ = sink.Write([]any{"9223372036854775807", "=1+1"})
	_ = sink.Write([]any{nil, "-42"})
	if err := sink.End(); err != nil {
		t.Fatalf("End: %v", err)
	}

	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("reopen workbook: %v", err)
	}
	defer f.Close()

	// The injected "=1+1" must be plain inline text, NOT a live formula.
	if formula, _ := f.GetCellFormula("Sheet1", "B2"); formula != "" {
		t.Errorf("B2 must not be a formula cell, got %q", formula)
	}
	for cell, want := range map[string]string{
		"A1": "id",
		"B1": "payload",
		"A2": "9223372036854775807",
		"B2": "=1+1",
		"A3": "", // NULL → blank
		"B3": "-42",
	} {
		got, err := f.GetCellValue("Sheet1", cell)
		if err != nil {
			t.Fatalf("GetCellValue(%s): %v", cell, err)
		}
		if got != want {
			t.Errorf("cell %s = %q, want %q", cell, got, want)
		}
	}
}

func TestXLSXTypedCells(t *testing.T) {
	var buf bytes.Buffer
	sink := NewXLSX(&buf).(*xlsxSink)
	_ = sink.Begin([]model.QueryColumn{{Name: "b", Type: "BOOL"}, {Name: "n", Type: "FLOAT"}})
	_ = sink.Write([]any{true, 1.5})
	if err := sink.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer f.Close()
	if _, err := f.GetCellType("Sheet1", "A2"); err != nil {
		t.Errorf("GetCellType(A2): %v", err)
	}
	v, _ := f.GetCellValue("Sheet1", "B2")
	if v != "1.5" {
		t.Errorf("B2 = %q, want 1.5", v)
	}
}

func TestXLSXRowLimit(t *testing.T) {
	var buf bytes.Buffer
	sink := NewXLSX(&buf).(*xlsxSink)
	sink.row = excelize.TotalRows - 2
	if err := sink.Begin(nil); err != nil {
		t.Fatalf("Begin hit limit too early: %v", err)
	}
	if err := sink.Write([]any{"x"}); err != nil {
		t.Fatalf("Write hit limit too early: %v", err)
	}
	if err := sink.Write([]any{"y"}); !errors.Is(err, ErrRowLimit) {
		t.Errorf("Write after limit = %v, want ErrRowLimit", err)
	}
}

func TestBaseTypeNormalization(t *testing.T) {
	for in, want := range map[string]string{
		"varchar(255)":      "varchar",
		"DECIMAL(10,2)":     "decimal",
		"timestamp with tz": "timestamp",
		"BYTEA":             "bytea",
		"int":               "int",
	} {
		if got := baseType(in); got != want {
			t.Errorf("baseType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteIdentifierExport(t *testing.T) {
	if got := quoteIdentifier(`"`, `weird"name`); got != `"weird""name"` {
		t.Errorf("pg quote = %q", got)
	}
	if got := quoteIdentifier("`", "a`b"); got != "`a``b`" {
		t.Errorf("mysql quote = %q", got)
	}
}