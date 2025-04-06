package redirects

import (
	"strings"
	"testing"
)

func TestRedirects__CheckProtocolFormatAfterProtocolReformat(t *testing.T) {
	redirector := redirects{}

	tests := []struct {
		input    string
		expected string
	}{
		{"https://google.com", "https://google.com"},
		{"http://google.com", "https://google.com"},
		{"//google.com", "https://google.com"},
		{"google.com", "https://google.com"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			cleanedURL := redirector.cleanRedirectProtocolSoLinksActuallyWork(tc.input)
			if !strings.HasPrefix(cleanedURL, "https://") {
				t.Errorf("Expected cleaned URL to start with 'https://', got %q", cleanedURL)
			}
			if cleanedURL != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, cleanedURL)
			}
		})
	}
}
