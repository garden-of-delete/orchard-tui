package components

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// dangerousControlBytes are C0 control bytes plus DEL, minus the
// pretty-print whitespace (\t, \n) that the JSON encoder is allowed
// to emit. ESC, BEL, CR, etc. are still flagged.
const dangerousControlBytes = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f"

// escSeq is the 6 ASCII bytes that make up a proper RFC 8259 escape
// for U+001B (ESC): a backslash, 'u', and the four hex digits '001b'.
// Built byte-by-byte so the test source itself contains no literal
// ESC bytes.
var escSeq = []byte{0x5c, 0x75, 0x30, 0x30, 0x31, 0x62}

func TestPrettyJSONEmpty(t *testing.T) {
	if got := PrettyJSON(nil); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
	if got := PrettyJSON(json.RawMessage("")); got != "" {
		t.Errorf("empty = %q, want empty", got)
	}
}

func TestPrettyJSONReformatsAndSorts(t *testing.T) {
	in := json.RawMessage(`{"b":2,"a":1,"nested":{"y":"v","x":"u"}}`)
	got := PrettyJSON(in)
	want := "{\n  \"a\": 1,\n  \"b\": 2,\n  \"nested\": {\n    \"x\": \"u\",\n    \"y\": \"v\"\n  }\n}"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrettyJSONReescapesLiteralControlBytes(t *testing.T) {
	// RFC 8259 forbids unescaped control bytes inside strings; defend
	// against a non-conformant or malicious producer. We construct raw
	// bytes that contain a literal ESC (0x1b) inside a string value.
	raw := json.RawMessage([]byte("{\"x\":\"a\x1b[2Jb\"}"))
	got := PrettyJSON(raw)
	if strings.ContainsAny(got, dangerousControlBytes) {
		t.Fatalf("output contains control bytes: %q", got)
	}
	if !strings.Contains(got, "a") || !strings.Contains(got, "[2Jb") {
		t.Errorf("output unexpectedly altered surrounding content: %q", got)
	}
}

func TestPrettyJSONWellFormedEscapeStaysEscaped(t *testing.T) {
	// Properly RFC-escaped input: the 6 source bytes are escSeq. After
	// round-trip, the output must still contain the 6-byte escape
	// sequence and never a literal ESC.
	raw := []byte(`{"x":"a`)
	raw = append(raw, escSeq...)
	raw = append(raw, []byte(`[2Jb"}`)...)
	got := PrettyJSON(json.RawMessage(raw))

	if strings.ContainsAny(got, dangerousControlBytes) {
		t.Fatalf("output contains control bytes: %q", got)
	}
	if !bytes.Contains([]byte(got), escSeq) {
		t.Errorf("expected escape sequence preserved in output, got %q", got)
	}
}

func TestPrettyJSONPreservesIntegerPrecision(t *testing.T) {
	// 2^60 — past float64's safe integer range (2^53). Without
	// UseNumber, decoding into any picks float64 and loses precision.
	in := json.RawMessage(`{"x":1152921504606846976}`)
	got := PrettyJSON(in)
	if !strings.Contains(got, "1152921504606846976") {
		t.Errorf("integer precision lost: %q", got)
	}
}

func TestPrettyJSONKeepsHTMLLiterals(t *testing.T) {
	// SetEscapeHTML(false) keeps these readable rather than <-encoded.
	in := json.RawMessage(`{"x":"<a href=\"u\">t</a>","y":"a&b"}`)
	got := PrettyJSON(in)
	if !strings.Contains(got, `<a href=\"u\">t</a>`) {
		t.Errorf("HTML chars escaped: %q", got)
	}
	if !strings.Contains(got, `a&b`) {
		t.Errorf("ampersand escaped: %q", got)
	}
}

func TestPrettyJSONInvalidFallbackIsSafe(t *testing.T) {
	// Broken JSON falls through to the Sanitize fallback.
	raw := json.RawMessage([]byte("not\x1bjson"))
	got := PrettyJSON(raw)
	if strings.ContainsAny(got, dangerousControlBytes) {
		t.Errorf("fallback leaked control bytes: %q", got)
	}
}
