package database

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestEncodeValueBigintAsString(t *testing.T) {
	cases := map[int64]string{
		0:                "0",
		42:               "42",
		9007199254740993: "9007199254740993", // 2^53 + 1, not representable as float64
		math.MaxInt64:    "9223372036854775807",
		math.MinInt64:    "-9223372036854775808",
	}
	for value, want := range cases {
		got, ok := encodeValue("int8", value).(string)
		if !ok {
			t.Fatalf("encodeValue(int8, %d) = %T, want string", value, got)
		}
		if got != want {
			t.Errorf("encodeValue(int8, %d) = %q, want %q", value, got, want)
		}
	}
}

func TestEncodeValueScalars(t *testing.T) {
	if got := encodeValue("text", nil); got != nil {
		t.Errorf("nil = %v, want nil", got)
	}
	if got := encodeValue("bool", true); got != true {
		t.Errorf("bool = %v, want true", got)
	}
	if got := encodeValue("float8", 1.5); got != 1.5 {
		t.Errorf("float8 = %v, want 1.5", got)
	}
	if got := encodeValue("int4", int32(5)); got != int32(5) {
		t.Errorf("int4 = %v (%T), want int32(5)", got, got)
	}
	if got := encodeValue("varchar", "hello"); got != "hello" {
		t.Errorf("varchar = %v, want hello", got)
	}
}

func TestEncodeValueNumeric(t *testing.T) {
	numeric := pgtype.Numeric{Int: big.NewInt(12345), Exp: -2, Valid: true}
	got := encodeValue("numeric", numeric)
	if got != "123.45" {
		t.Errorf("numeric = %v, want \"123.45\"", got)
	}
}

func TestEncodeValueUUID(t *testing.T) {
	raw := [16]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	got, ok := encodeValue("uuid", raw).(string)
	if !ok {
		t.Fatalf("uuid = %T, want string", got)
	}
	if want := "00112233-4455-6677-8899-aabbccddeeff"; got != want {
		t.Errorf("uuid = %q, want %q", got, want)
	}
}

func TestEncodeValueJSON(t *testing.T) {
	got := encodeValue("jsonb", map[string]any{"role": "admin"})
	raw, ok := got.(json.RawMessage)
	if !ok {
		t.Fatalf("jsonb = %T, want json.RawMessage", got)
	}
	if string(raw) != `{"role":"admin"}` {
		t.Errorf("jsonb = %s", raw)
	}
}

func TestEncodeValueByteaIsBase64(t *testing.T) {
	input := []byte{0x00, 0x01, 0xff}
	got, ok := encodeValue("bytea", input).(string)
	if !ok {
		t.Fatalf("bytea = %T, want string", got)
	}
	if want := base64.StdEncoding.EncodeToString(input); got != want {
		t.Errorf("bytea = %q, want %q", got, want)
	}
}

func TestEncodeValueTemporal(t *testing.T) {
	ts := time.Date(2026, 9, 25, 14, 32, 0, 0, time.UTC)
	if got := encodeValue("timestamptz", ts); got != "2026-09-25T14:32:00Z" {
		t.Errorf("timestamptz = %v", got)
	}
	if got := encodeValue("timestamp", ts); got != "2026-09-25T14:32:00Z" {
		t.Errorf("timestamp = %v", got)
	}
	if got := encodeValue("date", ts); got != "2026-09-25" {
		t.Errorf("date = %v", got)
	}
}

func TestEncodeValueRawBytesFallback(t *testing.T) {
	if got := encodeValue("unknown", []byte("hello")); got != "hello" {
		t.Errorf("utf8 bytes = %v, want hello", got)
	}
	if got, ok := encodeValue("unknown", []byte{0xff, 0xfe}).(string); !ok || got != base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe}) {
		t.Errorf("non-utf8 bytes = %v, want base64", got)
	}
}

func TestResultBufferTruncates(t *testing.T) {
	buffer := newResultBuffer(20)

	if !buffer.add([]any{"a"}) {
		t.Fatal("small row should fit")
	}
	if buffer.add([]any{"bbbbbbbbbbbbbbbbbbbb"}) {
		t.Fatal("oversized row should be rejected")
	}
	if len(buffer.rows) != 1 {
		t.Errorf("rows = %d, want 1", len(buffer.rows))
	}
	if buffer.size > buffer.max {
		t.Errorf("size = %d exceeds max = %d", buffer.size, buffer.max)
	}
}
