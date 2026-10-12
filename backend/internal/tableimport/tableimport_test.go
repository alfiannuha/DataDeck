package tableimport

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

type parsed struct {
	relation string
	cols     []string
	n        int
	rows     [][]model.ImportCell
	err      error
}

func parseCSV(src string, opts CSVOptions) parsed {
	var p parsed
	p.cols, p.n, p.err = ParseCSV(strings.NewReader(src), opts, func(row []model.ImportCell) error {
		r := make([]model.ImportCell, len(row))
		copy(r, row)
		p.rows = append(p.rows, r)
		return nil
	})
	return p
}

func parseSQL(src string, opts SQLOptions) parsed {
	var p parsed
	p.relation, p.cols, p.n, p.err = ParseSQLInserts(strings.NewReader(src), opts, func(row []model.ImportCell) error {
		r := make([]model.ImportCell, len(row))
		copy(r, row)
		p.rows = append(p.rows, r)
		return nil
	})
	return p
}

func TestParseCSVQuotedness(t *testing.T) {
	// ADR-010 §13: unquoted empty → NULL, quoted "" → empty string.
	p := parseCSV("NULL,empty,quoted,guard\r\n,,\"\",'=1+2\r\n", CSVOptions{HeaderRow: true})
	if p.err != nil {
		t.Fatalf("ParseCSV: %v", p.err)
	}
	if !reflect.DeepEqual(p.cols, []string{"NULL", "empty", "quoted", "guard"}) {
		t.Fatalf("columns = %v", p.cols)
	}
	if p.n != 1 || len(p.rows) != 1 {
		t.Fatalf("rows = %v, n = %d", p.rows, p.n)
	}
	want := []model.ImportCell{
		{IsNull: true},
		{IsNull: true},
		{Value: ""},     // quoted "" → empty string
		{Value: "=1+2"}, // guarded "'=1+2" unguarded on import
	}
	if !reflect.DeepEqual(p.rows[0], want) {
		t.Fatalf("row = %#v, want %#v", p.rows[0], want)
	}
}

func TestParseCSVWidthAndBlanks(t *testing.T) {
	// Blank lines are skipped; width violations are clear errors.
	p := parseCSV("a,b\n\n1,2\n\n3,4\n", CSVOptions{HeaderRow: true})
	if p.err != nil {
		t.Fatalf("ParseCSV: %v", p.err)
	}
	if p.n != 2 || !reflect.DeepEqual(p.cols, []string{"a", "b"}) {
		t.Fatalf("cols = %v, n = %d", p.cols, p.n)
	}

	bad := parseCSV("a,b\n1,2,3\n", CSVOptions{HeaderRow: true})
	if bad.err == nil || !strings.Contains(bad.err.Error(), "row 2") {
		t.Fatalf("width error = %v", bad.err)
	}
}

func TestParseCSVUnterminatedQuote(t *testing.T) {
	p := parseCSV("a,b\n1,\"oops\n", CSVOptions{HeaderRow: true})
	if !errors.Is(p.err, ErrUnterminatedQuote) {
		t.Fatalf("want ErrUnterminatedQuote, got %v", p.err)
	}
}

func TestParseCSVTrailingDelimiterField(t *testing.T) {
	// A record ending in a delimiter still has its final empty field (unquoted
	// empty → NULL). Regression: it used to be dropped, shifting width.
	p := parseCSV("a,b,c,d\r\n1,2,3,x\r\n2,Bob,false,\r\n3,Cara,,2025-05-06\r\n", CSVOptions{HeaderRow: true})
	if p.err != nil {
		t.Fatalf("ParseCSV: %v", p.err)
	}
	if p.n != 3 {
		t.Fatalf("n = %d, want 3", p.n)
	}
	want := [][]model.ImportCell{
		{{Value: "2"}, {Value: "Bob"}, {Value: "false"}, {IsNull: true}},
		{{Value: "3"}, {Value: "Cara"}, {IsNull: true}, {Value: "2025-05-06"}},
	}
	if !reflect.DeepEqual(p.rows[1:], want) {
		t.Fatalf("rows = %#v, want %#v", p.rows[1:], want)
	}
}

func TestParseSQLInsertOnly(t *testing.T) {
	src := `-- data for "schema"."users"
BEGIN;
INSERT INTO "schema"."users" (id, name, joined, note) VALUES
  (1, 'Ada', TRUE, NULL),
  (2, 'it''s ''ok''', -3.5e2, '');
COMMIT;
`
	p := parseSQL(src, SQLOptions{Dialect: "postgres"})
	if p.err != nil {
		t.Fatalf("ParseSQLInserts: %v", p.err)
	}
	if p.relation != `schema.users` {
		t.Fatalf("relation = %q", p.relation)
	}
	if !reflect.DeepEqual(p.cols, []string{"id", "name", "joined", "note"}) {
		t.Fatalf("columns = %v", p.cols)
	}
	if p.n != 2 || len(p.rows) != 2 {
		t.Fatalf("rows = %d, n = %d", len(p.rows), p.n)
	}
	if !reflect.DeepEqual(p.rows[0], []model.ImportCell{{Value: "1"}, {Value: "Ada"}, {Value: "true"}, {IsNull: true}}) {
		t.Fatalf("row 0 = %#v", p.rows[0])
	}
	if !reflect.DeepEqual(p.rows[1], []model.ImportCell{{Value: "2"}, {Value: "it's 'ok'"}, {Value: "-3.5e2"}, {Value: ""}}) {
		t.Fatalf("row 1 = %#v", p.rows[1])
	}
}

func TestParseSQLRejectsStatements(t *testing.T) {
	cases := []string{
		"DELETE FROM t;",
		"UPDATE t SET a=1;",
		"CREATE TABLE t (a int);",
		"SELECT 1;",
		"COPY t FROM 'x';",
		"PRAGMA user_version;",
		"CALL foo();",
		"INSERT INTO t VALUES (1, lower('X'));",
		"INSERT INTO t VALUES (1 || 2);",
		"INSERT INTO t VALUES ((SELECT 1));",
		"INSERT INTO t VALUES (uuid('x'));",
		"DROP TABLE t;",
		"INSERT INTO a VALUES (1); INSERT INTO b VALUES (2);",
		"INSERT INTO t (a) VALUES (1, 2);",
		"INSERT INTO t VALUES (1) garbage;",
	}
	for _, src := range cases {
		p := parseSQL(src, SQLOptions{Dialect: "postgres"})
		if p.err == nil {
			t.Errorf("expected rejection of %q", src)
		}
	}
}

func TestParseSQLHexAndDecode(t *testing.T) {
	// X'hex' (MySQL/SQLite blobs) and decode(...,'base64') (PG bytea) are the
	// exporter's value literals and must parse back to the canonical string.
	p := parseSQL("INSERT INTO t VALUES (X'DEADBEEF', decode('AQID', 'base64'));", SQLOptions{Dialect: "mysql"})
	if p.err != nil {
		t.Fatalf("ParseSQLInserts: %v", p.err)
	}
	want := []model.ImportCell{{Value: "DEADBEEF"}, {Value: "AQID"}}
	if !reflect.DeepEqual(p.rows[0], want) {
		t.Fatalf("row = %#v, want %#v", p.rows[0], want)
	}
}

func TestUnguardFormula(t *testing.T) {
	cases := map[string]string{
		"'=1+2":  "=1+2",
		"'+foo":  "+foo",
		"'@x":    "@x",
		"'-abc":  "-abc",
		"'-42":   "'-42", // exporter never guards "-42"; leave unchanged
		"'-42.5": "'-42.5",
		"'hello": "'hello", // legit leading quote, not a formula marker
		"-42":    "-42",
		"-42.5":  "-42.5",
	}
	for in, want := range cases {
		if got := unguardFormula(in, false); got != want {
			t.Errorf("unguardFormula(%q) = %q, want %q", in, got, want)
		}
	}
	if got := unguardFormula("anything", true); got != "anything" {
		t.Errorf("null value = %q", got)
	}
}
