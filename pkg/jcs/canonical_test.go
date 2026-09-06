package jcs

import (
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "whitespace stripping",
			input:    ` {  "b" : 2 ,  "a" : 1 } `,
			expected: `{"a":1,"b":2}`,
		},
		{
			name:     "key ordering",
			input:    `{"z": 1, "a": 2, "m": 3}`,
			expected: `{"a":2,"m":3,"z":1}`,
		},
		{
			name:     "unicode key ordering (RFC 8785 UTF-16 code units)",
			input:    `{"\u00e9": 1, "e": 2}`,
			expected: `{"e":2,"é":1}`,
		},
		{
			name:     "nested objects and arrays",
			input:    `{"outer": {"beta": [3, 2, 1], "alpha": true}}`,
			expected: `{"outer":{"alpha":true,"beta":[3,2,1]}}`,
		},
		{
			name:     "number formatting integer and float",
			input:    `{"int": 100, "float": 100.5, "zero": 0}`,
			expected: `{"float":100.5,"int":100,"zero":0}`,
		},
		{
			name:     "escaping control characters",
			input:    "{\"text\": \"hello\\nworld\\t!\"}",
			expected: "{\"text\":\"hello\\nworld\\t!\"}",
		},
		{
			name:     "do not escape forward slash",
			input:    `{"url": "https:\/\/example.com\/test"}`,
			expected: `{"url":"https://example.com/test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Canonicalize([]byte(tt.input))
			if err != nil {
				t.Fatalf("Canonicalize() unexpected error: %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("Canonicalize() =\n%s\nwant:\n%s", string(got), tt.expected)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	obj1 := map[string]any{"b": 2, "a": 1}
	obj2 := map[string]any{"a": 1, "b": 2}

	hash1, err := ContentHash(obj1)
	if err != nil {
		t.Fatalf("ContentHash(obj1) error: %v", err)
	}
	hash2, err := ContentHash(obj2)
	if err != nil {
		t.Fatalf("ContentHash(obj2) error: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected identical hashes for semantically identical objects: %s != %s", hash1, hash2)
	}

	if len(hash1) != 71 { // "sha256:" (7) + 64 hex chars = 71
		t.Errorf("unexpected hash format length: %d (got %s)", len(hash1), hash1)
	}
}
