package security

import "testing"

func TestIsAllowedRedirect(t *testing.T) {
	tests := []struct {
		uri      string
		expected bool
	}{
		{"https://claude.ai/callback", true},
		{"https://sub.claude.ai/callback", true},
		{"https://anthropic.com/callback", true},
		{"https://sub.anthropic.com/callback", true},
		{"http://claude.ai/callback", false}, // No HTTP
		{"https://evil.com/callback", false},
		{"https://claude.ai.evil.com/callback", false},
		{"https://anthropic.com.evil.com/callback", false},
		{"invalid-url", false},
		{"https://", false},
		{"", false},
		{"https://a b.com", false}, // Space in URL should fail parse or be invalid
	}

	for _, tt := range tests {
		if got := IsAllowedRedirect(tt.uri); got != tt.expected {
			t.Errorf("IsAllowedRedirect(%q) = %v; want %v", tt.uri, got, tt.expected)
		}
	}
}
