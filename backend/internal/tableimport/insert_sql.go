package tableimport

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Restricted SQL import parsing (PRF-02/T15). The accepted grammar is narrow
// and INSERT-only; anything else is rejected with a position. Multiple INSERT
// statements are allowed but every statement must target the same relation with
// the same column list, so the whole script imports into one table. T14 SQL
// exports wrap their script in a transaction, so BEGIN / START TRANSACTION /
// COMMIT are accepted as ignorable transaction-control statements for
// export→import round-trips; no other control or data-changing statement is.

var (
	// ErrNonInsertStatement marks SQL that is not a single-table INSERT.
	ErrNonInsertStatement = errors.New("tableimport: only single-table INSERT statements are allowed")
)

// SQLOptions controls restricted SQL parsing.
type SQLOptions struct {
	// Dialect is one of "postgres", "mysql", "sqlite" and selects string
	// escaping rules (MySQL backslash escapes) and hex/decode literal support.
	Dialect string
	// MaxRows bounds the total number of rows parsed across the script.
	MaxRows int
	// MaxRowBytes bounds the encoded size of a single VALUES row.
	MaxRowBytes int
}

const defaultSQLMaxBytes = 50 << 20 // 50 MiB

// ParseSQLInserts streams the VALUES rows of an SQL script. It returns the
// target relation as written (unquoted for display; dot-joined when
// schema-qualified) and the explicit column list (nil when statements use the
// implied full-column form), and invokes emit for every parsed row. The parsed
// cell keeps the source string plus a NULL flag; the database layer converts it
// to the target column type.
//
// Scripts that mix tables, mismatched column lists, non-literal expressions,
// or any DDL/DML/CALL/COPY/PRAGMA/etc. statement are rejected.
func ParseSQLInserts(r io.Reader, opts SQLOptions, emit func(row []model.ImportCell) error) (relation string, columns []string, rowCount int, err error) {
	data, err := readBounded(r, defaultSQLMaxBytes)
	if err != nil {
		return "", nil, 0, err
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = 1_000_000
	}
	if opts.MaxRowBytes <= 0 {
		opts.MaxRowBytes = 1 << 20 // 1 MiB
	}

	p := &sqlParser{tk: newSQLTokenizer(data, opts.Dialect), emit: emit, maxRows: opts.MaxRows, maxRowBytes: opts.MaxRowBytes}

	for {
		tok := p.tk.next()
		switch {
		case tok.kind == tkEOF:
			return p.relation, p.columns, p.rowCount, nil
		case tok.isPunct(";"):
			continue // tolerate empty statements / trailing separators
		case tok.kind == tkWord && (kwIn(tok.text, "begin", "start", "commit")):
			if err := p.skipTransactionControl(tok); err != nil {
				return p.relation, p.columns, p.rowCount, err
			}
		case tok.kind == tkWord && kwIn(tok.text, "insert"):
			if err := p.parseInsert(); err != nil {
				return p.relation, p.columns, p.rowCount, err
			}
		default:
			return p.relation, p.columns, p.rowCount, fmt.Errorf("sql: unexpected %s at offset %d: %w", tok.describe(), tok.offset, ErrNonInsertStatement)
		}
	}
}

func readBounded(r io.Reader, max int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > max {
		return nil, fmt.Errorf("sql: script exceeds the %d byte limit", max)
	}
	return data, nil
}

// skipTransactionControl consumes BEGIN, BEGIN WORK/TRANSACTION,
// START TRANSACTION, or COMMIT followed by ';'.
func (p *sqlParser) skipTransactionControl(first token) error {
	tok := p.tk.next()
	if strings.EqualFold(first.text, "start") {
		if !(tok.kind == tkWord && strings.EqualFold(tok.text, "transaction")) {
			return p.errpos(tok, "expected TRANSACTION after START")
		}
		tok = p.tk.next()
	} else if tok.kind == tkWord && (strings.EqualFold(tok.text, "work") || strings.EqualFold(tok.text, "transaction")) {
		tok = p.tk.next()
	}
	if !tok.isPunct(";") {
		return p.errpos(tok, "expected ; after transaction control")
	}
	return nil
}

// parseInsert parses INSERT [INTO] <qualified> [(<cols>)] VALUES <row-list>.
func (p *sqlParser) parseInsert() error {
	tok := p.tk.next()
	if tok.kind == tkWord && kwIn(tok.text, "into") {
		tok = p.tk.next()
	}
	relation, ok := p.qualifiedName(tok)
	if !ok {
		return p.errpos(tok, "expected table name after INSERT INTO")
	}
	if p.relation == "" {
		p.relation = relation
	} else if !strings.EqualFold(p.relation, relation) {
		return p.errpos(tok, "all INSERT statements must target the same table")
	}

	tok = p.tk.next()
	cols := []string(nil)
	if tok.isPunct("(") {
		var err error
		cols, err = p.columnList(true)
		if err != nil {
			return err
		}
		tok = p.tk.next()
	}
	if p.columns == nil {
		p.columns = cols
	} else if !equalFold(p.columns, cols) {
		return fmt.Errorf("sql: INSERT column list differs from the previous statement")
	}

	if !(tok.kind == tkWord && strings.EqualFold(tok.text, "values")) {
		return p.errpos(tok, "expected VALUES")
	}

	for {
		tok = p.tk.next()
		if !tok.isPunct("(") {
			return p.errpos(tok, "expected (")
		}
		row, err := p.valueRow()
		if err != nil {
			return err
		}
		if len(p.columns) > 0 && len(row) != len(p.columns) {
			return fmt.Errorf("sql: VALUES row has %d values, expected %d", len(row), len(p.columns))
		}
		if err := p.emit(row); err != nil {
			return err
		}
		p.rowCount++
		if p.rowCount > p.maxRows {
			return fmt.Errorf("sql: script exceeds the %d row limit", p.maxRows)
		}
		tok = p.tk.next()
		switch {
		case tok.isPunct(","):
			continue
		case tok.isPunct(";"), tok.kind == tkEOF:
			return nil
		default:
			return p.errpos(tok, "expected , or ; after VALUES row")
		}
	}
}

// columnList parses a parenthesized identifier list; opening '(' already read.
func (p *sqlParser) columnList(open bool) ([]string, error) {
	var cols []string
	for {
		tok := p.tk.next()
		if tok.isPunct(")") {
			return cols, nil
		}
		name, ok := p.identifier(tok)
		if !ok {
			return nil, p.errpos(tok, "expected column name")
		}
		cols = append(cols, name)
		tok = p.tk.next()
		if tok.isPunct(",") {
			continue
		}
		if tok.isPunct(")") {
			return cols, nil
		}
		return nil, p.errpos(tok, "expected , or ) in column list")
	}
}

func (p *sqlParser) valueRow() ([]model.ImportCell, error) {
	row := []model.ImportCell(nil)
	rowBytes := 0
	for {
		tok := p.tk.next()
		switch {
		case tok.isPunct(")"):
			return row, nil
		case tok.kind == tkWord && kwIn(tok.text, "null"):
			row = append(row, model.ImportCell{IsNull: true})
		case tok.kind == tkWord && kwIn(tok.text, "true"):
			row = append(row, model.ImportCell{Value: "true"})
		case tok.kind == tkWord && kwIn(tok.text, "false"):
			row = append(row, model.ImportCell{Value: "false"})
		case tok.kind == tkWord && strings.EqualFold(tok.text, "decode"):
			v, err := p.decodeBase64()
			if err != nil {
				return nil, err
			}
			row = append(row, model.ImportCell{Value: v})
		case tok.kind == tkString, tok.kind == tkHex, tok.kind == tkNumber:
			row = append(row, model.ImportCell{Value: tok.text})
			switch tok.kind {
			case tkString:
				rowBytes += len(tok.text)
			case tkHex:
				rowBytes += len(tok.text) / 2
			default:
				rowBytes += len(tok.text)
			}
		default:
			return nil, p.errpos(tok, "expected a literal in VALUES")
		}
		if rowBytes > p.maxRowBytes {
			return nil, fmt.Errorf("sql: VALUES row exceeds the %d byte limit", p.maxRowBytes)
		}
		tok = p.tk.next()
		if tok.isPunct(",") {
			continue
		}
		if tok.isPunct(")") {
			return row, nil
		}
		return nil, p.errpos(tok, "expected , or ) in VALUES row")
	}
}

// decodeBase64 consumes decode('<base64>','base64') — the PostgreSQL bytea
// literal the T14 SQL exporter emits — and returns the base64 payload. Any
// other decode() shape is rejected.
func (p *sqlParser) decodeBase64() (string, error) {
	tok := p.tk.next()
	if !tok.isPunct("(") {
		return "", p.errpos(tok, "expected ( after decode")
	}
	payload, ok := p.expectString()
	if !ok {
		return "", fmt.Errorf("sql: expected base64 payload string at offset %d", p.tk.offset)
	}
	if tok := p.tk.next(); !tok.isPunct(",") {
		return "", p.errpos(tok, "expected , in decode()")
	}
	algo, ok := p.expectString()
	if !ok {
		return "", fmt.Errorf("sql: expected encoding name at offset %d", p.tk.offset)
	}
	if !strings.EqualFold(algo, "base64") {
		return "", fmt.Errorf("sql: unsupported decode() encoding %q", algo)
	}
	if tok := p.tk.next(); !tok.isPunct(")") {
		return "", p.errpos(tok, "expected ) after decode()")
	}
	return payload, nil
}

func (p *sqlParser) expectString() (string, bool) {
	tok := p.tk.next()
	if tok.kind == tkString {
		return tok.text, true
	}
	return "", false
}

func (p *sqlParser) qualifiedName(tok token) (string, bool) {
	first, ok := p.identifier(tok)
	if !ok {
		return "", false
	}
	if !p.tk.peek().isPunct(".") {
		return first, true
	}
	p.tk.next() // consume '.'
	second, ok := p.identifier(p.tk.next())
	if !ok {
		return "", false
	}
	return first + "." + second, true
}

// identifier accepts quoted ("..." or `...`) or bare [A-Za-z_][A-Za-z0-9_$]*.
func (p *sqlParser) identifier(tok token) (string, bool) {
	switch tok.kind {
	case tkQuotedIdent, tkWord:
		return tok.text, true
	default:
		return "", false
	}
}

func (p *sqlParser) errpos(tok token, msg string) error {
	return fmt.Errorf("sql: %s at offset %d", msg, tok.offset)
}

func equalFold(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

func kwIn(s string, kws ...string) bool {
	for _, k := range kws {
		if strings.EqualFold(s, k) {
			return true
		}
	}
	return false
}

// ---- tokenizer ----

type tokenKind int

const (
	tkEOF tokenKind = iota
	tkWord
	tkString
	tkHex
	tkNumber
	tkQuotedIdent
	tkPunct
)

type token struct {
	kind   tokenKind
	text   string
	offset int
}

func (t token) describe() string {
	switch t.kind {
	case tkEOF:
		return "end of input"
	case tkWord:
		return "keyword " + t.text
	case tkString:
		return "string literal"
	case tkHex:
		return "hex literal"
	case tkNumber:
		return "number " + t.text
	case tkQuotedIdent:
		return "identifier " + t.text
	default:
		return t.text
	}
}

func (t token) isPunct(s string) bool { return t.kind == tkPunct && t.text == s }

type sqlParser struct {
	tk          *sqlTokenizer
	emit        func(row []model.ImportCell) error
	relation    string
	columns     []string
	rowCount    int
	maxRows     int
	maxRowBytes int
}

type sqlTokenizer struct {
	buf     []byte
	pos     int
	offset  int
	dialect string

	peeked    bool
	peekedTok token
}

func newSQLTokenizer(data []byte, dialect string) *sqlTokenizer {
	return &sqlTokenizer{buf: data, dialect: dialect}
}

func (t *sqlTokenizer) next() token {
	if t.peeked {
		t.peeked = false
		return t.peekedTok
	}
	return t.scan()
}

// peek returns the next token without consuming it.
func (t *sqlTokenizer) peek() token {
	if !t.peeked {
		t.peekedTok = t.scan()
		t.peeked = true
	}
	return t.peekedTok
}

func (t *sqlTokenizer) scan() token {
	t.skipWhitespaceAndComments()
	if t.pos >= len(t.buf) {
		return token{kind: tkEOF, offset: t.offset}
	}
	start := t.offset
	c := t.buf[t.pos]
	switch {
	case c == '\'':
		return t.stringToken(start)
	case c == '"' || c == '`':
		return t.quotedIdentToken(c, start)
	case (c == 'x' || c == 'X') && t.pos+1 < len(t.buf) && t.buf[t.pos+1] == '\'':
		t.pos++ // consume x
		t.offset++
		return t.hexToken(start)
	case isAlphaWordStart(c):
		return t.wordToken(start)
	case isDigit(c) || c == '.' || c == '-' || c == '+':
		return t.numberToken(start)
	default:
		t.pos++
		t.offset++
		return token{kind: tkPunct, text: string(c), offset: start}
	}
}

func (t *sqlTokenizer) skipWhitespaceAndComments() {
	for t.pos < len(t.buf) {
		c := t.buf[t.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			t.pos++
			t.offset++
			continue
		}
		if c == '-' && t.pos+1 < len(t.buf) && t.buf[t.pos+1] == '-' { // -- comment
			for t.pos < len(t.buf) && t.buf[t.pos] != '\n' {
				t.pos++
				t.offset++
			}
			continue
		}
		if c == '/' && t.pos+1 < len(t.buf) && t.buf[t.pos+1] == '*' { // /* */ comment
			t.pos += 2
			t.offset += 2
			for t.pos+1 < len(t.buf) && !(t.buf[t.pos] == '*' && t.buf[t.pos+1] == '/') {
				t.pos++
				t.offset++
			}
			if t.pos+1 < len(t.buf) {
				t.pos += 2
				t.offset += 2
			}
			continue
		}
		return
	}
}

func (t *sqlTokenizer) wordToken(start int) token {
	i := t.pos
	for i < len(t.buf) && (isAlphaWordContinue(t.buf[i])) {
		i++
	}
	text := string(t.buf[t.pos:i])
	t.advance(i - t.pos)
	return token{kind: tkWord, text: text, offset: start}
}

// numberToken accepts digits, one '.', optional exponent, and an optional
// leading sign. A lone '.' or '-' that is not a number is a punct token.
func (t *sqlTokenizer) numberToken(start int) token {
	i := t.pos
	switch t.buf[i] {
	case '-', '+':
		if i+1 >= len(t.buf) || !isDigit(t.buf[i+1]) {
			return t.punct(start)
		}
		i++
	case '.':
		if i+1 >= len(t.buf) || !isDigit(t.buf[i+1]) {
			return t.punct(start)
		}
	}
	seenDot := false
	if t.buf[i] == '.' {
		seenDot = true
		i++
	}
	for i < len(t.buf) && isDigit(t.buf[i]) {
		i++
	}
	if !seenDot && i < len(t.buf) && t.buf[i] == '.' {
		seenDot = true
		i++
		for i < len(t.buf) && isDigit(t.buf[i]) {
			i++
		}
	}
	if i < len(t.buf) && (t.buf[i] == 'e' || t.buf[i] == 'E') {
		j := i + 1
		if j < len(t.buf) && (t.buf[j] == '+' || t.buf[j] == '-') {
			j++
		}
		k := j
		for k < len(t.buf) && isDigit(t.buf[k]) {
			k++
		}
		if k > j {
			i = k
		}
	}
	text := string(t.buf[t.pos:i])
	t.advance(i - t.pos)
	return token{kind: tkNumber, text: text, offset: start}
}

func (t *sqlTokenizer) punct(start int) token {
	t.pos++
	t.offset++
	return token{kind: tkPunct, text: string(t.buf[start]), offset: start}
}

func (t *sqlTokenizer) stringToken(start int) token {
	t.pos++ // opening '
	t.offset++
	var b strings.Builder
	for t.pos < len(t.buf) {
		c := t.buf[t.pos]
		if c == '\'' {
			if t.pos+1 < len(t.buf) && t.buf[t.pos+1] == '\'' { // '' escapes a quote
				b.WriteByte('\'')
				t.pos += 2
				t.offset += 2
				continue
			}
			t.pos++
			t.offset++
			return token{kind: tkString, text: b.String(), offset: start}
		}
		if c == '\\' && t.dialect == "mysql" && t.pos+1 < len(t.buf) {
			// MySQL backslash escaping: the next character is taken literally.
			b.WriteByte(t.buf[t.pos+1])
			t.pos += 2
			t.offset += 2
			continue
		}
		b.WriteByte(c)
		t.pos++
		t.offset++
	}
	return token{kind: tkString, text: b.String(), offset: start} // unterminated
}

func (t *sqlTokenizer) quotedIdentToken(quote byte, start int) token {
	t.pos++ // opening quote
	t.offset++
	var b strings.Builder
	for t.pos < len(t.buf) {
		c := t.buf[t.pos]
		if c == quote {
			if t.pos+1 < len(t.buf) && t.buf[t.pos+1] == quote { // "" escapes
				b.WriteByte(quote)
				t.pos += 2
				t.offset += 2
				continue
			}
			t.pos++
			t.offset++
			return token{kind: tkQuotedIdent, text: b.String(), offset: start}
		}
		b.WriteByte(c)
		t.pos++
		t.offset++
	}
	return token{kind: tkQuotedIdent, text: b.String(), offset: start} // unterminated
}

func (t *sqlTokenizer) hexToken(start int) token {
	t.pos++ // opening '
	t.offset++
	var b strings.Builder
	for t.pos < len(t.buf) {
		c := t.buf[t.pos]
		if c == '\'' {
			t.pos++
			t.offset++
			return token{kind: tkHex, text: b.String(), offset: start}
		}
		b.WriteByte(c)
		t.pos++
		t.offset++
	}
	return token{kind: tkHex, text: b.String(), offset: start} // unterminated
}

func (t *sqlTokenizer) advance(n int) {
	t.pos += n
	t.offset += n
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAlphaWordStart(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}

func isAlphaWordContinue(b byte) bool {
	return isAlphaWordStart(b) || isDigit(b) || b == '$'
}
