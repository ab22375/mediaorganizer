package config

import (
	"testing"
)

func TestIsValidScheme(t *testing.T) {
	tests := []struct {
		name     string
		scheme   string
		expected bool
	}{
		{"extension_first is valid", "extension_first", true},
		{"date_first is valid", "date_first", true},
		{"empty string is invalid", "", false},
		{"random string is invalid", "random", false},
		{"similar but wrong is invalid", "date-first", false},
		{"uppercase is invalid", "EXTENSION_FIRST", false},
		{"mixed case is invalid", "Extension_First", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidScheme(tt.scheme)
			if result != tt.expected {
				t.Errorf("IsValidScheme(%q) = %v, want %v", tt.scheme, result, tt.expected)
			}
		})
	}
}

func TestOrganizationSchemeConstants(t *testing.T) {
	// Verify the constant values are as expected
	if SchemeExtensionFirst != "extension_first" {
		t.Errorf("SchemeExtensionFirst = %q, want %q", SchemeExtensionFirst, "extension_first")
	}
	if SchemeDateFirst != "date_first" {
		t.Errorf("SchemeDateFirst = %q, want %q", SchemeDateFirst, "date_first")
	}
}

func TestValidSchemesContainsAllSchemes(t *testing.T) {
	expectedSchemes := []OrganizationScheme{SchemeExtensionFirst, SchemeDateFirst}

	if len(ValidSchemes) != len(expectedSchemes) {
		t.Errorf("ValidSchemes has %d elements, want %d", len(ValidSchemes), len(expectedSchemes))
	}

	for _, expected := range expectedSchemes {
		found := false
		for _, valid := range ValidSchemes {
			if valid == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidSchemes does not contain %q", expected)
		}
	}
}
