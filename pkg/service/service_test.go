package service

import (
	"testing"
)

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"12345", true},
		{"1234a", false},
		{"", true}, // Empty string is considered numeric
		{"0", true},
		{"abc", false},
	}

	for _, test := range tests {
		result := isNumeric(test.input)
		if result != test.expected {
			t.Errorf("isNumeric(%s) = %v; expected %v", test.input, result, test.expected)
		}
	}
}

func TestSanitizeQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"jakarta", "Jakarta"},
		{"jakarta barat", "Jakarta Barat"},
		{"jakarta-barat", "Jakartabarat"}, // Hyphens are removed but no space is added
		{"jakarta123", "Jakarta123"},
		{"", ""},
	}

	for _, test := range tests {
		result := sanitizeQuery(test.input)
		if result != test.expected {
			t.Errorf("sanitizeQuery(%s) = %s; expected %s", test.input, result, test.expected)
		}
	}
}
