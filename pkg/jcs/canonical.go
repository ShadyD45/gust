package jcs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	ErrInvalidUTF8 = errors.New("jcs: invalid UTF-8 string")
	ErrInfinite    = errors.New("jcs: NaN or Infinity are not valid JSON numbers")
)

// Canonicalize takes arbitrary JSON data and converts it to its RFC 8785 canonical form.
func Canonicalize(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("jcs: empty input")
	}

	var raw any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("jcs decode error: %w", err)
	}

	var buf bytes.Buffer
	if err := serializeValue(&buf, raw); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ContentHash computes the SHA-256 hash of the RFC 8785 canonical representation of v.
// It returns a string in the format "sha256:<64-char-hex>".
func ContentHash(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("jcs: failed to marshal input: %w", err)
	}
	canonical, err := Canonicalize(data)
	if err != nil {
		return "", fmt.Errorf("jcs: failed to canonicalize: %w", err)
	}
	h := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

func serializeValue(buf *bytes.Buffer, val any) error {
	switch v := val.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case string:
		return serializeString(buf, v)
	case json.Number:
		return serializeNumber(buf, v)
	case float64:
		return serializeFloat(buf, v)
	case []any:
		return serializeArray(buf, v)
	case map[string]any:
		return serializeObject(buf, v)
	default:
		// Fallback for custom Go types
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return serializeValue(buf, raw)
	}
}

func serializeString(buf *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return ErrInvalidUTF8
	}
	buf.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\\':
			buf.WriteString(`\\`)
		case '"':
			buf.WriteString(`\"`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteString(s[i : i+size])
			}
		}
		i += size
	}
	buf.WriteByte('"')
	return nil
}

func serializeNumber(buf *bytes.Buffer, num json.Number) error {
	f, err := num.Float64()
	if err != nil {
		return err
	}
	return serializeFloat(buf, f)
}

func serializeFloat(buf *bytes.Buffer, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return ErrInfinite
	}
	// Check for negative zero
	if f == 0 {
		buf.WriteByte('0')
		return nil
	}

	// ECMAScript Number.prototype.toString formatting
	s := strconv.FormatFloat(f, 'g', -1, 64)

	// In Go, 'g' may emit exponents like 'e+05' or 'e-05'.
	// RFC 8785 mandates lowercase 'e' without redundant '+'.
	s = normalizeExponent(s)
	buf.WriteString(s)
	return nil
}

func normalizeExponent(s string) string {
	idx := bytes.IndexByte([]byte(s), 'e')
	if idx == -1 {
		idx = bytes.IndexByte([]byte(s), 'E')
		if idx != -1 {
			s = s[:idx] + "e" + s[idx+1:]
		}
	}
	if idx == -1 {
		return s
	}

	// If there is 'e+', remove '+'
	if idx+1 < len(s) && s[idx+1] == '+' {
		s = s[:idx+1] + s[idx+2:]
	}
	return s
}

func serializeArray(buf *bytes.Buffer, arr []any) error {
	buf.WriteByte('[')
	for i, item := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := serializeValue(buf, item); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

func serializeObject(buf *bytes.Buffer, obj map[string]any) error {
	buf.WriteByte('{')

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}

	// Sort keys by UTF-16 code units according to RFC 8785
	sort.Slice(keys, func(i, j int) bool {
		return compareUTF16(keys[i], keys[j]) < 0
	})

	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := serializeString(buf, k); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := serializeValue(buf, obj[k]); err != nil {
			return err
		}
	}

	buf.WriteByte('}')
	return nil
}

// compareUTF16 compares two strings by their UTF-16 code unit values.
func compareUTF16(a, b string) int {
	u16a := utf16.Encode([]rune(a))
	u16b := utf16.Encode([]rune(b))

	minLen := len(u16a)
	if len(u16b) < minLen {
		minLen = len(u16b)
	}

	for i := 0; i < minLen; i++ {
		if u16a[i] < u16b[i] {
			return -1
		}
		if u16a[i] > u16b[i] {
			return 1
		}
	}

	if len(u16a) < len(u16b) {
		return -1
	}
	if len(u16a) > len(u16b) {
		return 1
	}
	return 0
}
